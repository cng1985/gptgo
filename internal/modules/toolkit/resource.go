package toolkit

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	aiplat "gptgo/internal/platform/ai"
	apperr "gptgo/internal/platform/errors"
	"gptgo/internal/platform/response"
)

type Resource struct {
	repo   *Repository
	cipher *Cipher
	http   *http.Client
	mu     sync.Mutex
	cache  map[int64]cachedTools
}

type cachedTools struct {
	at      time.Time
	updated int64
	tools   []mcpRemoteTool
}

func NewResource(repo *Repository, cipher *Cipher) *Resource {
	return &Resource{repo: repo, cipher: cipher, http: newSafeClient(), cache: map[int64]cachedTools{}}
}

func (r *Resource) SearchBuiltin(ctx context.Context, search *ToolSearch) (response.PageResponse[BuiltinTool], error) {
	var zero response.PageResponse[BuiltinTool]
	if search == nil {
		return zero, apperr.Invalid("参数格式错误")
	}
	page, size, err := normalizePage(search.Page, search.PageSize, 20)
	if err != nil {
		return zero, err
	}
	prefs, err := r.repo.Prefs(ctx)
	if err != nil {
		return zero, err
	}
	catalog := builtinCatalog()
	data := make([]BuiltinTool, 0, len(catalog))
	if page == 1 {
		for _, item := range catalog {
			enabled := true
			if value, ok := prefs[item.Name]; ok {
				enabled = value
			}
			data = append(data, BuiltinTool{Name: item.Name, Description: item.Description, Enabled: enabled})
		}
	}
	return response.PageResponse[BuiltinTool]{Data: data, Total: int64(len(catalog)), Page: page, PageSize: size}, nil
}

func (r *Resource) UpdateBuiltin(ctx context.Context, request *BuiltinUpdate) (response.Response[BuiltinTool], error) {
	var zero response.Response[BuiltinTool]
	if request == nil {
		return zero, apperr.Invalid("参数格式错误")
	}
	var found *builtinTool
	for _, item := range builtinCatalog() {
		if item.Name == request.Name {
			copy := item
			found = &copy
			break
		}
	}
	if found == nil {
		return zero, apperr.NotFound("工具不存在")
	}
	if err := r.repo.SetPref(ctx, request.Name, request.Enabled); err != nil {
		return zero, err
	}
	return response.OK(BuiltinTool{Name: found.Name, Description: found.Description, Enabled: request.Enabled}), nil
}

func (r *Resource) CreateHTTP(ctx context.Context, request *HTTPToolRequest) (response.Response[HTTPToolResponse], error) {
	var zero response.Response[HTTPToolResponse]
	if err := prepareHTTP(request, false); err != nil {
		return zero, err
	}
	headers, err := r.seal(request.Headers, false)
	if err != nil {
		return zero, err
	}
	stored := ""
	if headers != nil {
		stored = *headers
	}
	return r.repo.CreateHTTP(ctx, request, stored)
}

func (r *Resource) UpdateHTTP(ctx context.Context, request *HTTPToolRequest) (response.Response[HTTPToolResponse], error) {
	var zero response.Response[HTTPToolResponse]
	if request == nil || request.ID <= 0 {
		return zero, apperr.Invalid("工具不存在")
	}
	if err := prepareHTTP(request, true); err != nil {
		return zero, err
	}
	headers, err := r.seal(request.Headers, true)
	if err != nil {
		return zero, err
	}
	return r.repo.UpdateHTTP(ctx, request, headers)
}

func (r *Resource) OneHTTP(ctx context.Context, request *HTTPToolRequest) (response.Response[HTTPToolResponse], error) {
	var zero response.Response[HTTPToolResponse]
	if request == nil || request.ID <= 0 {
		return zero, apperr.NotFound("工具不存在")
	}
	return r.repo.OneHTTP(ctx, request.ID)
}

func (r *Resource) SearchHTTP(ctx context.Context, search *HTTPToolSearch) (response.PageResponse[HTTPToolSimple], error) {
	var zero response.PageResponse[HTTPToolSimple]
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
	return r.repo.SearchHTTP(ctx, search)
}

