package chat

import (
	"context"
	"log/slog"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"gptgo/internal/platform/domain"
	apperr "gptgo/internal/platform/errors"
	"gptgo/internal/platform/response"
)

type WebSearch interface {
	Search(ctx context.Context, query string) (string, error)
}

type Optimizer interface {
	Optimize(ctx context.Context, content, model string) (string, error)
}

type Resource struct {
	repo         *Repository
	search       WebSearch
	optimizer    Optimizer
	defaultModel string
}

func NewResource(repo *Repository, search WebSearch, optimizer Optimizer, defaultModel string) *Resource {
	if strings.TrimSpace(defaultModel) == "" {
		defaultModel = "minimax-m3.1-flash"
	}
	return &Resource{repo: repo, search: search, optimizer: optimizer, defaultModel: defaultModel}
}

func (r *Resource) Create(ctx context.Context, request *ConversationRequest) (response.Response[ConversationResponse], error) {
	var zero response.Response[ConversationResponse]
	if err := requireActor(ctx); err != nil {
		return zero, err
	}
	if request == nil {
		request = &ConversationRequest{}
	}
	request.Title = strings.TrimSpace(request.Title)
	if request.Title == "" {
		request.Title = "新对话"
	}
	if len([]rune(request.Title)) > 80 {
		return zero, apperr.Invalid("标题长度需要在 1 到 80 个字之间")
	}
	if request.Model == "" {
		request.Model = r.defaultModel
	}
	if _, ok := Models[request.Model]; !ok {
		return zero, apperr.Invalid("不支持的模型")
	}
	if request.ThinkingMode == "" {
		request.ThinkingMode = "off"
	}
	switch request.ThinkingMode {
	case "off", "light", "deep":
	default:
		return zero, apperr.Invalid("不支持的思考模式")
	}
	return r.repo.Create(ctx, request)
}

func (r *Resource) Update(ctx context.Context, request *ConversationRequest) (response.Response[ConversationResponse], error) {
	var zero response.Response[ConversationResponse]
	if err := requireActor(ctx); err != nil {
		return zero, err
	}
	if request == nil || request.ID <= 0 {
		return zero, apperr.Invalid("对话不存在")
	}
	request.Title = strings.TrimSpace(request.Title)
	if request.Title == "" || len([]rune(request.Title)) > 80 {
		return zero, apperr.Invalid("标题长度需要在 1 到 80 个字之间")
	}
	return r.repo.Update(ctx, request)
}

func (r *Resource) One(ctx context.Context, request *ConversationRequest) (response.Response[ConversationResponse], error) {
	var zero response.Response[ConversationResponse]
	if err := requireActor(ctx); err != nil {
		return zero, err
	}
	if request == nil || request.ID <= 0 {
		return zero, apperr.NotFound("对话不存在")
	}
	return r.repo.One(ctx, request)
}

