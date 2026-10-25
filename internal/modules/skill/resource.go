package skill

import (
	"context"
	"encoding/json"
	"strings"

	aiplat "gptgo/internal/platform/ai"
	apperr "gptgo/internal/platform/errors"
	"gptgo/internal/platform/response"
)

const defaultParameters = `{"type":"object","properties":{"input":{"type":"string","description":"传给脚本的文本"}},"additionalProperties":true}`

type Resource struct {
	repo *Repository
}

func NewResource(repo *Repository) *Resource {
	return &Resource{repo: repo}
}

func (r *Resource) Create(ctx context.Context, request *SkillRequest) (response.Response[SkillResponse], error) {
	var zero response.Response[SkillResponse]
	if err := prepare(request, false); err != nil {
		return zero, err
	}
	return r.repo.Create(ctx, request)
}

func (r *Resource) Update(ctx context.Context, request *SkillRequest) (response.Response[SkillResponse], error) {
	var zero response.Response[SkillResponse]
	if request == nil || request.ID <= 0 {
		return zero, apperr.Invalid("技能不存在")
	}
	if err := prepare(request, true); err != nil {
		return zero, err
	}
	return r.repo.Update(ctx, request)
}

func (r *Resource) One(ctx context.Context, request *SkillRequest) (response.Response[SkillResponse], error) {
	var zero response.Response[SkillResponse]
	if request == nil || request.ID <= 0 {
		return zero, apperr.NotFound("技能不存在")
	}
	return r.repo.One(ctx, request.ID)
}

func (r *Resource) Search(ctx context.Context, search *SkillSearch) (response.PageResponse[SkillSimple], error) {
	var zero response.PageResponse[SkillSimple]
	if search == nil {
		return zero, apperr.Invalid("参数格式错误")
	}
	page, size, err := normalizePage(search.Page, search.PageSize, 50)
	if err != nil {
		return zero, err
	}
	search.Page, search.PageSize = page, size
	if len([]rune(search.Keyword)) > 100 {
		return zero, apperr.Invalid("搜索关键词过长")
	}
	return r.repo.Search(ctx, search)
}

func (r *Resource) Delete(ctx context.Context, request *SkillRequest) (response.Response[SkillResponse], error) {
	var zero response.Response[SkillResponse]
	if request == nil || request.ID <= 0 {
		return zero, apperr.NotFound("技能不存在")
	}
	return r.repo.Delete(ctx, request.ID)
}

func (r *Resource) Run(ctx context.Context, request *SkillRunRequest) (response.Response[SkillRunResult], error) {
	var zero response.Response[SkillRunResult]
	if _, err := ownerID(ctx); err != nil {
		return zero, err
	}
	if request == nil {
		return zero, apperr.Invalid("参数格式错误")
	}
	spec := execSpec{
		Name:    strings.TrimSpace(request.Name),
		Runtime: strings.TrimSpace(request.Runtime),
		Script:  request.Script,
		Timeout: request.TimeoutSec,
	}
	if request.ID > 0 && strings.TrimSpace(spec.Script) == "" {
		entity, err := r.repo.Find(ctx, request.ID)
		if err != nil {
			return zero, err
		}
		spec.Script = entity.Script
		if spec.Runtime == "" {
			spec.Runtime = entity.Runtime
		}
		if spec.Timeout == 0 {
			spec.Timeout = entity.TimeoutSec
		}
		if spec.Name == "" {
			spec.Name = entity.Name
		}
	}
	input, err := normalizeInput(request.Input)
	if err != nil {
		return zero, err
	}
	spec.Input = input
	result, err := execute(ctx, spec)
	if err != nil {
		return zero, err
	}
	return response.OK(result), nil
}

func (r *Resource) Catalog(ctx context.Context) ([]aiplat.Tool, error) {
	rows, err := r.repo.Enabled(ctx)
	if err != nil {
		return nil, err
	}
	tools := make([]aiplat.Tool, 0, len(rows))
	for i := range rows {
		parameters := rows[i].Parameters
		if !json.Valid([]byte(parameters)) || !strings.HasPrefix(strings.TrimSpace(parameters), "{") {
			parameters = defaultParameters
		}
		tools = append(tools, aiplat.Tool{
			Name:        rows[i].Name,
			Description: describe(&rows[i]),
			Parameters:  json.RawMessage(parameters),
		})
	}
	return tools, nil
}

func (r *Resource) Owns(ctx context.Context, name string) (bool, error) {
	_, err := r.repo.FindByName(ctx, name)
	if err == nil {
		return true, nil
	}
	if appErr, ok := apperr.As(err); ok && appErr.Cat == apperr.CategoryNotFound {
		return false, nil
	}
	return false, err
}