func (r *Resource) DeleteHTTP(ctx context.Context, request *HTTPToolRequest) (response.Response[HTTPToolResponse], error) {
	var zero response.Response[HTTPToolResponse]
	if request == nil || request.ID <= 0 {
		return zero, apperr.NotFound("工具不存在")
	}
	return r.repo.DeleteHTTP(ctx, request.ID)
}

func (r *Resource) CreateMCP(ctx context.Context, request *MCPRequest) (response.Response[MCPResponse], error) {
	var zero response.Response[MCPResponse]
	if err := prepareMCP(request); err != nil {
		return zero, err
	}
	envCipher, headersCipher, err := r.sealPair(request.Env, request.Headers, false)
	if err != nil {
		return zero, err
	}
	env, headers := "", ""
	if envCipher != nil {
		env = *envCipher
	}
	if headersCipher != nil {
		headers = *headersCipher
	}
	return r.repo.CreateMCP(ctx, request, env, headers)
}

func (r *Resource) UpdateMCP(ctx context.Context, request *MCPRequest) (response.Response[MCPResponse], error) {
	var zero response.Response[MCPResponse]
	if request == nil || request.ID <= 0 {
		return zero, apperr.Invalid("MCP 服务不存在")
	}
	if err := prepareMCP(request); err != nil {
		return zero, err
	}
	envCipher, headersCipher, err := r.sealPair(request.Env, request.Headers, true)
	if err != nil {
		return zero, err
	}
	result, err := r.repo.UpdateMCP(ctx, request, envCipher, headersCipher)
	if err == nil {
		r.dropCache(request.ID)
	}
	return result, err
}

func (r *Resource) OneMCP(ctx context.Context, request *MCPRequest) (response.Response[MCPResponse], error) {
	var zero response.Response[MCPResponse]
	if request == nil || request.ID <= 0 {
		return zero, apperr.NotFound("MCP 服务不存在")
	}
	return r.repo.OneMCP(ctx, request.ID)
}

func (r *Resource) SearchMCP(ctx context.Context, search *MCPSearch) (response.PageResponse[MCPSimple], error) {
	var zero response.PageResponse[MCPSimple]
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
	return r.repo.SearchMCP(ctx, search)
}

func (r *Resource) DeleteMCP(ctx context.Context, request *MCPRequest) (response.Response[MCPResponse], error) {
	var zero response.Response[MCPResponse]
	if request == nil || request.ID <= 0 {
		return zero, apperr.NotFound("MCP 服务不存在")
	}
	result, err := r.repo.DeleteMCP(ctx, request.ID)
	if err == nil {
		r.dropCache(request.ID)
	}
	return result, err
}

func (r *Resource) RemoteTools(ctx context.Context, request *MCPRequest) (response.Response[[]MCPToolSimple], error) {
	var zero response.Response[[]MCPToolSimple]
	if request == nil || request.ID <= 0 {
		return zero, apperr.NotFound("MCP 服务不存在")
	}
	server, err := r.repo.findMCP(ctx, request.ID)
	if err != nil {
		return zero, err
	}
	listed, err := r.toolsOf(ctx, server, true)
	if err != nil {
		return zero, apperr.Unavailable(clipText(err.Error(), 120))
	}
	items := make([]MCPToolSimple, 0, len(listed))
	for _, tool := range listed {
		items = append(items, MCPToolSimple{Name: tool.Name, Description: tool.Description})
	}
	return response.OK(items), nil
}

