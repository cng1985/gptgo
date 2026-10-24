package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"

	"gptgo/internal/modules/chatbi"
	"gptgo/internal/platform/domain"
	apperr "gptgo/internal/platform/errors"
	"gptgo/internal/platform/response"
)

type Repository struct {
	db     *gorm.DB
	cipher *chatbi.Cipher
}

func NewRepository(db *gorm.DB, cipher *chatbi.Cipher) *Repository {
	return &Repository{db: db, cipher: cipher}
}

func (r *Repository) Create(ctx context.Context, request *ServerRequest) (response.Response[ServerResponse], error) {
	var result response.Response[ServerResponse]
	owner, err := ownerID(ctx)
	if err != nil {
		return result, err
	}
	var count int64
	if err := r.db.WithContext(ctx).Model(&Server{}).Where("account_id = ?", owner).Count(&count).Error; err != nil {
		return result, mapDB(err)
	}
	if count >= 20 {
		return result, apperr.Invalid("最多创建 20 个 MCP 服务")
	}
	cipherText, err := r.sealHeaders(nil, request.Headers)
	if err != nil {
		return result, err
	}
	now := time.Now().UTC()
	entity := Server{
		AccountID:    owner,
		Name:         request.Name,
		Transport:    request.Transport,
		URL:          request.URL,
		Command:      request.Command,
		Args:         encodeArgs(request.Args),
		HeaderCipher: cipherText,
		Enabled:      enabledOr(request.Enabled, true),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := r.db.WithContext(ctx).Create(&entity).Error; err != nil {
		return result, mapDB(err)
	}
	return response.OK(r.toResponse(&entity)), nil
}

func (r *Repository) Update(ctx context.Context, request *ServerRequest) (response.Response[ServerResponse], error) {
	var result response.Response[ServerResponse]
	owner, err := ownerID(ctx)
	if err != nil {
		return result, err
	}
	var current Server
	err = r.db.WithContext(ctx).Where("id = ? AND account_id = ?", request.ID, owner).First(&current).Error
	if err != nil {
		return result, mapDB(err)
	}
	cipherText, err := r.sealHeaders([]byte(current.HeaderCipher), request.Headers)
	if err != nil {
		return result, err
	}
	enabled := current.Enabled
	if request.Enabled != nil {
		enabled = *request.Enabled
	}
	now := time.Now().UTC()
	res := r.db.WithContext(ctx).Model(&Server{}).Where("id = ? AND account_id = ?", request.ID, owner).Updates(map[string]any{
		"name":          request.Name,
		"transport":     request.Transport,
		"url":           request.URL,
		"command":       request.Command,
		"args":          encodeArgs(request.Args),
		"header_cipher": cipherText,
		"enabled":       enabled,
		"updated_at":    now,
	})
	if res.Error != nil {
		return result, mapDB(res.Error)
	}
	return r.One(ctx, request)
}

func (r *Repository) One(ctx context.Context, request *ServerRequest) (response.Response[ServerResponse], error) {
	var result response.Response[ServerResponse]
	entity, err := r.owned(ctx, request.ID)
	if err != nil {
		return result, err
	}
	return response.OK(r.toResponse(entity)), nil
}

func (r *Repository) Search(ctx context.Context, search *ServerSearch) (response.PageResponse[ServerSimple], error) {
	var result response.PageResponse[ServerSimple]
	owner, err := ownerID(ctx)
	if err != nil {
		return result, err
	}
	offset := int64(search.Page-1) * int64(search.PageSize)
	base := func() *gorm.DB {
		query := r.db.WithContext(ctx).Model(&Server{}).Where("account_id = ?", owner)
		if keyword := strings.TrimSpace(search.Keyword); keyword != "" {
			query = query.Where("name LIKE ? ESCAPE '\\'", likePattern(keyword))
		}
		return query
	}
	var total int64
	if err := base().Count(&total).Error; err != nil {
		return result, mapDB(err)
	}
	var rows []Server
	if err := base().Order("updated_at desc").Order("id desc").Offset(int(offset)).Limit(search.PageSize).Find(&rows).Error; err != nil {
		return result, mapDB(err)
	}
	data := make([]ServerSimple, 0, len(rows))
	for i := range rows {
		keys := r.headerKeys(rows[i].HeaderCipher)
		data = append(data, ServerSimple{
			ID:         rows[i].ID,
			Name:       rows[i].Name,
			Transport:  rows[i].Transport,
			Enabled:    rows[i].Enabled,
			HeaderKeys: keys,
			HeadersSet: rows[i].HeaderCipher != "",
			UpdatedAt:  rows[i].UpdatedAt,
		})
	}
	return response.PageResponse[ServerSimple]{Data: data, Total: total, Page: search.Page, PageSize: search.PageSize}, nil
}

func (r *Repository) Delete(ctx context.Context, request *ServerRequest) (response.Response[ServerResponse], error) {
	var result response.Response[ServerResponse]
	entity, err := r.owned(ctx, request.ID)
	if err != nil {
		return result, err
	}
	if err := r.db.WithContext(ctx).Delete(entity).Error; err != nil {
		return result, mapDB(err)
	}
	return response.OK(r.toResponse(entity)), nil
}

func (r *Repository) Owns(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	owner, err := ownerID(ctx)
	if err != nil {
		return err
	}
	var count int64
	if err := r.db.WithContext(ctx).Model(&Server{}).Where("account_id = ? AND id IN ?", owner, ids).Count(&count).Error; err != nil {
		return mapDB(err)
	}
	if count != int64(len(ids)) {
		return apperr.Invalid("包含不属于当前账号的 MCP 服务")
	}
	return nil
}

func (r *Repository) Open(ctx context.Context, id int64) (Server, map[string]string, error) {
	entity, err := r.owned(ctx, id)
	if err != nil {
		return Server{}, nil, err
	}
	headers, err := r.decryptHeaders(entity.HeaderCipher)
	if err != nil {
		return Server{}, nil, apperr.Unavailable("请求头无法读取")
	}
	return *entity, headers, nil
}

func (r *Repository) owned(ctx context.Context, id int64) (*Server, error) {
	owner, err := ownerID(ctx)
	if err != nil {
		return nil, err
	}
	var entity Server
	err = r.db.WithContext(ctx).Where("id = ? AND account_id = ?", id, owner).First(&entity).Error
	if err != nil {
		return nil, mapDB(err)
	}
	return &entity, nil
}

func (r *Repository) sealHeaders(existing []byte, incoming json.RawMessage) (string, error) {
	trimmed := bytesTrim(incoming)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return string(existing), nil
	}
	if string(trimmed) == "{}" {
		return "", nil
	}
	headers, err := parseHeaders(trimmed)
	if err != nil {
		return "", err
	}
	if len(headers) == 0 {
		return "", nil
	}
	if r.cipher == nil {
		return "", apperr.Unavailable("请求头密钥未就绪")
	}
	raw, err := json.Marshal(headers)
	if err != nil {
		return "", apperr.Internal(err)
	}
	sealed, err := r.cipher.Encrypt(string(raw))
	if err != nil {
		return "", apperr.Internal(err)
	}
	return sealed, nil
}

