package skill

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"gptgo/internal/platform/config"
	apperr "gptgo/internal/platform/errors"
	"gptgo/internal/platform/response"
)

type Resource struct {
	repo *Repository
	root string
	hub  *Hub
}

func NewResource(repo *Repository, cfg *config.Config) (*Resource, error) {
	hub, err := NewHub(cfg.SkillHub)
	if err != nil {
		return nil, err
	}
	return &Resource{repo: repo, root: filepath.Join(cfg.App.DataDir, "skills"), hub: hub}, nil
}

func (r *Resource) Search(ctx context.Context, search *SkillSearch) (response.PageResponse[SkillSimple], error) {
	var zero response.PageResponse[SkillSimple]
	if search == nil {
		return zero, apperr.Invalid("参数格式错误")
	}
	page, size, err := normalizePage(search.Page, search.PageSize, 20)
	if err != nil {
		return zero, err
	}
	if len([]rune(search.Keyword)) > 100 {
		return zero, apperr.Invalid("搜索关键词过长")
	}
	search.Page, search.PageSize = page, size
	return r.repo.Search(ctx, search)
}

func (r *Resource) One(ctx context.Context, request *SkillRequest) (response.Response[SkillResponse], error) {
	var zero response.Response[SkillResponse]
	if request == nil || request.ID <= 0 {
		return zero, apperr.NotFound("技能不存在")
	}
	entity, err := r.repo.FindByID(ctx, request.ID)
	if err != nil {
		return zero, err
	}
	return response.OK(r.detail(entity, 2000)), nil
}

func (r *Resource) Update(ctx context.Context, request *SkillRequest) (response.Response[SkillResponse], error) {
	var zero response.Response[SkillResponse]
	if request == nil || request.ID <= 0 {
		return zero, apperr.NotFound("技能不存在")
	}
	entity, err := r.repo.SetEnabled(ctx, request.ID, request.Enabled)
	if err != nil {
		return zero, err
	}
	return response.OK(r.detail(entity, 2000)), nil
}

func (r *Resource) Delete(ctx context.Context, request *SkillRequest) (response.Response[SkillResponse], error) {
	var zero response.Response[SkillResponse]
	if request == nil || request.ID <= 0 {
		return zero, apperr.NotFound("技能不存在")
	}
	entity, err := r.repo.Delete(ctx, request.ID)
	if err != nil {
		return zero, err
	}
	_ = os.RemoveAll(r.dir(entity.AccountID, entity.ID))
	return response.OK(responseOf(entity, nil, "")), nil
}

func (r *Resource) Upload(ctx context.Context, filename string, file io.Reader) (response.Response[SkillResponse], error) {
	var zero response.Response[SkillResponse]
	if file == nil {
		return zero, apperr.Invalid("请上传 .zip 或 SKILL.md")
	}
	raw, err := io.ReadAll(io.LimitReader(file, maxZipBytes+1))
	if err != nil {
		var max *http.MaxBytesError
		if errors.As(err, &max) {
			return zero, apperr.TooLarge("技能包过大")
		}
		return zero, apperr.Invalid("技能包无法读取")
	}
	files, doc, err := readPackage(filename, raw)
	if err != nil {
		return zero, err
	}
	entity, err := r.save(ctx, doc, files, origin{Source: "upload", Slug: doc.Name})
	if err != nil {
		return zero, err
	}
	return response.OK(r.detail(entity, 2000)), nil
}

func (r *Resource) SearchHub(ctx context.Context, search *HubSearch) (response.PageResponse[HubSkill], error) {
	var zero response.PageResponse[HubSkill]
	if search == nil {
		return zero, apperr.Invalid("参数格式错误")
	}
	page, size, err := normalizePage(search.Page, search.PageSize, 20)
	if err != nil {
		return zero, err
	}
	if size > 50 {
		size = 50
	}
	items, total, err := r.hub.Search(ctx, search.Keyword, page, size)
	if err != nil {
		return zero, err
	}
	if items == nil {
		items = []HubSkill{}
	}
	return response.PageResponse[HubSkill]{Data: items, Total: total, Page: page, PageSize: size}, nil
}

