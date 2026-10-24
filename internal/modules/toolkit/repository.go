package toolkit

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"gptgo/internal/platform/domain"
	apperr "gptgo/internal/platform/errors"
	"gptgo/internal/platform/response"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Prefs(ctx context.Context) (map[string]bool, error) {
	owner, err := ownerID(ctx)
	if err != nil {
		return nil, err
	}
	var rows []ToolPref
	if err := r.db.WithContext(ctx).Where("account_id = ?", owner).Find(&rows).Error; err != nil {
		return nil, mapDB(err)
	}
	out := make(map[string]bool, len(rows))
	for _, row := range rows {
		out[row.Name] = row.Enabled
	}
	return out, nil
}

func (r *Repository) SetPref(ctx context.Context, name string, enabled bool) error {
	owner, err := ownerID(ctx)
	if err != nil {
		return err
	}
	return mapDB(r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("account_id = ? AND name = ?", owner, name).Delete(&ToolPref{}).Error; err != nil {
			return err
		}
		return tx.Create(&ToolPref{AccountID: owner, Name: name, Enabled: enabled}).Error
	}))
}

func (r *Repository) CreateHTTP(ctx context.Context, request *HTTPToolRequest, headers string) (response.Response[HTTPToolResponse], error) {
	var result response.Response[HTTPToolResponse]
	owner, err := ownerID(ctx)
	if err != nil {
		return result, err
	}
	var count int64
	if err := r.db.WithContext(ctx).Model(&HTTPTool{}).Where("account_id = ?", owner).Count(&count).Error; err != nil {
		return result, mapDB(err)
	}
	if count >= 30 {
		return result, apperr.Invalid("最多创建 30 个 HTTP 工具")
	}
	if err := r.ensureHTTPName(ctx, owner, request.Name, 0); err != nil {
		return result, err
	}
	now := time.Now().UTC()
	entity := HTTPTool{
		AccountID: owner, Name: request.Name, Description: request.Description, Parameters: request.Parameters,
		Method: request.Method, URL: request.URL, Headers: headers, Enabled: request.Enabled, CreatedAt: now, UpdatedAt: now,
	}
	if err := r.db.WithContext(ctx).Create(&entity).Error; err != nil {
		return result, mapDB(err)
	}
	return response.OK(httpResponse(&entity)), nil
}

func (r *Repository) UpdateHTTP(ctx context.Context, request *HTTPToolRequest, headers *string) (response.Response[HTTPToolResponse], error) {
	var result response.Response[HTTPToolResponse]
	owner, err := ownerID(ctx)
	if err != nil {
		return result, err
	}
	if err := r.ensureHTTPName(ctx, owner, request.Name, request.ID); err != nil {
		return result, err
	}
	values := map[string]any{
		"name": request.Name, "description": request.Description, "parameters": request.Parameters,
		"method": request.Method, "url": request.URL, "enabled": request.Enabled, "updated_at": time.Now().UTC(),
	}
	if headers != nil {
		values["headers"] = *headers
	}
	res := r.db.WithContext(ctx).Model(&HTTPTool{}).Where("id = ? AND account_id = ?", request.ID, owner).Updates(values)
	if res.Error != nil {
		return result, mapDB(res.Error)
	}
	if res.RowsAffected == 0 {
		return result, apperr.NotFound("工具不存在")
	}
	return r.OneHTTP(ctx, request.ID)
}

func (r *Repository) OneHTTP(ctx context.Context, id int64) (response.Response[HTTPToolResponse], error) {
	var result response.Response[HTTPToolResponse]
	entity, err := r.findHTTP(ctx, id)
	if err != nil {
		return result, err
	}
	return response.OK(httpResponse(entity)), nil
}

func (r *Repository) SearchHTTP(ctx context.Context, search *HTTPToolSearch) (response.PageResponse[HTTPToolSimple], error) {
	var result response.PageResponse[HTTPToolSimple]
	owner, err := ownerID(ctx)
	if err != nil {
		return result, err
	}
	base := func() *gorm.DB {
		query := r.db.WithContext(ctx).Model(&HTTPTool{}).Where("account_id = ?", owner)
		if keyword := strings.TrimSpace(search.Keyword); keyword != "" {
			query = query.Where("name LIKE ? ESCAPE '\\' OR description LIKE ? ESCAPE '\\'", likePattern(keyword), likePattern(keyword))
		}
		return query
	}
	var total int64
	if err := base().Count(&total).Error; err != nil {
		return result, mapDB(err)
	}
	var rows []HTTPTool
	err = base().Order("updated_at desc, id desc").Offset((search.Page - 1) * search.PageSize).Limit(search.PageSize).Find(&rows).Error
	if err != nil {
		return result, mapDB(err)
	}
	data := make([]HTTPToolSimple, 0, len(rows))
	for i := range rows {
		data = append(data, HTTPToolSimple{
			ID: rows[i].ID, Name: rows[i].Name, Description: rows[i].Description,
			Method: rows[i].Method, URL: rows[i].URL, Enabled: rows[i].Enabled, UpdatedAt: rows[i].UpdatedAt,
		})
	}
	return response.PageResponse[HTTPToolSimple]{Data: data, Total: total, Page: search.Page, PageSize: search.PageSize}, nil
}

