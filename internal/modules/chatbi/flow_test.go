package chatbi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"gptgo/internal/platform/database"
	"gptgo/internal/platform/domain"
	apperr "gptgo/internal/platform/errors"
)

type fakeEngine struct {
	columns  []Column
	result   QueryResult
	queries  int
	lastSQL  string
	pingErr  error
	queryErr error
}

func (f *fakeEngine) Ping(context.Context, ConnInfo) error { return f.pingErr }
func (f *fakeEngine) Schema(context.Context, ConnInfo) ([]Column, error) {
	return f.columns, nil
}
func (f *fakeEngine) Query(_ context.Context, _ ConnInfo, query string, _ int) (QueryResult, error) {
	f.queries++
	f.lastSQL = query
	if f.queryErr != nil {
		return QueryResult{}, f.queryErr
	}
	return f.result, nil
}

type scriptCompleter struct {
	out   string
	err   error
	calls int
}

func (s *scriptCompleter) Complete(context.Context, string, string, string) (string, error) {
	s.calls++
	return s.out, s.err
}

func TestDatasourceIsPrivateAndAskBillsUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openChatBIDB(t)
	repo := NewRepository(db)
	box, err := OpenCipher(filepath.Join(t.TempDir(), "datasource.key"))
	if err != nil {
		t.Fatal(err)
	}
	engine := &fakeEngine{
		columns: demoColumns(),
		result: QueryResult{
			Columns: []string{"city", "total"},
			Rows:    [][]any{{"上海", int64(3)}, {"北京", int64(1)}},
		},
	}
	completer := &scriptCompleter{err: errors.New("offline")}
	resource := NewResource(repo, engine, completer, box)
	ctx1 := domain.WithScope(context.Background(), domain.Scope{ActorID: 1})
	ctx2 := domain.WithScope(context.Background(), domain.Scope{ActorID: 2})

	created, err := resource.Create(ctx1, &DatasourceRequest{
		Name: "销售库", Driver: "pgsql", Host: "127.0.0.1", Database: "sales", Username: "reader", Password: "p@ss/word",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Data.Driver != "postgres" || created.Data.Port != 5432 || !created.Data.PasswordSet {
		t.Fatalf("数据源没有按预期保存: %+v", created.Data)
	}
	raw, _ := json.Marshal(created.Data)
	if strings.Contains(string(raw), "p@ss/word") || strings.Contains(string(raw), `"password"`) {
		t.Fatalf("响应泄露了密码: %s", raw)
	}
	stored, err := repo.FindSource(ctx1, created.Data.ID)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := box.Decrypt(stored.PasswordCipher)
	if err != nil || plain != "p@ss/word" {
		t.Fatalf("密码应可解密: %s %v", plain, err)
	}
	if _, err := resource.Create(ctx1, &DatasourceRequest{
		Name: "销售库", Driver: "mysql", Host: "127.0.0.1", Database: "sales", Username: "reader", Password: "secret",
	}); err == nil {
		t.Fatal("同名数据源应冲突")
	}
	page, err := resource.Search(ctx2, &DatasourceSearch{Page: 1, PageSize: 20})
	if err != nil || page.Total != 0 {
		t.Fatalf("其他用户不应看到数据源: %+v %v", page, err)
	}
	if _, err := resource.One(ctx2, &DatasourceRequest{ID: created.Data.ID}); err == nil {
		t.Fatal("其他用户不能读取数据源")
	}

	asked, err := resource.Ask(ctx1, &AskRequest{DatasourceID: created.Data.ID, Question: "orders 每个 city 的销售额", Model: "builtin"})
	if err != nil {
		t.Fatal(err)
	}
	if asked.Data.Balance != WelcomeCredits-AskCost || asked.Data.Cost != AskCost || asked.Data.Chart != "bar" {
		t.Fatalf("扣费或图表不正确: %+v", asked.Data)
	}
	if !strings.Contains(asked.Data.SQL, "SUM(") || !strings.Contains(asked.Data.SQL, "LIMIT 200") || strings.Contains(strings.ToLower(asked.Data.SQL), "drop") {
		t.Fatalf("sql: %s", asked.Data.SQL)
	}
	if engine.queries != 1 {
		t.Fatalf("queries=%d", engine.queries)
	}
	wallet2, err := resource.Wallet(ctx2)
	if err != nil || wallet2.Data.Balance != WelcomeCredits {
		t.Fatalf("其他用户的额度被影响: %+v %v", wallet2.Data, err)
	}

	completer.err = nil
	completer.out = "DROP TABLE orders"
	if _, err := resource.Ask(ctx1, &AskRequest{DatasourceID: created.Data.ID, Question: "今天天气怎么样", Model: "deepseek-chat"}); err == nil {
		t.Fatal("无法落到表上的问题应失败")
	}
	if strings.Contains(strings.ToLower(engine.lastSQL), "drop") || engine.queries != 1 {
		t.Fatalf("危险 SQL 被执行了: %s queries=%d", engine.lastSQL, engine.queries)
	}

	if err := db.Model(&Wallet{}).Where("account_id = ?", 1).Update("balance", 1).Error; err != nil {
		t.Fatal(err)
	}
	_, err = resource.Ask(ctx1, &AskRequest{DatasourceID: created.Data.ID, Question: "orders 有多少条", Model: "builtin"})
	if ae, ok := apperr.As(err); !ok || ae.AppCode != 403 {
		t.Fatalf("余额不足应拒绝: %v", err)
	}

	topped, err := resource.Topup(ctx1, &TopupRequest{Amount: 100, Note: "月度额度"})
	if err != nil || topped.Data.Balance != 101 {
		t.Fatalf("充值: %+v %v", topped.Data, err)
	}
	summary, err := resource.Summarize(ctx1, &UsageSummaryRequest{Days: 7})
	if err != nil {
		t.Fatal(err)
	}
	if summary.Data.QueryCount < 2 || summary.Data.SuccessCount != 1 || summary.Data.Cost != AskCost || len(summary.Data.Days) != 7 {
		t.Fatalf("用量统计不正确: %+v", summary.Data)
	}
	if len(summary.Data.Datasources) != 1 || summary.Data.Datasources[0].DatasourceName != "销售库" {
		t.Fatalf("数据源用量不正确: %+v", summary.Data.Datasources)
	}
	usagePage, err := resource.SearchUsage(ctx2, &UsageSearch{Page: 1, PageSize: 20})
	if err != nil || usagePage.Total != 0 {
		t.Fatalf("其他用户不应看到用量: %+v %v", usagePage, err)
	}

	httpEngine := gin.New()
	httpEngine.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(domain.WithScope(c.Request.Context(), domain.Scope{ActorID: 1}))
		c.Next()
	})
	NewController(resource).Register(httpEngine.Group("/api"))
	body := postChatBI(t, httpEngine, "/api/billing/search", `{"page":1,"page_size":20}`)
	if body["total"].(float64) < 2 {
		t.Fatalf("账单应包含赠送和充值: %#v", body)
	}
	checked := postChatBI(t, httpEngine, "/api/datasource/test", `{"id":"`+created.Data.IDString()+`"}`)
	if checked["code"].(float64) != 200 || checked["data"].(map[string]any)["table_count"].(float64) != 2 {
		t.Fatalf("测试连接: %#v", checked)
	}
}

