package chatbi

import (
	"context"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	apperr "gptgo/internal/platform/errors"
	"gptgo/internal/platform/response"
)

var (
	databasePattern = regexp.MustCompile(`^[\p{L}\p{N}_.-]{1,128}$`)
	usernamePattern = regexp.MustCompile(`^[\p{L}\p{N}_.@\\-]{1,128}$`)
)

type Completer interface {
	Complete(ctx context.Context, system, user, model string) (string, error)
}

type Resource struct {
	repo      *Repository
	engine    QueryEngine
	completer Completer
	cipher    *Cipher
}

func NewResource(repo *Repository, engine QueryEngine, completer Completer, cipher *Cipher) *Resource {
	return &Resource{repo: repo, engine: engine, completer: completer, cipher: cipher}
}

func (r *Resource) Create(ctx context.Context, request *DatasourceRequest) (response.Response[DatasourceResponse], error) {
	var zero response.Response[DatasourceResponse]
	input, err := normalizeSource(request, true)
	if err != nil {
		return zero, err
	}
	cipherText, err := r.cipher.Encrypt(input.Password)
	if err != nil {
		return zero, apperr.Internal(err)
	}
	entity := Datasource{
		Name:           input.Name,
		Driver:         input.Driver,
		Host:           input.Host,
		Port:           input.Port,
		DatabaseName:   input.Database,
		Username:       input.Username,
		PasswordCipher: cipherText,
		SSLMode:        input.SSLMode,
	}
	if err := r.repo.CreateSource(ctx, &entity); err != nil {
		return zero, err
	}
	return response.OK(*DatasourceEntityToResponse(&entity)), nil
}

func (r *Resource) Update(ctx context.Context, request *DatasourceRequest) (response.Response[DatasourceResponse], error) {
	var zero response.Response[DatasourceResponse]
	if request == nil || request.ID <= 0 {
		return zero, apperr.NotFound("数据源不存在")
	}
	input, err := normalizeSource(request, false)
	if err != nil {
		return zero, err
	}
	entity, err := r.repo.FindSource(ctx, request.ID)
	if err != nil {
		return zero, err
	}
	entity.Name = input.Name
	entity.Driver = input.Driver
	entity.Host = input.Host
	entity.Port = input.Port
	entity.DatabaseName = input.Database
	entity.Username = input.Username
	entity.SSLMode = input.SSLMode
	if input.Password != "" {
		cipherText, err := r.cipher.Encrypt(input.Password)
		if err != nil {
			return zero, apperr.Internal(err)
		}
		entity.PasswordCipher = cipherText
	}
	if entity.PasswordCipher == "" {
		return zero, apperr.Invalid("请填写数据库密码")
	}
	if err := r.repo.SaveSource(ctx, &entity); err != nil {
		return zero, err
	}
	return response.OK(*DatasourceEntityToResponse(&entity)), nil
}

func (r *Resource) One(ctx context.Context, request *DatasourceRequest) (response.Response[DatasourceResponse], error) {
	var zero response.Response[DatasourceResponse]
	if request == nil || request.ID <= 0 {
		return zero, apperr.NotFound("数据源不存在")
	}
	entity, err := r.repo.FindSource(ctx, request.ID)
	if err != nil {
		return zero, err
	}
	return response.OK(*DatasourceEntityToResponse(&entity)), nil
}

func (r *Resource) Search(ctx context.Context, search *DatasourceSearch) (response.PageResponse[DatasourceSimple], error) {
	var zero response.PageResponse[DatasourceSimple]
	if search == nil {
		return zero, apperr.Invalid("参数格式错误")
	}
	page, size, err := normalizePage(search.Page, search.PageSize, 50)
	if err != nil {
		return zero, err
	}
	search.Page, search.PageSize = page, size
	if utf8.RuneCountInString(search.Keyword) > 100 {
		return zero, apperr.Invalid("搜索关键词过长")
	}
	return r.repo.SearchSource(ctx, search)
}

func (r *Resource) Delete(ctx context.Context, request *DatasourceRequest) (response.Response[DatasourceResponse], error) {
	var zero response.Response[DatasourceResponse]
	if request == nil || request.ID <= 0 {
		return zero, apperr.NotFound("数据源不存在")
	}
	entity, err := r.repo.FindSource(ctx, request.ID)
	if err != nil {
		return zero, err
	}
	if err := r.repo.DeleteSource(ctx, request.ID); err != nil {
		return zero, err
	}
	return response.OK(*DatasourceEntityToResponse(&entity)), nil
}

