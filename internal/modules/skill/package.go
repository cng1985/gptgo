package skill

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	apperr "gptgo/internal/platform/errors"
)

const (
	maxZipBytes     = 7 << 20
	maxFiles        = 80
	maxUncompressed = 16 << 20
	maxFileBytes    = 1 << 20
	maxSkillMD      = 256 << 10
	maxSkills       = 40
)

type skillDoc struct {
	Name        string
	DisplayName string
	Description string
	Version     string
	Body        string
}

type origin struct {
	Source    string
	Namespace string
	Slug      string
	Registry  string
}

func readPackage(filename string, raw []byte) (map[string][]byte, skillDoc, error) {
	var zero skillDoc
	if len(raw) == 0 {
		return nil, zero, apperr.Invalid("技能包是空的")
	}
	if len(raw) > maxZipBytes {
		return nil, zero, apperr.TooLarge("技能包过大")
	}
	var files map[string][]byte
	var err error
	if isZip(filename, raw) {
		files, err = filesFromZip(raw)
	} else {
		files = map[string][]byte{"SKILL.md": raw}
	}
	if err != nil {
		return nil, zero, err
	}
	return documentFrom(files)
}

func isZip(filename string, raw []byte) bool {
	if bytes.HasPrefix(raw, []byte("PK")) {
		return true
	}
	return strings.HasSuffix(strings.ToLower(filename), ".zip")
}

func filesFromZip(raw []byte) (map[string][]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, apperr.Invalid("技能包不是有效的压缩文件")
	}
	if len(reader.File) > maxFiles {
		return nil, apperr.Invalid("技能包文件过多")
	}
	out := map[string][]byte{}
	var total int64
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		if file.Mode()&os.ModeSymlink != 0 {
			return nil, apperr.Invalid("技能包不能包含链接")
		}
		rel, skip, err := classifyPath(file.Name)
		if err != nil {
			return nil, err
		}
		if skip {
			continue
		}
		if file.UncompressedSize64 > maxFileBytes {
			return nil, apperr.TooLarge("技能包里有过大的文件")
		}
		rc, err := file.Open()
		if err != nil {
			return nil, apperr.Invalid("技能包无法读取")
		}
		body, readErr := io.ReadAll(io.LimitReader(rc, maxFileBytes+1))
		_ = rc.Close()
		if readErr != nil {
			return nil, apperr.Invalid("技能包无法读取")
		}
		if len(body) > maxFileBytes {
			return nil, apperr.TooLarge("技能包里有过大的文件")
		}
		total += int64(len(body))
		if total > maxUncompressed {
			return nil, apperr.TooLarge("技能包解压后过大")
		}
		if blockedBinary(rel, body) {
			return nil, apperr.Invalid("技能包不能包含可执行文件")
		}
		if _, ok := out[rel]; ok {
			return nil, apperr.Invalid("技能包里有重复文件")
		}
		out[rel] = body
	}
	if len(out) == 0 {
		return nil, apperr.Invalid("技能包是空的")
	}
	return out, nil
}

func documentFrom(files map[string][]byte) (map[string][]byte, skillDoc, error) {
	var zero skillDoc
	normalized, err := normalizeTree(files)
	if err != nil {
		return nil, zero, err
	}
	raw, ok := normalized["SKILL.md"]
	if !ok {
		return nil, zero, apperr.Invalid("技能包需要包含 SKILL.md")
	}
	if len(raw) > maxSkillMD {
		return nil, zero, apperr.TooLarge("SKILL.md 过大")
	}
	doc, err := parseSkill(string(raw))
	if err != nil {
		return nil, zero, err
	}
	return normalized, doc, nil
}

func normalizeTree(files map[string][]byte) (map[string][]byte, error) {
	cleaned := map[string][]byte{}
	for name, body := range files {
		rel, skip, err := classifyPath(name)
		if err != nil {
			return nil, err
		}
		if skip {
			continue
		}
		cleaned[rel] = body
	}
	if skillKey(cleaned) != "" {
		return renameSkill(cleaned), nil
	}
	prefix := singleRoot(cleaned)
	if prefix == "" {
		return nil, apperr.Invalid("技能包需要包含 SKILL.md")
	}
	hoisted := map[string][]byte{}
	for name, body := range cleaned {
		hoisted[strings.TrimPrefix(name, prefix+"/")] = body
	}
	if skillKey(hoisted) == "" {
		return nil, apperr.Invalid("技能包需要包含 SKILL.md")
	}
	return renameSkill(hoisted), nil
}

func skillKey(files map[string][]byte) string {
	for name := range files {
		if strings.EqualFold(name, "SKILL.md") && !strings.Contains(name, "/") {
			return name
		}
	}
	return ""
}

func renameSkill(files map[string][]byte) map[string][]byte {
	key := skillKey(files)
	if key == "" || key == "SKILL.md" {
		return files
	}
	files["SKILL.md"] = files[key]
	delete(files, key)
	return files
}

func singleRoot(files map[string][]byte) string {
	prefix := ""
	for name := range files {
		slash := strings.IndexByte(name, '/')
		if slash <= 0 {
			return ""
		}
		root := name[:slash]
		if prefix == "" {
			prefix = root
			continue
		}
		if root != prefix {
			return ""
		}
	}
	return prefix
}