func (r *Resource) Catalog(ctx context.Context) ([]aiplat.Tool, error) {
	prefs, err := r.repo.Prefs(ctx)
	if err != nil {
		return nil, err
	}
	tools := make([]aiplat.Tool, 0, 8)
	for _, item := range builtinCatalog() {
		enabled := true
		if value, ok := prefs[item.Name]; ok {
			enabled = value
		}
		if !enabled {
			continue
		}
		tools = append(tools, aiplat.Tool{
			Name: item.Name, Description: item.Description, Parameters: json.RawMessage(item.Parameters),
		})
	}
	https, err := r.repo.EnabledHTTP(ctx)
	if err != nil {
		return nil, err
	}
	for _, item := range https {
		if reservedToolName(item.Name) {
			continue
		}
		tools = append(tools, aiplat.Tool{
			Name: item.Name, Description: item.Description, Parameters: json.RawMessage(item.Parameters),
		})
	}
	servers, err := r.repo.EnabledMCP(ctx)
	if err != nil {
		return nil, err
	}
	tools = append(tools, r.mcpCatalog(ctx, servers)...)
	if len(tools) > 32 {
		tools = tools[:32]
	}
	return tools, nil
}

func (r *Resource) Invoke(ctx context.Context, name, arguments string) (bool, string, error) {
	if err := ctx.Err(); err != nil {
		return false, "", err
	}
	switch name {
	case "current_time", "calculator":
		enabled, err := r.builtinOn(ctx, name)
		if err != nil {
			return false, "", err
		}
		if !enabled {
			return false, "这个工具已停用", nil
		}
		if name == "current_time" {
			ok, text := runCurrentTime(arguments)
			return ok, text, nil
		}
		ok, text := runCalculator(arguments)
		return ok, text, nil
	}
	if id, ok := mcpServerID(name); ok {
		return r.invokeMCP(ctx, id, name, arguments)
	}
	return r.invokeHTTP(ctx, name, arguments)
}

func (r *Resource) builtinOn(ctx context.Context, name string) (bool, error) {
	prefs, err := r.repo.Prefs(ctx)
	if err != nil {
		return false, err
	}
	if value, ok := prefs[name]; ok {
		return value, nil
	}
	return true, nil
}

func (r *Resource) invokeHTTP(ctx context.Context, name, arguments string) (bool, string, error) {
	entity, err := r.repo.FindHTTPByName(ctx, name)
	if err != nil {
		if appErr, ok := apperr.As(err); ok && appErr.Cat == apperr.CategoryNotFound {
			return false, "没有这个工具", nil
		}
		return false, "", err
	}
	if !entity.Enabled {
		return false, "这个工具已停用", nil
	}
	headers, err := r.openMap(entity.Headers)
	if err != nil {
		return false, "工具请求头无法读取", nil
	}
	ok, text := callHTTPTool(ctx, r.http, entity.Method, entity.URL, headers, arguments)
	if ctx.Err() != nil {
		return false, "", ctx.Err()
	}
	return ok, text, nil
}

func (r *Resource) invokeMCP(ctx context.Context, id int64, name, arguments string) (bool, string, error) {
	server, err := r.repo.findMCP(ctx, id)
	if err != nil {
		if appErr, ok := apperr.As(err); ok && appErr.Cat == apperr.CategoryNotFound {
			return false, "没有这个工具", nil
		}
		return false, "", err
	}
	if !server.Enabled {
		return false, "这个 MCP 服务已停用", nil
	}
	listed, err := r.toolsOf(ctx, server, false)
	if err != nil {
		if ctx.Err() != nil {
			return false, "", ctx.Err()
		}
		return false, "MCP 服务暂时不可用", nil
	}
	remote := ""
	for index, tool := range listed {
		if publicMCPName(server.ID, index, tool.Name) == name {
			remote = tool.Name
			break
		}
	}
	if remote == "" {
		return false, "MCP 里没有这个工具", nil
	}
	spec, err := r.specOf(server)
	if err != nil {
		return false, "MCP 配置无法读取", nil
	}
	return callMCP(ctx, r.http, spec, remote, arguments)
}

func (r *Resource) mcpCatalog(ctx context.Context, servers []MCPServer) []aiplat.Tool {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	var mu sync.Mutex
	var out []aiplat.Tool
	var wg sync.WaitGroup
	sem := make(chan struct{}, 3)
	for i := range servers {
		server := servers[i]
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			listed, err := r.toolsOf(ctx, &server, false)
			if err != nil {
				return
			}
			mu.Lock()
			defer mu.Unlock()
			for index, tool := range listed {
				out = append(out, aiplat.Tool{
					Name:        publicMCPName(server.ID, index, tool.Name),
					Description: clipText("MCP「"+server.Name+"」的 "+tool.Name+"。"+tool.Description, 400),
					Parameters:  tool.InputSchema,
				})
			}
		}()
	}
	wg.Wait()
	return out
}