func (r *Resource) Test(ctx context.Context, request *DatasourceRequest) (response.Response[DatasourceTestResponse], error) {
	var zero response.Response[DatasourceTestResponse]
	info, err := r.connFor(ctx, request, false)
	if err != nil {
		return zero, err
	}
	if err := r.engine.Ping(ctx, info); err != nil {
		return zero, connectErr(err, info.Password)
	}
	columns, err := r.engine.Schema(ctx, info)
	if err != nil {
		return zero, connectErr(err, info.Password)
	}
	return response.OK(DatasourceTestResponse{OK: true, Message: "连接成功", TableCount: len(groupSchema(columns))}), nil
}

func (r *Resource) Schema(ctx context.Context, request *DatasourceRequest) (response.Response[SchemaResponse], error) {
	var zero response.Response[SchemaResponse]
	if request == nil || request.ID <= 0 {
		return zero, apperr.NotFound("数据源不存在")
	}
	info, err := r.connFor(ctx, request, true)
	if err != nil {
		return zero, err
	}
	columns, err := r.engine.Schema(ctx, info)
	if err != nil {
		return zero, connectErr(err, info.Password)
	}
	tables := groupSchema(columns)
	if tables == nil {
		tables = []TableInfo{}
	}
	return response.OK(SchemaResponse{Tables: tables}), nil
}

func (r *Resource) connFor(ctx context.Context, request *DatasourceRequest, storedOnly bool) (ConnInfo, error) {
	if request == nil {
		return ConnInfo{}, apperr.Invalid("参数格式错误")
	}
	if request.ID > 0 {
		entity, err := r.repo.FindSource(ctx, request.ID)
		if err != nil {
			return ConnInfo{}, err
		}
		if !storedOnly && strings.TrimSpace(request.Host) != "" {
			input, err := normalizeSource(request, false)
			if err != nil {
				return ConnInfo{}, err
			}
			password := input.Password
			if password == "" {
				password, err = r.passwordOf(entity, "")
				if err != nil {
					return ConnInfo{}, err
				}
			}
			return ConnInfo{
				Driver: input.Driver, Host: input.Host, Port: input.Port, Database: input.Database,
				Username: input.Username, Password: password, SSLMode: input.SSLMode,
			}, nil
		}
		password, err := r.passwordOf(entity, "")
		if err != nil {
			return ConnInfo{}, err
		}
		return connFrom(entity, password), nil
	}
	input, err := normalizeSource(request, true)
	if err != nil {
		return ConnInfo{}, err
	}
	return ConnInfo{
		Driver: input.Driver, Host: input.Host, Port: input.Port, Database: input.Database,
		Username: input.Username, Password: input.Password, SSLMode: input.SSLMode,
	}, nil
}

func (r *Resource) passwordOf(entity Datasource, override string) (string, error) {
	if override != "" {
		return override, nil
	}
	if entity.PasswordCipher == "" {
		return "", apperr.Invalid("请填写数据库密码")
	}
	plain, err := r.cipher.Decrypt(entity.PasswordCipher)
	if err != nil {
		return "", apperr.Invalid("数据源密码无法读取，请重新保存")
	}
	return plain, nil
}

func connFrom(entity Datasource, password string) ConnInfo {
	return ConnInfo{
		Driver: entity.Driver, Host: entity.Host, Port: entity.Port, Database: entity.DatabaseName,
		Username: entity.Username, Password: password, SSLMode: entity.SSLMode,
	}
}

type sourceInput struct {
	Name     string
	Driver   string
	Host     string
	Port     int
	Database string
	Username string
	Password string
	SSLMode  string
}

