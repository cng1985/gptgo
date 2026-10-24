package skill

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"gptgo/internal/platform/config"
	apperr "gptgo/internal/platform/errors"
)

type Hub struct {
	base  *url.URL
	token string
	http  *http.Client
}

func NewHub(cfg config.SkillHubConfig) (*Hub, error) {
	raw := strings.TrimSpace(cfg.BaseURL)
	if raw == "" {
		return &Hub{}, nil
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("skillhub.base_url 需要是 http 或 https")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	parsed.RawQuery = ""
	parsed.Fragment = ""
	timeout := cfg.Timeout.Std()
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	return &Hub{
		base:  parsed,
		token: strings.TrimSpace(cfg.Token),
		http:  newHubClient(timeout, loopbackHost(parsed.Hostname()), parsed.Host),
	}, nil
}

func (h *Hub) ready() error {
	if h == nil || h.base == nil || h.http == nil {
		return apperr.Invalid("还没有配置 SkillHub 地址")
	}
	return nil
}

func (h *Hub) Search(ctx context.Context, keyword string, page, size int) ([]HubSkill, int64, error) {
	if err := h.ready(); err != nil {
		return nil, 0, err
	}
	keyword = strings.TrimSpace(keyword)
	if len([]rune(keyword)) > 100 {
		return nil, 0, apperr.Invalid("搜索关键词过长")
	}
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 50 {
		size = 20
	}
	query := url.Values{}
	query.Set("q", keyword)
	query.Set("keyword", keyword)
	query.Set("limit", strconv.Itoa(size))
	query.Set("size", strconv.Itoa(size))
	query.Set("page", strconv.Itoa(page))
	first, total, err1 := h.searchAt(ctx, "/api/v1/search?"+query.Encode())
	if err1 == nil && len(first) > 0 {
		return first, total, nil
	}
	query.Set("page", strconv.Itoa(page-1))
	second, total2, err2 := h.searchAt(ctx, "/api/v1/skills?"+query.Encode())
	if err2 == nil && len(second) > 0 {
		return second, total2, nil
	}
	if err1 == nil {
		return first, total, nil
	}
	if err2 == nil {
		return second, total2, nil
	}
	return nil, 0, err1
}

func (h *Hub) Download(ctx context.Context, namespace, slug, version string) ([]byte, error) {
	if err := h.ready(); err != nil {
		return nil, err
	}
	namespace, slug, err := splitCoord(namespace, slug)
	if err != nil {
		return nil, err
	}
	version = normalizeVersion(version)
	paths := make([]string, 0, 4)
	canonical := slug
	if namespace != "global" {
		canonical = namespace + "--" + slug
	}
	if version != "" {
		paths = append(paths, "/api/v1/skills/"+namespace+"/"+slug+"/versions/"+url.PathEscape(version)+"/download")
		paths = append(paths, "/api/v1/download/"+canonical+"/"+url.PathEscape(version))
	}
	paths = append(paths,
		"/api/v1/skills/"+namespace+"/"+slug+"/download",
		"/api/v1/download?slug="+url.QueryEscape(canonical),
	)
	var last error
	for _, path := range paths {
		raw, status, err := h.get(ctx, path, maxZipBytes+1)
		if err != nil {
			last = apperr.Unavailable("SkillHub 暂时无法下载")
			continue
		}
		if status == http.StatusUnauthorized || status == http.StatusForbidden {
			return nil, apperr.Forbidden("SkillHub 拒绝了这次下载")
		}
		if status == http.StatusNotFound || status == http.StatusMethodNotAllowed {
			last = apperr.NotFound("SkillHub 上没有这个技能")
			continue
		}
		if status < 200 || status >= 300 {
			last = apperr.Unavailable("SkillHub 暂时无法下载")
			continue
		}
		if len(raw) > maxZipBytes {
			return nil, apperr.TooLarge("技能包过大")
		}
		if !bytesHasZip(raw) {
			last = apperr.Unavailable("SkillHub 没有返回技能包")
			continue
		}
		return raw, nil
	}
	if last == nil {
		last = apperr.Unavailable("SkillHub 暂时无法下载")
	}
	return nil, last
}

func (h *Hub) searchAt(ctx context.Context, path string) ([]HubSkill, int64, error) {
	raw, status, err := h.get(ctx, path, 1<<20)
	if err != nil {
		return nil, 0, apperr.Unavailable("SkillHub 暂时无法搜索")
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return nil, 0, apperr.Forbidden("SkillHub 拒绝了这次搜索")
	}
	if status < 200 || status >= 300 {
		return nil, 0, apperr.Unavailable("SkillHub 暂时无法搜索")
	}
	items, total, err := parseSearch(raw)
	if err != nil {
		return nil, 0, apperr.Unavailable("SkillHub 返回的内容无法识别")
	}
	return items, total, nil
}

func (h *Hub) get(ctx context.Context, path string, limit int64) ([]byte, int, error) {
	ref, err := url.Parse(path)
	if err != nil {
		return nil, 0, err
	}
	target := h.base.ResolveReference(ref)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/json, application/zip")
	req.Header.Set("User-Agent", "ChatBI-AI")
	if h.token != "" {
		req.Header.Set("Authorization", "Bearer "+h.token)
	}
	resp, err := h.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}

func parseSearch(raw []byte) ([]HubSkill, int64, error) {
	var root any
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, 0, err
	}
	obj, ok := root.(map[string]any)
	if !ok {
		return nil, 0, fmt.Errorf("响应不是对象")
	}
	if code, ok := numberCode(obj["code"]); ok && code != 0 && code != 200 {
		return nil, 0, fmt.Errorf("%s", messageOf(obj))
	}
	items := firstArray(obj, "results", "skills", "items", "content")
	payload := obj
	switch data := obj["data"].(type) {
	case []any:
		if len(items) == 0 {
			items = data
		}
	case map[string]any:
		payload = data
		if len(items) == 0 {
			items = firstArray(data, "results", "skills", "items", "content")
		}
	}
	out := make([]HubSkill, 0, len(items))
	for _, item := range items {
		row, ok := item.(map[string]any)
		if !ok {
			continue
		}
		skill, ok := hubSkillFrom(row)
		if ok {
			out = append(out, skill)
		}
	}
	total := firstTotal(obj, payload)
	if total < int64(len(out)) {
		total = int64(len(out))
	}
	return out, total, nil
}