func (r *Repository) DeleteHTTP(ctx context.Context, id int64) (response.Response[HTTPToolResponse], error) {
	var result response.Response[HTTPToolResponse]
	entity, err := r.findHTTP(ctx, id)
	if err != nil {
		return result, err
	}
	owner, _ := ownerID(ctx)
	if err := r.db.WithContext(ctx).Where("id = ? AND account_id = ?", id, owner).Delete(&HTTPTool{}).Error; err != nil {
		return result, mapDB(err)
	}
	return response.OK(httpResponse(entity)), nil
}

func (r *Repository) EnabledHTTP(ctx context.Context) ([]HTTPTool, error) {
	owner, err := ownerID(ctx)
	if err != nil {
		return nil, err
	}
	var rows []HTTPTool
	err = r.db.WithContext(ctx).Where("account_id = ? AND enabled = ?", owner, true).Order("id asc").Find(&rows).Error
	return rows, mapDB(err)
}

func (r *Repository) FindHTTPByName(ctx context.Context, name string) (*HTTPTool, error) {
	owner, err := ownerID(ctx)
	if err != nil {
		return nil, err
	}
	var entity HTTPTool
	err = r.db.WithContext(ctx).Where("account_id = ? AND name = ?", owner, name).First(&entity).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperr.NotFound("工具不存在")
	}
	if err != nil {
		return nil, mapDB(err)
	}
	return &entity, nil
}

func (r *Repository) CreateMCP(ctx context.Context, request *MCPRequest, envCipher, headersCipher string) (response.Response[MCPResponse], error) {
	var result response.Response[MCPResponse]
	owner, err := ownerID(ctx)
	if err != nil {
		return result, err
	}
	var count int64
	if err := r.db.WithContext(ctx).Model(&MCPServer{}).Where("account_id = ?", owner).Count(&count).Error; err != nil {
		return result, mapDB(err)
	}
	if count >= 20 {
		return result, apperr.Invalid("最多创建 20 个 MCP 服务")
	}
	now := time.Now().UTC()
	entity := MCPServer{
		AccountID: owner, Name: request.Name, Transport: request.Transport, Command: request.Command,
		Args: encodeArgs(request.Args), EnvCipher: envCipher, URL: request.URL, HeadersCipher: headersCipher,
		Enabled: request.Enabled, CreatedAt: now, UpdatedAt: now,
	}
	if err := r.db.WithContext(ctx).Create(&entity).Error; err != nil {
		return result, mapDB(err)
	}
	return response.OK(mcpResponse(&entity)), nil
}

func (r *Repository) UpdateMCP(ctx context.Context, request *MCPRequest, envCipher, headersCipher *string) (response.Response[MCPResponse], error) {
	var result response.Response[MCPResponse]
	owner, err := ownerID(ctx)
	if err != nil {
		return result, err
	}
	values := map[string]any{
		"name": request.Name, "transport": request.Transport, "command": request.Command,
		"args": encodeArgs(request.Args), "url": request.URL, "enabled": request.Enabled, "updated_at": time.Now().UTC(),
	}
	if envCipher != nil {
		values["env_cipher"] = *envCipher
	}
	if headersCipher != nil {
		values["headers_cipher"] = *headersCipher
	}
	res := r.db.WithContext(ctx).Model(&MCPServer{}).Where("id = ? AND account_id = ?", request.ID, owner).Updates(values)
	if res.Error != nil {
		return result, mapDB(res.Error)
	}
	if res.RowsAffected == 0 {
		return result, apperr.NotFound("MCP 服务不存在")
	}
	return r.OneMCP(ctx, request.ID)
}

func (r *Repository) OneMCP(ctx context.Context, id int64) (response.Response[MCPResponse], error) {
	var result response.Response[MCPResponse]
	entity, err := r.findMCP(ctx, id)
	if err != nil {
		return result, err
	}
	return response.OK(mcpResponse(entity)), nil
}