func (r *Resource) InstallHub(ctx context.Context, request *HubInstall) (response.Response[SkillResponse], error) {
	var zero response.Response[SkillResponse]
	if request == nil {
		return zero, apperr.Invalid("参数格式错误")
	}
	namespace, slug, err := splitCoord(request.Namespace, request.Slug)
	if err != nil {
		return zero, err
	}
	raw, err := r.hub.Download(ctx, namespace, slug, request.Version)
	if err != nil {
		return zero, err
	}
	files, doc, err := readPackage(slug+".zip", raw)
	if err != nil {
		return zero, err
	}
	if doc.Version == "" {
		doc.Version = normalizeVersion(request.Version)
	}
	entity, err := r.save(ctx, doc, files, origin{
		Source: "skillhub", Namespace: namespace, Slug: slug, Registry: r.registry(),
	})
	if err != nil {
		return zero, err
	}
	return response.OK(r.detail(entity, 2000)), nil
}

func (r *Resource) Enabled(ctx context.Context) ([]Entry, error) {
	rows, err := r.repo.Enabled(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Entry, 0, len(rows))
	for i := range rows {
		out = append(out, Entry{Name: rows[i].Name, DisplayName: rows[i].DisplayName, Description: rows[i].Description})
	}
	return out, nil
}

func (r *Resource) Load(ctx context.Context, arguments string) (bool, string, error) {
	var body struct {
		Name string `json:"name"`
		Path string `json:"path"`
	}
	if err := decodeObject(arguments, &body); err != nil {
		return false, "参数需要是 JSON 对象", nil
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		return false, "需要技能名称", nil
	}
	entity, err := r.repo.FindMatch(ctx, name)
	if err != nil {
		if appErr, ok := apperr.As(err); ok && appErr.Cat == apperr.CategoryNotFound {
			return false, "没有这个技能", nil
		}
		if appErr, ok := apperr.As(err); ok && appErr.Cat == apperr.CategoryInvalid {
			return false, appErr.Msg, nil
		}
		return false, "", err
	}
	if !entity.Enabled {
		return false, "这个技能已停用", nil
	}
	text, err := r.read(entity, body.Path)
	if err != nil {
		if appErr, ok := apperr.As(err); ok && appErr.Cat != apperr.CategoryInternal {
			return false, appErr.Msg, nil
		}
		return false, "", err
	}
	return true, text, nil
}

func (r *Resource) save(ctx context.Context, doc skillDoc, files map[string][]byte, from origin) (*Skill, error) {
	owner, err := ownerID(ctx)
	if err != nil {
		return nil, err
	}
	if from.Slug == "" {
		from.Slug = doc.Name
	}
	existing, err := r.repo.FindByName(ctx, doc.Name)
	if err != nil {
		if appErr, ok := apperr.As(err); !ok || appErr.Cat != apperr.CategoryNotFound {
			return nil, err
		}
		count, err := r.repo.Count(ctx, owner)
		if err != nil {
			return nil, err
		}
		if count >= maxSkills {
			return nil, apperr.Invalid("最多安装 40 个技能")
		}
		now := time.Now().UTC()
		entity := &Skill{
			AccountID: owner, Name: doc.Name, DisplayName: doc.DisplayName, Description: doc.Description,
			Version: doc.Version, Source: from.Source, Namespace: from.Namespace, Slug: from.Slug,
			Registry: from.Registry, Enabled: true, CreatedAt: now, UpdatedAt: now,
		}
		if err := r.repo.Create(ctx, entity); err != nil {
			return nil, err
		}
		if err := writeTree(r.dir(owner, entity.ID), files); err != nil {
			_, _ = r.repo.Delete(ctx, entity.ID)
			_ = os.RemoveAll(r.dir(owner, entity.ID))
			return nil, apperr.Unavailable("技能文件无法保存")
		}
		return entity, nil
	}
	if err := writeTree(r.dir(owner, existing.ID), files); err != nil {
		return nil, apperr.Unavailable("技能文件无法保存")
	}
	existing.Name = doc.Name
	existing.DisplayName = doc.DisplayName
	existing.Description = doc.Description
	existing.Version = doc.Version
	existing.Source = from.Source
	existing.Namespace = from.Namespace
	existing.Slug = from.Slug
	existing.Registry = from.Registry
	existing.UpdatedAt = time.Now().UTC()
	if err := r.repo.UpdateMeta(ctx, existing); err != nil {
		return nil, err
	}
	return existing, nil
}

func (r *Resource) detail(entity *Skill, limit int) SkillResponse {
	dir := r.dir(entity.AccountID, entity.ID)
	files := listFiles(dir)
	body := ""
	if raw, err := os.ReadFile(filepath.Join(dir, "SKILL.md")); err == nil {
		if doc, err := parseSkill(string(raw)); err == nil {
			body = clipRunes(doc.Body, limit)
		} else {
			body = clipRunes(string(raw), limit)
		}
	}
	return responseOf(entity, files, body)
}