func normalizeSource(request *DatasourceRequest, requirePassword bool) (sourceInput, error) {
	var zero sourceInput
	if request == nil {
		return zero, apperr.Invalid("参数格式错误")
	}
	name := strings.TrimSpace(request.Name)
	if !validLabel(name, 40) {
		return zero, apperr.Invalid("数据源名称需要是 1 到 40 个字符")
	}
	driver, err := CanonicalDriver(request.Driver)
	if err != nil {
		return zero, err
	}
	host := strings.TrimSpace(request.Host)
	if !validHost(host) {
		return zero, apperr.Invalid("数据库地址无效")
	}
	port := request.Port
	if port == 0 {
		port = defaultPort(driver)
	}
	if port < 1 || port > 65535 {
		return zero, apperr.Invalid("端口需要在 1 到 65535 之间")
	}
	database := strings.TrimSpace(request.Database)
	if !databasePattern.MatchString(database) {
		return zero, apperr.Invalid("库名只能包含字母、数字、下划线、点和连字符")
	}
	username := strings.TrimSpace(request.Username)
	if !usernamePattern.MatchString(username) {
		return zero, apperr.Invalid("数据库用户名无效")
	}
	password := request.Password
	if requirePassword || password != "" {
		if !validSecret(password) {
			return zero, apperr.Invalid("数据库密码需要是 1 到 256 个字符")
		}
	}
	ssl, err := normalizeSSL(driver, request.SSLMode)
	if err != nil {
		return zero, err
	}
	return sourceInput{
		Name: name, Driver: driver, Host: host, Port: port, Database: database,
		Username: username, Password: password, SSLMode: ssl,
	}, nil
}

func normalizeSSL(driver, raw string) (string, error) {
	mode := strings.ToLower(strings.TrimSpace(raw))
	switch driver {
	case "postgres":
		if mode == "" {
			mode = "disable"
		}
		switch mode {
		case "disable", "require", "verify-ca", "verify-full":
			return mode, nil
		default:
			return "", apperr.Invalid("不支持的 SSL 模式")
		}
	case "sqlserver":
		if mode == "" {
			mode = "disable"
		}
		switch mode {
		case "disable", "require":
			return mode, nil
		default:
			return "", apperr.Invalid("不支持的 SSL 模式")
		}
	default:
		if mode == "" || mode == "disable" {
			return "", nil
		}
		if mode == "require" {
			return "require", nil
		}
		return "", apperr.Invalid("不支持的 SSL 模式")
	}
}

func defaultPort(driver string) int {
	switch driver {
	case "postgres":
		return 5432
	case "sqlserver":
		return 1433
	default:
		return 3306
	}
}

func validHost(host string) bool {
	if host == "" || len(host) > 253 || strings.ContainsAny(host, " \t\r\n/\\@?#") {
		return false
	}
	return true
}

func validLabel(name string, max int) bool {
	runes := []rune(name)
	if len(runes) < 1 || len(runes) > max {
		return false
	}
	for _, r := range runes {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func validSecret(password string) bool {
	count := utf8.RuneCountInString(password)
	if count < 1 || count > 256 {
		return false
	}
	for _, r := range password {
		if unicode.IsControl(r) {
			return false
		}
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

func groupSchema(columns []Column) []TableInfo {
	tables := groupTables(columns)
	out := make([]TableInfo, 0, len(tables))
	for _, table := range tables {
		item := TableInfo{Schema: table.Schema, Name: table.Name, Columns: make([]ColumnInfo, 0, len(table.Columns))}
		for _, column := range table.Columns {
			item.Columns = append(item.Columns, ColumnInfo{Name: column.Name, DataType: column.DataType})
		}
		out = append(out, item)
	}
	return out
}

func connectErr(err error, secret string) error {
	if err == nil {
		return nil
	}
	if _, ok := apperr.As(err); ok {
		return err
	}
	return apperr.New(apperr.CategoryUnavailable, 503, "连接数据源失败，请检查地址、端口、账号和库名", errString(redact(err.Error(), secret)))
}

func queryFailed(err error, secret string) error {
	if err == nil {
		return nil
	}
	if existing, ok := apperr.As(err); ok {
		return existing
	}
	return apperr.Invalid("查询没有成功：" + redact(err.Error(), secret))
}

func redact(msg, secret string) string {
	if secret != "" {
		msg = strings.ReplaceAll(msg, secret, "***")
	}
	msg = strings.Join(strings.Fields(msg), " ")
	runes := []rune(msg)
	if len(runes) > 180 {
		msg = string(runes[:180])
	}
	return msg
}

type errorString string

func (e errorString) Error() string { return string(e) }

func errString(msg string) error {
	if msg == "" {
		return errorString("连接失败")
	}
	return errorString(msg)
}