func (r *Repository) SearchMCP(ctx context.Context, search *MCPSearch) (response.PageResponse[MCPSimple], error) {
	var result response.PageResponse[MCPSimple]
	owner, err := ownerID(ctx)
	if err != nil {
		return result, err
	}
	base := func() *gorm.DB {
		query := r.db.WithContext(ctx).Model(&MCPServer{}).Where("account_id = ?", owner)
		if keyword := strings.TrimSpace(search.Keyword); keyword != "" {
			query = query.Where("name LIKE ? ESCAPE '\\'", likePattern(keyword))
		}
		return query
	}
	var total int64
	if err := base().Count(&total).Error; err != nil {
		return result, mapDB(err)
	}
	var rows []MCPServer
	err = base().Order("updated_at desc, id desc").Offset((search.Page - 1) * search.PageSize).Limit(search.PageSize).Find(&rows).Error
	if err != nil {
		return result, mapDB(err)
	}
	data := make([]MCPSimple, 0, len(rows))
	for i := range rows {
		data = append(data, MCPSimple{
			ID: rows[i].ID, Name: rows[i].Name, Transport: rows[i].Transport,
			URL: rows[i].URL, Enabled: rows[i].Enabled, UpdatedAt: rows[i].UpdatedAt,
		})
	}
	return response.PageResponse[MCPSimple]{Data: data, Total: total, Page: search.Page, PageSize: search.PageSize}, nil
}

func (r *Repository) DeleteMCP(ctx context.Context, id int64) (response.Response[MCPResponse], error) {
	var result response.Response[MCPResponse]
	entity, err := r.findMCP(ctx, id)
	if err != nil {
		return result, err
	}
	owner, _ := ownerID(ctx)
	if err := r.db.WithContext(ctx).Where("id = ? AND account_id = ?", id, owner).Delete(&MCPServer{}).Error; err != nil {
		return result, mapDB(err)
	}
	return response.OK(mcpResponse(entity)), nil
}

func (r *Repository) EnabledMCP(ctx context.Context) ([]MCPServer, error) {
	owner, err := ownerID(ctx)
	if err != nil {
		return nil, err
	}
	var rows []MCPServer
	err = r.db.WithContext(ctx).Where("account_id = ? AND enabled = ?", owner, true).Order("id asc").Limit(8).Find(&rows).Error
	return rows, mapDB(err)
}

func (r *Repository) findHTTP(ctx context.Context, id int64) (*HTTPTool, error) {
	owner, err := ownerID(ctx)
	if err != nil {
		return nil, err
	}
	var entity HTTPTool
	err = r.db.WithContext(ctx).Where("id = ? AND account_id = ?", id, owner).First(&entity).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperr.NotFound("工具不存在")
	}
	if err != nil {
		return nil, mapDB(err)
	}
	return &entity, nil
}

func (r *Repository) findMCP(ctx context.Context, id int64) (*MCPServer, error) {
	owner, err := ownerID(ctx)
	if err != nil {
		return nil, err
	}
	var entity MCPServer
	err = r.db.WithContext(ctx).Where("id = ? AND account_id = ?", id, owner).First(&entity).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperr.NotFound("MCP 服务不存在")
	}
	if err != nil {
		return nil, mapDB(err)
	}
	return &entity, nil
}

func (r *Repository) ensureHTTPName(ctx context.Context, owner int64, name string, ignoreID int64) error {
	var count int64
	query := r.db.WithContext(ctx).Model(&HTTPTool{}).Where("account_id = ? AND name = ?", owner, name)
	if ignoreID > 0 {
		query = query.Where("id <> ?", ignoreID)
	}
	if err := query.Count(&count).Error; err != nil {
		return mapDB(err)
	}
	if count > 0 {
		return apperr.Conflict("已经有同名工具")
	}
	return nil
}

func httpResponse(entity *HTTPTool) HTTPToolResponse {
	return HTTPToolResponse{
		ID: entity.ID, Name: entity.Name, Description: entity.Description, Parameters: entity.Parameters,
		Method: entity.Method, URL: entity.URL, HasHeaders: entity.Headers != "", Enabled: entity.Enabled,
		CreatedAt: entity.CreatedAt, UpdatedAt: entity.UpdatedAt,
	}
}

func mcpResponse(entity *MCPServer) MCPResponse {
	args := []string{}
	_ = json.Unmarshal([]byte(entity.Args), &args)
	return MCPResponse{
		ID: entity.ID, Name: entity.Name, Transport: entity.Transport, Command: entity.Command, Args: args,
		URL: entity.URL, HasEnv: entity.EnvCipher != "", HasHeaders: entity.HeadersCipher != "", Enabled: entity.Enabled,
		CreatedAt: entity.CreatedAt, UpdatedAt: entity.UpdatedAt,
	}
}

func encodeArgs(args []string) string {
	if args == nil {
		args = []string{}
	}
	raw, _ := json.Marshal(args)
	return string(raw)
}

func ownerID(ctx context.Context) (int64, error) {
	scope, ok := domain.ScopeFrom(ctx)
	if !ok || scope.ActorID <= 0 {
		return 0, apperr.Unauthorized("未登录")
	}
	return scope.ActorID, nil
}

func likePattern(keyword string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + replacer.Replace(keyword) + "%"
}

func mapDB(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := apperr.As(err); ok {
		return err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return apperr.NotFound("记录不存在")
	}
	return apperr.Internal(err)
}