func (r *Resource) read(entity *Skill, rel string) (string, error) {
	rel = strings.TrimSpace(rel)
	if rel == "" || strings.EqualFold(rel, "SKILL.md") {
		rel = "SKILL.md"
	}
	clean, skip, err := classifyPath(rel)
	if err != nil || skip || clean == "" {
		return "", apperr.Invalid("技能文件路径无效")
	}
	dir := r.dir(entity.AccountID, entity.ID)
	full := filepath.Clean(filepath.Join(dir, filepath.FromSlash(clean)))
	root := filepath.Clean(dir)
	if full != root && !strings.HasPrefix(full, root+string(os.PathSeparator)) {
		return "", apperr.Invalid("技能文件路径无效")
	}
	info, err := os.Stat(full)
	if err != nil || info.IsDir() {
		return "", apperr.NotFound("技能里没有这个文件")
	}
	raw, err := os.ReadFile(full)
	if err != nil {
		return "", apperr.Unavailable("技能文件无法读取")
	}
	if bytes.IndexByte(raw, 0) >= 0 {
		return "", apperr.Invalid("这个文件不是文本")
	}
	text := string(raw)
	if clean == "SKILL.md" {
		if doc, err := parseSkill(text); err == nil {
			text = doc.Body
		}
	}
	clipped := false
	if len([]rune(text)) > 3200 {
		text = clipRunes(text, 3200)
		clipped = true
	}
	var b strings.Builder
	b.WriteString("技能 ")
	b.WriteString(entity.Name)
	if entity.DisplayName != "" && entity.DisplayName != entity.Name {
		b.WriteString("（")
		b.WriteString(entity.DisplayName)
		b.WriteString("）")
	}
	if entity.Version != "" {
		b.WriteString("\n版本 ")
		b.WriteString(entity.Version)
	}
	b.WriteString("\n\n")
	b.WriteString(text)
	if clipped {
		b.WriteString("\n\n（说明已截断）")
	}
	if names := listFiles(dir); len(names) > 0 {
		if len(names) > 20 {
			names = append(names[:20], "…")
		}
		b.WriteString("\n\n包内文件：")
		b.WriteString(strings.Join(names, ", "))
	}
	return b.String(), nil
}

func (r *Resource) dir(account, id int64) string {
	return filepath.Join(r.root, strconv.FormatInt(account, 10), strconv.FormatInt(id, 10))
}

func (r *Resource) registry() string {
	if r == nil || r.hub == nil || r.hub.base == nil {
		return ""
	}
	return r.hub.base.String()
}

func responseOf(entity *Skill, files []string, body string) SkillResponse {
	if files == nil {
		files = []string{}
	}
	return SkillResponse{
		SkillSimple: simpleOf(entity),
		Files:       files,
		Body:        body,
		CreatedAt:   entity.CreatedAt,
	}
}

func writeTree(dir string, files map[string][]byte) error {
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return err
	}
	staging := dir + ".staging"
	backup := dir + ".bak"
	if err := os.RemoveAll(staging); err != nil {
		return err
	}
	if err := os.MkdirAll(staging, 0o755); err != nil {
		return err
	}
	for rel, body := range files {
		target := filepath.Join(staging, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			_ = os.RemoveAll(staging)
			return err
		}
		if err := os.WriteFile(target, body, 0o644); err != nil {
			_ = os.RemoveAll(staging)
			return err
		}
	}
	_ = os.RemoveAll(backup)
	if _, err := os.Stat(dir); err == nil {
		if err := os.Rename(dir, backup); err != nil {
			_ = os.RemoveAll(staging)
			return err
		}
	}
	if err := os.Rename(staging, dir); err != nil {
		_ = os.Rename(backup, dir)
		_ = os.RemoveAll(staging)
		return err
	}
	_ = os.RemoveAll(backup)
	return nil
}

func listFiles(dir string) []string {
	var out []string
	_ = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil || strings.HasPrefix(rel, "..") {
			return nil
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	sort.Strings(out)
	return out
}

func decodeObject(raw string, dest any) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = "{}"
	}
	if !json.Valid([]byte(raw)) || raw[0] != '{' {
		return apperr.Invalid("参数需要是 JSON 对象")
	}
	return json.Unmarshal([]byte(raw), dest)
}

func normalizePage(page, size, fallback int) (int, int, error) {
	if page == 0 {
		page = 1
	}
	if size == 0 {
		size = fallback
	}
	if page < 1 || size < 1 || size > 200 {
		return 0, 0, apperr.Invalid("分页参数无效")
	}
	if int64(page-1) > 100000/int64(size) {
		return 0, 0, apperr.Invalid("分页范围超出限制")
	}
	return page, size, nil
}