func (r *Resource) toolsOf(ctx context.Context, server *MCPServer, force bool) ([]mcpRemoteTool, error) {
	updated := server.UpdatedAt.UTC().Unix()
	if !force {
		r.mu.Lock()
		if item, ok := r.cache[server.ID]; ok && item.updated == updated && time.Since(item.at) < 2*time.Minute {
			tools := item.tools
			r.mu.Unlock()
			return tools, nil
		}
		r.mu.Unlock()
	}
	spec, err := r.specOf(server)
	if err != nil {
		return nil, err
	}
	tools, err := listMCP(ctx, r.http, spec)
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	r.cache[server.ID] = cachedTools{at: time.Now(), updated: updated, tools: tools}
	r.mu.Unlock()
	return tools, nil
}

func (r *Resource) specOf(server *MCPServer) (mcpSpec, error) {
	spec := mcpSpec{
		Transport: server.Transport,
		Command:   server.Command,
		URL:       server.URL,
		Args:      []string{},
		Env:       map[string]string{},
		Headers:   map[string]string{},
	}
	if strings.TrimSpace(server.Args) != "" && server.Args != "[]" {
		if err := json.Unmarshal([]byte(server.Args), &spec.Args); err != nil {
			return spec, err
		}
	}
	env, err := r.openMap(server.EnvCipher)
	if err != nil {
		return spec, err
	}
	headers, err := r.openMap(server.HeadersCipher)
	if err != nil {
		return spec, err
	}
	spec.Env = env
	spec.Headers = headers
	return spec, nil
}

func (r *Resource) openMap(cipherText string) (map[string]string, error) {
	out := map[string]string{}
	if strings.TrimSpace(cipherText) == "" || r == nil {
		return out, nil
	}
	plain, err := r.cipher.Decrypt(cipherText)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(plain) == "" || plain == "{}" {
		return out, nil
	}
	if err := json.Unmarshal([]byte(plain), &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *Resource) sealPair(envRaw, headerRaw string, updating bool) (*string, *string, error) {
	env, err := r.seal(envRaw, updating)
	if err != nil {
		return nil, nil, err
	}
	headers, err := r.seal(headerRaw, updating)
	if err != nil {
		return nil, nil, err
	}
	return env, headers, nil
}

func (r *Resource) seal(raw string, updating bool) (*string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		if updating {
			return nil, nil
		}
		empty := ""
		return &empty, nil
	}
	values, err := parseSecretMap(raw)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return nil, apperr.Invalid("请求头格式无效")
	}
	if r == nil || r.cipher == nil {
		return nil, apperr.Unavailable("暂时无法保存密钥")
	}
	sealed, err := r.cipher.Encrypt(string(encoded))
	if err != nil {
		return nil, apperr.Unavailable("暂时无法保存密钥")
	}
	return &sealed, nil
}

func (r *Resource) dropCache(id int64) {
	r.mu.Lock()
	delete(r.cache, id)
	r.mu.Unlock()
}

func prepareHTTP(request *HTTPToolRequest, updating bool) error {
	if request == nil {
		return apperr.Invalid("参数格式错误")
	}
	request.Name = strings.TrimSpace(request.Name)
	request.Description = strings.TrimSpace(request.Description)
	request.Method = strings.ToUpper(strings.TrimSpace(request.Method))
	request.URL = strings.TrimSpace(request.URL)
	if !toolNamePattern.MatchString(request.Name) || reservedToolName(request.Name) {
		return apperr.Invalid("名称需要是小写字母、数字或下划线，且不能占用内置工具名")
	}
	if request.Description == "" || len([]rune(request.Description)) > 300 {
		return apperr.Invalid("说明长度需要在 1 到 300 个字之间")
	}
	if request.Method != http.MethodGet && request.Method != http.MethodPost {
		return apperr.Invalid("只支持 GET 和 POST")
	}
	if _, err := parseToolURL(request.URL); err != nil || len(request.URL) > 500 {
		return apperr.Invalid("地址需要是 http 或 https")
	}
	schema, err := cleanSchema(request.Parameters)
	if err != nil {
		return err
	}
	request.Parameters = schema
	if strings.TrimSpace(request.Headers) != "" {
		if _, err := parseSecretMap(request.Headers); err != nil {
			return err
		}
	}
	if updating && request.ID <= 0 {
		return apperr.Invalid("工具不存在")
	}
	return nil
}