func (d DatasourceResponse) IDString() string {
	raw, _ := json.Marshal(d.ID)
	return strings.Trim(string(raw), `"`)
}

func demoColumns() []Column {
	return []Column{
		{Table: "orders", Name: "id", DataType: "int"},
		{Table: "orders", Name: "city", DataType: "varchar"},
		{Table: "orders", Name: "amount", DataType: "decimal"},
		{Table: "orders", Name: "created_at", DataType: "datetime"},
		{Table: "users", Name: "id", DataType: "int"},
		{Table: "users", Name: "name", DataType: "varchar"},
	}
}

func postChatBI(t *testing.T, engine *gin.Engine, path, body string) map[string]any {
	t.Helper()
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(recorder, req)
	if recorder.Code != 200 {
		t.Fatalf("%s http %d %s", path, recorder.Code, recorder.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	return payload
}

func openChatBIDB(t *testing.T) *gorm.DB {
	t.Helper()
	dir := t.TempDir()
	dsn := (&url.URL{Scheme: "file", Path: filepath.Join(dir, "app.db"), RawQuery: "_pragma=foreign_keys(1)"}).String()
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db, filepath.Clean(filepath.Join("..", "..", "..", "migrations"))); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := db.Exec("INSERT INTO account (name, username, password_hash, created_at, updated_at) VALUES (?, ?, '', ?, ?), (?, ?, '', ?, ?)",
		"甲", "user_a", now, now, "乙", "user_b", now, now).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	})
	return db
}