func classifyPath(name string) (string, bool, error) {
	name = strings.ReplaceAll(name, "\\", "/")
	if strings.Contains(name, "\x00") {
		return "", false, apperr.Invalid("技能包路径无效")
	}
	name = strings.TrimPrefix(name, "/")
	if name == "" || strings.HasSuffix(name, "/") {
		return "", true, nil
	}
	clean := path.Clean(name)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || path.IsAbs(clean) || strings.Contains(clean, ":") {
		return "", false, apperr.Invalid("技能包路径无效")
	}
	if strings.Contains(clean, "__MACOSX") || strings.HasSuffix(clean, ".DS_Store") {
		return "", true, nil
	}
	if strings.Count(clean, "/") > 7 || len(clean) > 180 {
		return "", false, apperr.Invalid("技能包路径过深")
	}
	return clean, false, nil
}

func blockedBinary(rel string, body []byte) bool {
	switch strings.ToLower(path.Ext(rel)) {
	case ".exe", ".dll", ".so", ".dylib", ".bin", ".com", ".msi", ".bat", ".cmd", ".ps1":
		return true
	}
	if len(body) >= 4 && bytes.Equal(body[:4], []byte{0x7f, 'E', 'L', 'F'}) {
		return true
	}
	if len(body) >= 2 && body[0] == 'M' && body[1] == 'Z' {
		return true
	}
	return false
}

func parseSkill(raw string) (skillDoc, error) {
	var doc skillDoc
	front, body, err := splitFront(raw)
	if err != nil {
		return doc, err
	}
	var meta struct {
		Name        string         `yaml:"name"`
		Description string         `yaml:"description"`
		Version     any            `yaml:"version"`
		Metadata    map[string]any `yaml:"metadata"`
	}
	if err := yaml.Unmarshal([]byte(front), &meta); err != nil {
		return doc, apperr.Invalid("SKILL.md 的说明头无法读取")
	}
	name, err := normalizeName(meta.Name)
	if err != nil {
		return doc, err
	}
	description := strings.TrimSpace(meta.Description)
	if description == "" {
		return doc, apperr.Invalid("技能说明不能为空")
	}
	description = clipRunes(description, 1000)
	display := strings.TrimSpace(metaString(meta.Metadata, "display-name", "display_name", "displayName"))
	if display == "" {
		display = name
	}
	display = clipRunes(display, 80)
	version := normalizeVersion(scalarString(meta.Version))
	if version == "" {
		version = normalizeVersion(metaString(meta.Metadata, "version"))
	}
	body = strings.TrimSpace(body)
	if body == "" {
		body = description
	}
	return skillDoc{Name: name, DisplayName: display, Description: description, Version: version, Body: body}, nil
}

func splitFront(raw string) (string, string, error) {
	raw = strings.TrimPrefix(raw, "\uFEFF")
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.TrimPrefix(raw, "\n")
	if !strings.HasPrefix(raw, "---\n") {
		return "", "", apperr.Invalid("SKILL.md 需要以名称和说明开头")
	}
	rest := strings.TrimPrefix(raw, "---\n")
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return "", "", apperr.Invalid("SKILL.md 的说明头不完整")
	}
	body := strings.TrimPrefix(rest[idx+4:], "\n")
	return rest[:idx], body, nil
}

func normalizeName(raw string) (string, error) {
	raw = strings.ToLower(strings.TrimSpace(raw))
	raw = strings.ReplaceAll(raw, "_", "-")
	raw = strings.Trim(raw, "-")
	for strings.Contains(raw, "--") {
		raw = strings.ReplaceAll(raw, "--", "-")
	}
	if !namePattern.MatchString(raw) || len(raw) > 64 {
		return "", apperr.Invalid("技能名称需要是小写字母、数字和短横线")
	}
	return raw, nil
}

func normalizeVersion(raw string) string {
	raw = strings.TrimSpace(raw)
	if len(raw) > 32 {
		raw = raw[:32]
	}
	if raw == "" || !versionPattern.MatchString(raw) {
		return ""
	}
	return raw
}

func metaString(meta map[string]any, keys ...string) string {
	for _, key := range keys {
		if meta == nil {
			return ""
		}
		if value, ok := meta[key]; ok {
			if text := scalarString(value); text != "" {
				return text
			}
		}
	}
	return ""
}

func scalarString(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	default:
		return ""
	}
}

func splitCoord(namespace, slug string) (string, string, error) {
	namespace = strings.ToLower(strings.TrimSpace(namespace))
	slug = strings.ToLower(strings.Trim(strings.TrimSpace(slug), "/"))
	if strings.Count(slug, "/") == 1 {
		parts := strings.Split(slug, "/")
		if namespace == "" {
			namespace = parts[0]
		}
		slug = parts[1]
	}
	if namespace == "" {
		if parts := strings.SplitN(slug, "--", 2); len(parts) == 2 && parts[0] != "" && parts[1] != "" {
			namespace, slug = parts[0], parts[1]
		}
	}
	if namespace == "" {
		namespace = "global"
	}
	namespace, err := normalizeName(namespace)
	if err != nil {
		return "", "", apperr.Invalid("SkillHub 命名空间无效")
	}
	slug, err = normalizeName(slug)
	if err != nil {
		return "", "", apperr.Invalid("SkillHub 技能名称无效")
	}
	return namespace, slug, nil
}

var (
	namePattern    = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	versionPattern = regexp.MustCompile(`^[0-9A-Za-z][0-9A-Za-z._+-]{0,31}$`)
)