func hubSkillFrom(row map[string]any) (HubSkill, bool) {
	slug := strField(row, "slug", "skillSlug", "skill_slug")
	namespace := strField(row, "namespace", "ns")
	if strings.Count(slug, "/") == 1 && namespace == "" {
		parts := strings.Split(slug, "/")
		namespace, slug = parts[0], parts[1]
	}
	if namespace == "" {
		if parts := strings.SplitN(slug, "--", 2); len(parts) == 2 && parts[1] != "" {
			namespace, slug = parts[0], parts[1]
		}
	}
	namespace = strings.ToLower(strings.TrimSpace(namespace))
	slug = strings.ToLower(strings.TrimSpace(slug))
	if namespace == "" {
		namespace = "global"
	}
	if slug == "" || strings.Contains(slug, "/") {
		return HubSkill{}, false
	}
	name := strField(row, "name", "displayName", "display_name", "title")
	if name == "" {
		name = slug
	}
	return HubSkill{
		Namespace:   namespace,
		Slug:        slug,
		Name:        clipRunes(name, 80),
		Description: clipRunes(strField(row, "description", "summary"), 300),
		Version:     normalizeVersion(strField(row, "version", "latestVersion", "latest_version")),
	}, true
}

func firstArray(obj map[string]any, keys ...string) []any {
	for _, key := range keys {
		if items, ok := obj[key].([]any); ok {
			return items
		}
	}
	return nil
}

func firstTotal(groups ...map[string]any) int64 {
	for _, obj := range groups {
		for _, key := range []string{"total", "totalElements", "total_elements"} {
			if n, ok := numberCode(obj[key]); ok && n >= 0 {
				return int64(n)
			}
		}
	}
	return 0
}

func numberCode(value any) (int, bool) {
	switch typed := value.(type) {
	case float64:
		return int(typed), true
	case int:
		return typed, true
	case json.Number:
		n, err := typed.Int64()
		return int(n), err == nil
	default:
		return 0, false
	}
}

func messageOf(obj map[string]any) string {
	text := strField(obj, "msg", "message", "error")
	if text == "" {
		text = "SkillHub 返回了错误"
	}
	return clipRunes(text, 120)
}

func strField(obj map[string]any, keys ...string) string {
	for _, key := range keys {
		switch typed := obj[key].(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				return strings.TrimSpace(typed)
			}
		case float64:
			return strconv.FormatFloat(typed, 'f', -1, 64)
		}
	}
	return ""
}

func bytesHasZip(raw []byte) bool {
	return len(raw) >= 2 && raw[0] == 'P' && raw[1] == 'K'
}

func loopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
