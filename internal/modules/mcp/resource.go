package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"unicode/utf8"

	apperr "gptgo/internal/platform/errors"
	"gptgo/internal/platform/response"
)

type Resource struct {
	repo *Repository
}

func NewResource(repo *Repository) *Resource {
	return &Resource{repo: repo}
}

func (r *Resource) Create(ctx context.Context, request *ServerRequest) (response.Response[ServerResponse], error) {
	var zero response.Response[ServerResponse]
	if err := prepareServer(request, false); err != nil {
		return zero, err
	}
	return r.repo.Create(ctx, request)
}

func (r *Resource) Update(ctx context.Context, request *ServerRequest) (response.Response[ServerResponse], error) {
	var zero response.Response[ServerResponse]
	if request == nil || request.ID <= 0 {
		return zero, apperr.Invalid("MCP 服务不存在")
	}
	if err := prepareServer(request, true); err != nil {
		return zero, err
	}
	return r.repo.Update(ctx, request)
}

func (r *Resource) One(ctx context.Context, request *ServerRequest) (response.Response[ServerResponse], error) {
	var zero response.Response[ServerResponse]
	if request == nil || request.ID <= 0 {
		return zero, apperr.NotFound("MCP 服务不存在")
	}
	return r.repo.One(ctx, request)
}

func (r *Resource) Search(ctx context.Context, search *ServerSearch) (response.PageResponse[ServerSimple], error) {
	var zero response.PageResponse[ServerSimple]
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

func (r *Resource) Delete(ctx context.Context, request *ServerRequest) (response.Response[ServerResponse], error) {
	var zero response.Response[ServerResponse]
	if request == nil || request.ID <= 0 {
		return zero, apperr.NotFound("MCP 服务不存在")
	}
	return r.repo.Delete(ctx, request)
}

func (r *Resource) Test(ctx context.Context, request *ServerRequest) (response.Response[TestResponse], error) {
	var zero response.Response[TestResponse]
	server, headers, err := r.resolveProbe(ctx, request)
	if err != nil {
		return zero, err
	}
	tools, err := probeTools(ctx, server, headers)
	if err != nil {
		return zero, asPublic(err)
	}
	views := make([]RemoteToolView, 0, len(tools))
	for _, tool := range tools {
		views = append(views, RemoteToolView{Name: tool.Name, Description: clipText(tool.Description, 200)})
	}
	return response.OK(TestResponse{Tools: views}), nil
}

func (r *Resource) Tools(ctx context.Context) (response.Response[[]BuiltinTool], error) {
	return response.OK(BuiltinInfo()), nil
}

func (r *Resource) resolveProbe(ctx context.Context, request *ServerRequest) (Server, map[string]string, error) {
	if request != nil && request.ID > 0 && len(bytesTrim(request.Headers)) == 0 {
		entity, headers, err := r.repo.Open(ctx, request.ID)
		if err != nil {
			return Server{}, nil, err
		}
		return entity, headers, nil
	}
	if err := prepareServer(request, false); err != nil {
		return Server{}, nil, err
	}
	headers, err := parseHeaders(request.Headers)
	if err != nil {
		return Server{}, nil, err
	}
	return Server{
		Transport: request.Transport,
		URL:       request.URL,
		Command:   request.Command,
		Args:      encodeArgs(request.Args),
		Enabled:   true,
	}, headers, nil
}

func prepareServer(request *ServerRequest, updating bool) error {
	if request == nil {
		return apperr.Invalid("参数格式错误")
	}
	request.Name = strings.TrimSpace(request.Name)
	request.Transport = strings.TrimSpace(request.Transport)
	request.URL = strings.TrimSpace(request.URL)
	request.Command = strings.TrimSpace(request.Command)
	if request.Name == "" || len([]rune(request.Name)) > 40 {
		return apperr.Invalid("名称长度需要在 1 到 40 个字之间")
	}
	switch request.Transport {
	case "http":
		if err := validateEndpoint(request.URL); err != nil {
			return apperr.Invalid(err.Error())
		}
		request.Command = ""
		request.Args = []string{}
	case "stdio":
		request.URL = ""
		args := make([]string, 0, len(request.Args))
		for _, arg := range request.Args {
			arg = strings.TrimSpace(arg)
			if arg == "" {
				continue
			}
			args = append(args, arg)
		}
		request.Args = args
		if err := validateCommand(request.Command, request.Args); err != nil {
			return apperr.Invalid(err.Error())
		}
	default:
		return apperr.Invalid("传输方式只支持 http 或 stdio")
	}
	if _, err := parseHeaders(request.Headers); err != nil {
		return err
	}
	if updating && request.ID <= 0 {
		return apperr.Invalid("MCP 服务不存在")
	}
	return nil
}

func parseHeaders(raw json.RawMessage) (map[string]string, error) {
	trimmed := bytesTrim(raw)
	if len(trimmed) == 0 || string(trimmed) == "null" || string(trimmed) == "{}" {
		return map[string]string{}, nil
	}
	var headers map[string]string
	if err := json.Unmarshal(trimmed, &headers); err != nil || headers == nil {
		return nil, apperr.Invalid("请求头格式错误")
	}
	if len(headers) > 10 {
		return nil, apperr.Invalid("请求头不能超过 10 个")
	}
	for key, value := range headers {
		if !validHeaderName(key) || blockedHeader(key) {
			return nil, apperr.Invalid("请求头名称无效")
		}
		if len(value) > 500 || strings.ContainsAny(value, "\r\n") || !utf8.ValidString(value) {
			return nil, apperr.Invalid("请求头内容无效")
		}
	}
	return headers, nil
}

func validHeaderName(name string) bool {
	if name == "" || len(name) > 64 {
		return false
	}
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
			continue
		}
		return false
	}
	return true
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

func probeTools(ctx context.Context, server Server, headers map[string]string) ([]RemoteTool, error) {
	session, closeFn, err := openSession(ctx, server, headers)
	if err != nil {
		return nil, err
	}
	defer closeFn()
	return session.ListTools(ctx)
}

func openSession(ctx context.Context, server Server, headers map[string]string) (*Session, func(), error) {
	switch server.Transport {
	case "http":
		if err := validateEndpoint(server.URL); err != nil {
			return nil, nil, apperr.Invalid(err.Error())
		}
		return newHTTPSession(server.URL, headers), func() {}, nil
	case "stdio":
		return startStdio(ctx, server.Command, decodeArgs(server.Args))
	default:
		return nil, nil, apperr.Invalid("传输方式只支持 http 或 stdio")
	}
}

func asPublic(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := apperr.As(err); ok {
		return err
	}
	text := strings.TrimSpace(err.Error())
	if text == "" || strings.Contains(text, "/") || strings.Contains(text, `\`) {
		return apperr.Unavailable("MCP 服务暂时无法连接")
	}
	return apperr.Unavailable(text)
}