func (r *Resource) Invoke(ctx context.Context, name, arguments string) (bool, string, error) {
	entity, err := r.repo.FindByName(ctx, name)
	if err != nil {
		if appErr, ok := apperr.As(err); ok && appErr.Cat == apperr.CategoryNotFound {
			return false, "没有这个技能", nil
		}
		return false, "", err
	}
	if !entity.Enabled {
		return false, "这个技能已停用", nil
	}
	input, err := normalizeInput(arguments)
	if err != nil {
		return false, "参数需要是 JSON 对象", nil
	}
	result, err := execute(ctx, execSpec{
		Name:    entity.Name,
		Runtime: entity.Runtime,
		Script:  entity.Script,
		Timeout: entity.TimeoutSec,
		Input:   input,
	})
	if err != nil {
		return false, "", err
	}
	if !result.OK {
		detail := strings.TrimSpace(result.Stderr)
		if detail == "" {
			detail = "脚本没有成功结束"
		}
		if text := strings.TrimSpace(result.Stdout); text != "" {
			detail = text + "\n" + detail
		}
		return false, clipRunes(detail, 4000), nil
	}
	text := strings.TrimSpace(result.Stdout)
	if text == "" {
		text = "脚本没有输出"
	}
	return true, clipRunes(text, 4000), nil
}

func prepare(request *SkillRequest, updating bool) error {
	if request == nil {
		return apperr.Invalid("参数格式错误")
	}
	request.Title = strings.TrimSpace(request.Title)
	request.Name = strings.TrimSpace(request.Name)
	request.Description = strings.TrimSpace(request.Description)
	request.Instructions = strings.TrimSpace(request.Instructions)
	request.Runtime = strings.TrimSpace(request.Runtime)
	request.Parameters = strings.TrimSpace(request.Parameters)
	if request.Title == "" || len([]rune(request.Title)) > 40 {
		return apperr.Invalid("名称长度需要在 1 到 40 个字之间")
	}
	if !validName(request.Name) {
		return apperr.Invalid("调用名要用小写字母开头，只能包含小写字母、数字和下划线，且不能占用内置或 MCP 工具名")
	}
	if request.Description == "" || len([]rune(request.Description)) > 300 {
		return apperr.Invalid("说明长度需要在 1 到 300 个字之间")
	}
	if len([]rune(request.Instructions)) > 2000 {
		return apperr.Invalid("补充说明不能超过 2000 个字")
	}
	switch request.Runtime {
	case "python", "node", "bash":
	default:
		return apperr.Invalid("只支持 python、node、bash")
	}
	if strings.TrimSpace(request.Script) == "" {
		return apperr.Invalid("脚本不能为空")
	}
	if strings.ContainsRune(request.Script, 0) || len(request.Script) > maxScriptBytes {
		return apperr.Invalid("脚本不能超过 32 KB，且不能包含空字符")
	}
	if request.TimeoutSec == 0 {
		request.TimeoutSec = defaultTimeout
	}
	if request.TimeoutSec < 1 || request.TimeoutSec > maxTimeout {
		return apperr.Invalid("超时时间需要在 1 到 20 秒之间")
	}
	parameters, err := normalizeParameters(request.Parameters)
	if err != nil {
		return err
	}
	request.Parameters = parameters
	if updating && request.ID <= 0 {
		return apperr.Invalid("技能不存在")
	}
	return nil
}

func normalizeParameters(raw string) (string, error) {
	if raw == "" {
		return defaultParameters, nil
	}
	if len(raw) > 4000 || !json.Valid([]byte(raw)) || raw[0] != '{' {
		return "", apperr.Invalid("参数结构需要是 JSON 对象")
	}
	var payload any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return "", apperr.Invalid("参数结构需要是 JSON 对象")
	}
	compact, err := json.Marshal(payload)
	if err != nil {
		return "", apperr.Invalid("参数结构需要是 JSON 对象")
	}
	return string(compact), nil
}

func normalizeInput(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "{}", nil
	}
	if len(raw) > maxInputBytes || !json.Valid([]byte(raw)) || raw[0] != '{' {
		return "", apperr.Invalid("输入需要是 JSON 对象")
	}
	return raw, nil
}

func describe(entity *Skill) string {
	var b strings.Builder
	title := strings.TrimSpace(entity.Title)
	if title != "" && title != entity.Name {
		b.WriteString(title)
		b.WriteString("。")
	}
	b.WriteString(strings.TrimSpace(entity.Description))
	if extra := strings.TrimSpace(entity.Instructions); extra != "" {
		b.WriteString("\n")
		b.WriteString(extra)
	}
	b.WriteString("\n调用后执行已保存的脚本：JSON 参数从标准输入读入，标准输出作为结果。")
	return clipRunes(b.String(), 800)
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