func (r *Repository) decryptHeaders(encoded string) (map[string]string, error) {
	if strings.TrimSpace(encoded) == "" {
		return map[string]string{}, nil
	}
	if r.cipher == nil {
		return nil, apperr.Unavailable("请求头密钥未就绪")
	}
	plain, err := r.cipher.Decrypt(encoded)
	if err != nil {
		return nil, err
	}
	var headers map[string]string
	if err := json.Unmarshal([]byte(plain), &headers); err != nil {
		return nil, err
	}
	if headers == nil {
		headers = map[string]string{}
	}
	return headers, nil
}

func (r *Repository) headerKeys(encoded string) []string {
	headers, err := r.decryptHeaders(encoded)
	if err != nil || len(headers) == 0 {
		return []string{}
	}
	keys := make([]string, 0, len(headers))
	for key := range headers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (r *Repository) toResponse(entity *Server) ServerResponse {
	args := decodeArgs(entity.Args)
	keys := r.headerKeys(entity.HeaderCipher)
	return ServerResponse{
		ID:         entity.ID,
		Name:       entity.Name,
		Transport:  entity.Transport,
		URL:        entity.URL,
		Command:    entity.Command,
		Args:       args,
		HeaderKeys: keys,
		HeadersSet: entity.HeaderCipher != "",
		Enabled:    entity.Enabled,
		CreatedAt:  entity.CreatedAt,
		UpdatedAt:  entity.UpdatedAt,
	}
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
		return apperr.NotFound("MCP 服务不存在")
	}
	return apperr.Internal(err)
}

func enabledOr(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func encodeArgs(args []string) string {
	if args == nil {
		args = []string{}
	}
	raw, err := json.Marshal(args)
	if err != nil {
		return "[]"
	}
	return string(raw)
}

func decodeArgs(raw string) []string {
	var args []string
	if err := json.Unmarshal([]byte(raw), &args); err != nil || args == nil {
		return []string{}
	}
	return args
}

func bytesTrim(raw json.RawMessage) []byte {
	return bytes.TrimSpace(raw)
}
