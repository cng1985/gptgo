package account

import (
	"context"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"

	"gptgo/internal/platform/domain"
	apperr "gptgo/internal/platform/errors"
	"gptgo/internal/platform/response"
)

const SessionCookie = "chatbi_session"

var usernamePattern = regexp.MustCompile(`^[\p{Han}a-zA-Z0-9_]{2,20}$`)

type ProviderStatus interface {
	Info() (string, bool)
}

type Resource struct {
	repo     *Repository
	provider ProviderStatus
	enabled  bool
	ttl      time.Duration
}

func NewResource(repo *Repository, provider ProviderStatus, enabled bool, ttl time.Duration) *Resource {
	if ttl <= 0 {
		ttl = 8 * time.Hour
	}
	return &Resource{repo: repo, provider: provider, enabled: enabled, ttl: ttl}
}

func (r *Resource) One(ctx context.Context, request *AccountRequest) (response.Response[AccountResponse], error) {
	var zero response.Response[AccountResponse]
	if err := requireActor(ctx); err != nil {
		return zero, err
	}
	result, err := r.repo.One(ctx, request)
	if err != nil {
		return zero, err
	}
	r.decorate(&result)
	return result, nil
}

func (r *Resource) Update(ctx context.Context, request *AccountRequest) (response.Response[AccountResponse], error) {
	var zero response.Response[AccountResponse]
	if err := requireActor(ctx); err != nil {
		return zero, err
	}
	if request == nil {
		return zero, apperr.Invalid("参数格式错误")
	}
	request.Name = strings.TrimSpace(request.Name)
	if !validName(request.Name) {
		return zero, apperr.Invalid("名称需要是 1 到 20 个字符")
	}
	result, err := r.repo.Update(ctx, request)
	if err != nil {
		return zero, err
	}
	r.decorate(&result)
	return result, nil
}

func (r *Resource) decorate(result *response.Response[AccountResponse]) {
	name, ready := r.provider.Info()
	result.Data.Provider = name
	result.Data.ProviderReady = ready
}

func requireActor(ctx context.Context) error {
	if _, ok := domain.ScopeFrom(ctx); !ok {
		return apperr.Unauthorized("未登录")
	}
	return nil
}

func (r *Resource) Register(ctx context.Context, request *AuthRequest) (AccountResponse, string, error) {
	var zero AccountResponse
	if err := r.requireAuth(); err != nil {
		return zero, "", err
	}
	if request == nil {
		return zero, "", apperr.Invalid("参数格式错误")
	}
	username, err := normalizeUsername(request.Username)
	if err != nil {
		return zero, "", err
	}
	if err := validPassword(request.Password); err != nil {
		return zero, "", err
	}
	name := strings.TrimSpace(request.Name)
	if name == "" {
		name = strings.TrimSpace(request.Username)
	}
	if !validName(name) {
		return zero, "", apperr.Invalid("名称需要是 1 到 20 个字符")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(request.Password), bcrypt.DefaultCost)
	if err != nil {
		return zero, "", apperr.Internal(err)
	}
	entity := Account{Name: name, Username: username, PasswordHash: string(hash)}
	token, err := r.repo.CreateWithSession(ctx, &entity, r.ttl)
	if err != nil {
		return zero, "", err
	}
	result := AccountEntityToResponse(&entity, "", false)
	r.decorateResponse(result)
	return *result, token, nil
}

func (r *Resource) Login(ctx context.Context, request *AuthRequest) (AccountResponse, string, error) {
	var zero AccountResponse
	if err := r.requireAuth(); err != nil {
		return zero, "", err
	}
	if request == nil {
		return zero, "", apperr.Invalid("参数格式错误")
	}
	username, err := normalizeUsername(request.Username)
	if err != nil {
		return zero, "", apperr.Unauthorized("用户名或密码错误")
	}
	if strings.TrimSpace(request.Password) == "" {
		return zero, "", apperr.Unauthorized("用户名或密码错误")
	}
	entity, err := r.repo.FindByUsername(ctx, username)
	if err != nil {
		if ae, ok := apperr.As(err); ok && ae.Cat == apperr.CategoryNotFound {
			return zero, "", apperr.Unauthorized("用户名或密码错误")
		}
		return zero, "", err
	}
	if bcrypt.CompareHashAndPassword([]byte(entity.PasswordHash), []byte(request.Password)) != nil {
		return zero, "", apperr.Unauthorized("用户名或密码错误")
	}
	token, err := r.repo.IssueSession(ctx, entity.ID, r.ttl)
	if err != nil {
		return zero, "", err
	}
	result := AccountEntityToResponse(&entity, "", false)
	r.decorateResponse(result)
	return *result, token, nil
}

func (r *Resource) Logout(ctx context.Context, token string) error {
	return r.repo.DeleteSession(ctx, token)
}

func (r *Resource) requireAuth() error {
	if !r.enabled {
		return apperr.Forbidden("当前未开启登录注册")
	}
	return nil
}

func (r *Resource) decorateResponse(result *AccountResponse) {
	name, ready := r.provider.Info()
	result.Provider = name
	result.ProviderReady = ready
}

func normalizeUsername(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if !usernamePattern.MatchString(name) {
		return "", apperr.Invalid("用户名需要是 2 到 20 位字母、数字、下划线或汉字")
	}
	return strings.ToLower(name), nil
}

func validPassword(password string) error {
	count := utf8.RuneCountInString(password)
	if count < 6 || count > 64 || len(password) > 72 {
		return apperr.Invalid("密码需要是 6 到 64 个字符")
	}
	return nil
}

func validName(name string) bool {
	runes := []rune(name)
	if len(runes) < 1 || len(runes) > 20 {
		return false
	}
	for _, r := range runes {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