func prepareMCP(request *MCPRequest) error {
	if request == nil {
		return apperr.Invalid("参数格式错误")
	}
	request.Name = strings.TrimSpace(request.Name)
	request.Transport = strings.TrimSpace(request.Transport)
	request.Command = strings.TrimSpace(request.Command)
	request.URL = strings.TrimSpace(request.URL)
	if request.Name == "" || len([]rune(request.Name)) > 40 {
		return apperr.Invalid("名称长度需要在 1 到 40 个字之间")
	}
	args, err := cleanArgs(request.Args)
	if err != nil {
		return err
	}
	request.Args = args
	switch request.Transport {
	case "stdio":
		if !validCommand(request.Command) {
			return apperr.Invalid("命令无效，不能包含空格或 shell 符号")
		}
		request.URL = ""
	case "http":
		if _, err := parseToolURL(request.URL); err != nil || len(request.URL) > 500 {
			return apperr.Invalid("地址需要是 http 或 https")
		}
		request.Command = ""
		request.Args = []string{}
	default:
		return apperr.Invalid("传输方式只支持 stdio 或 http")
	}
	if strings.TrimSpace(request.Env) != "" {
		if _, err := parseSecretMap(request.Env); err != nil {
			return err
		}
	}
	if strings.TrimSpace(request.Headers) != "" {
		if _, err := parseSecretMap(request.Headers); err != nil {
			return err
		}
	}
	return nil
}

func cleanSchema(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return `{"type":"object","properties":{"input":{"type":"string","description":"传给接口的内容"}},"additionalProperties":true}`, nil
	}
	if len(raw) > 4000 || !json.Valid([]byte(raw)) || raw[0] != '{' {
		return "", apperr.Invalid("参数说明需要是 JSON 对象")
	}
	return raw, nil
}

func cleanArgs(args []string) ([]string, error) {
	if len(args) > 16 {
		return nil, apperr.Invalid("参数不能超过 16 个")
	}
	out := make([]string, 0, len(args))
	for _, arg := range args {
		arg = strings.TrimSpace(arg)
		if arg == "" {
			continue
		}
		if len(arg) > 300 || strings.ContainsAny(arg, "\r\n\x00") {
			return nil, apperr.Invalid("命令参数无效")
		}
		out = append(out, arg)
	}
	return out, nil
}

func parseSecretMap(raw string) (map[string]string, error) {
	raw = strings.TrimSpace(raw)
	if !json.Valid([]byte(raw)) || raw[0] != '{' {
		return nil, apperr.Invalid("密钥需要是 JSON 对象")
	}
	var decoded map[string]string
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return nil, apperr.Invalid("密钥需要是字符串到字符串的 JSON 对象")
	}
	if len(decoded) > 20 {
		return nil, apperr.Invalid("密钥不能超过 20 项")
	}
	out := make(map[string]string, len(decoded))
	for key, value := range decoded {
		if !secretKeyPattern.MatchString(key) {
			return nil, apperr.Invalid("密钥名称无效")
		}
		if len(value) > 500 || strings.ContainsAny(value, "\r\n\x00") {
			return nil, apperr.Invalid("密钥内容无效")
		}
		out[key] = value
	}
	return out, nil
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

var (
	toolNamePattern  = regexp.MustCompile(`^[a-z][a-z0-9_]{0,39}$`)
	secretKeyPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]{0,39}$`)
)