func (r *Resource) Search(ctx context.Context, search *ConversationSearch) (response.PageResponse[ConversationSimple], error) {
	var zero response.PageResponse[ConversationSimple]
	if err := requireActor(ctx); err != nil {
		return zero, err
	}
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

func (r *Resource) Delete(ctx context.Context, request *ConversationRequest) (response.Response[ConversationResponse], error) {
	var zero response.Response[ConversationResponse]
	if err := requireActor(ctx); err != nil {
		return zero, err
	}
	if request == nil || request.ID <= 0 {
		return zero, apperr.NotFound("对话不存在")
	}
	return r.repo.Delete(ctx, request)
}

func (r *Resource) Clear(ctx context.Context, request *ConversationClearRequest) (response.Response[ConversationResponse], error) {
	var zero response.Response[ConversationResponse]
	if err := requireActor(ctx); err != nil {
		return zero, err
	}
	if request == nil || !request.Confirm {
		return zero, apperr.Invalid("需要确认后才能清空对话")
	}
	return r.repo.Clear(ctx, request)
}

func (r *Resource) SearchMessages(ctx context.Context, search *MessageSearch) (response.PageResponse[MessageSimple], error) {
	var zero response.PageResponse[MessageSimple]
	if err := requireActor(ctx); err != nil {
		return zero, err
	}
	if search == nil || search.ConversationID <= 0 {
		return zero, apperr.NotFound("对话不存在")
	}
	page, size, err := normalizePage(search.Page, search.PageSize, 100)
	if err != nil {
		return zero, err
	}
	search.Page, search.PageSize = page, size
	return r.repo.SearchMessages(ctx, search)
}

func (r *Resource) Prepare(ctx context.Context, request *ChatStreamRequest) (response.Response[ChatPrepareResponse], error) {
	var zero response.Response[ChatPrepareResponse]
	if err := requireActor(ctx); err != nil {
		return zero, err
	}
	if request == nil {
		return zero, apperr.Invalid("参数格式错误")
	}
	if request.RegenerateMessageID > 0 && request.EditMessageID > 0 {
		return zero, apperr.Invalid("不能同时编辑和重新生成")
	}
	request.Content = strings.TrimSpace(request.Content)
	if request.RegenerateMessageID == 0 && request.Content == "" && len(request.Attachments) == 0 {
		return zero, apperr.Invalid("消息不能为空")
	}
	if len([]rune(request.Content)) > 20000 {
		return zero, apperr.Invalid("消息过长")
	}
	if request.Model == "" {
		request.Model = r.defaultModel
	}
	if _, ok := Models[request.Model]; !ok {
		return zero, apperr.Invalid("不支持的模型")
	}
	switch request.ThinkingMode {
	case "", "off":
		request.ThinkingMode = "off"
	case "light", "deep":
	default:
		return zero, apperr.Invalid("不支持的思考模式")
	}
	if err := validateAttachments(request.Attachments); err != nil {
		return zero, err
	}
	result, err := r.repo.Prepare(ctx, request)
	if err != nil {
		return zero, err
	}
	if request.WebSearch {
		query := lastTurn(result.Data.Messages)
		notes, searchErr := r.search.Search(ctx, clipRunes(query, 180))
		if searchErr != nil {
			slog.Default().Warn("web search skipped", "err", searchErr.Error())
		} else {
			result.Data.SearchNotes = clipRunes(strings.TrimSpace(notes), 2000)
		}
	}
	return result, nil
}

func (r *Resource) Complete(ctx context.Context, request *ChatCompleteRequest) (response.Response[ChatCompleteResponse], error) {
	var zero response.Response[ChatCompleteResponse]
	if err := requireActor(ctx); err != nil {
		return zero, err
	}
	if request == nil || request.ConversationID <= 0 || request.AssistantMessageID <= 0 {
		return zero, apperr.Invalid("消息不存在")
	}
	switch request.Status {
	case "done", "stopped", "error":
	default:
		return zero, apperr.Invalid("消息状态无效")
	}
	request.Content = clipRunes(request.Content, 100000)
	request.Thinking = clipRunes(request.Thinking, 20000)
	return r.repo.Complete(ctx, request)
}

func (r *Resource) Optimize(ctx context.Context, request *ChatOptimizeRequest) (response.Response[ChatOptimizeResponse], error) {
	var zero response.Response[ChatOptimizeResponse]
	if err := requireActor(ctx); err != nil {
		return zero, err
	}
	if request == nil {
		return zero, apperr.Invalid("参数格式错误")
	}
	request.Content = strings.TrimSpace(request.Content)
	if request.Content == "" {
		return zero, apperr.Invalid("没有可以优化的内容")
	}
	if len([]rune(request.Content)) > 8000 {
		return zero, apperr.Invalid("提示词过长")
	}
	if request.Model == "" {
		request.Model = r.defaultModel
	}
	optimized, err := r.optimizer.Optimize(ctx, request.Content, request.Model)
	if err != nil {
		return zero, apperr.Unavailable("暂时无法优化提示词")
	}
	optimized = strings.TrimSpace(optimized)
	if optimized == "" {
		return zero, apperr.Unavailable("暂时无法优化提示词")
	}
	return response.OK(ChatOptimizeResponse{Content: optimized}), nil
}

func requireActor(ctx context.Context) error {
	if _, ok := domain.ScopeFrom(ctx); !ok {
		return apperr.Unauthorized("未登录")
	}
	return nil
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

func validateAttachments(items []AttachmentInput) error {
	if len(items) > 5 {
		return apperr.Invalid("最多附加 5 个文件")
	}
	for i := range items {
		name := filepath.Base(strings.TrimSpace(items[i].Name))
		if name == "." || name == string(filepath.Separator) || name == "" || strings.Contains(name, "..") {
			return apperr.Invalid("文件名无效")
		}
		ext := strings.ToLower(filepath.Ext(name))
		if !allowedExt[ext] {
			return apperr.Invalid("仅支持文本与代码文件")
		}
		if !utf8.ValidString(items[i].Content) || strings.ContainsRune(items[i].Content, 0) {
			return apperr.Invalid("文件内容需要是文本")
		}
		if len(items[i].Content) > 100*1024 {
			return apperr.TooLarge("单个文件不能超过 100KB")
		}
		if items[i].MediaType == "" {
			items[i].MediaType = "text/plain"
		}
		items[i].Name = name
	}
	return nil
}

func lastTurn(messages []ModelTurn) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			return messages[i].Content
		}
	}
	return ""
}

func clipRunes(text string, max int) string {
	rs := []rune(text)
	if len(rs) <= max {
		return text
	}
	return string(rs[:max])
}

var allowedExt = map[string]bool{
	".txt": true, ".md": true, ".json": true, ".yaml": true, ".yml": true,
	".go": true, ".java": true, ".sql": true, ".js": true, ".ts": true, ".tsx": true, ".jsx": true,
	".vue": true, ".py": true, ".sh": true, ".xml": true, ".csv": true, ".html": true, ".css": true,
	".c": true, ".cpp": true, ".h": true, ".rs": true, ".toml": true, ".ini": true, ".log": true,
	".php": true, ".rb": true, ".mod": true, ".sum": true, ".ps1": true, ".cmd": true,
}
