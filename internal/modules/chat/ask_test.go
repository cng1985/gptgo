package chat

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"gptgo/internal/platform/domain"
)

func TestAskReadsEveryTable(t *testing.T) {
	db := openTestDB(t)
	if err := db.Exec(`CREATE TABLE biz_order (id INTEGER PRIMARY KEY, region TEXT, amount INTEGER, created_at DATETIME)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE biz_empty (id INTEGER PRIMARY KEY, note TEXT)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO biz_order (region, amount, created_at) VALUES ('华东', 10, '2026-10-01 00:00:00'), ('华北', 7, '2026-10-02 00:00:00')`).Error; err != nil {
		t.Fatal(err)
	}
	var master []string
	if err := db.Raw(`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name`).Scan(&master).Error; err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(db)
	outcome, ok, err := repo.Ask(context.Background(), "问数：读取系统全部数据表，并用图表展示", "deep")
	if err != nil || !ok || outcome.Chart == nil {
		t.Fatalf("ok=%v err=%v outcome=%#v", ok, err, outcome)
	}
	if outcome.Chart.Type != "bar" {
		t.Fatalf("图表类型: %s", outcome.Chart.Type)
	}
	got := map[string]int64{}
	for _, table := range outcome.Chart.Tables {
		got[table.Name] = table.Rows
	}
	if len(got) != len(master) {
		t.Fatalf("读到 %d 张表，sqlite_master 有 %d 张: %#v master=%v", len(got), len(master), got, master)
	}
	for _, name := range master {
		rows, exists := got[name]
		if !exists {
			t.Fatalf("没有读取 %s，实际读到 %#v", name, got)
		}
		if name == "biz_order" && rows != 2 {
			t.Fatalf("biz_order 条数 = %d", rows)
		}
		if name == "biz_empty" && rows != 0 {
			t.Fatalf("空表 biz_empty 应该被读到，条数 = %d", rows)
		}
	}
	for _, row := range outcome.Chart.Rows {
		if len(row) == 0 {
			continue
		}
		delete(got, fmt.Sprint(row[0]))
	}
	if len(got) != 0 {
		t.Fatalf("柱状图缺少这些表: %#v", got)
	}
	if !strings.Contains(outcome.Answer, "已读取系统全部") || !strings.Contains(outcome.Thinking, "biz_order") {
		t.Fatalf("回答没有说明全部数据表: %s", outcome.Answer)
	}
}

func TestAskChartsCustomAndConversationTables(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	if err := db.Exec(`CREATE TABLE biz_order (id INTEGER PRIMARY KEY, region TEXT, amount INTEGER, created_at DATETIME)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO biz_order (region, amount, created_at) VALUES ('华东', 10, '2026-10-01 00:00:00'), ('华东', 5, '2026-10-02 00:00:00'), ('华北', 7, '2026-10-03 00:00:00')`).Error; err != nil {
		t.Fatal(err)
	}
	grouped, ok, err := repo.Ask(context.Background(), "biz_order 按 region 统计金额，用柱状图", "off")
	if err != nil || !ok {
		t.Fatal(ok, err)
	}
	if grouped.Chart.Type != "bar" {
		t.Fatalf("type=%s sql=%s", grouped.Chart.Type, grouped.Chart.SQL)
	}
	sums := map[string]float64{}
	for _, row := range grouped.Chart.Rows {
		sums[fmt.Sprint(row[0])] = asFloat(row[1])
	}
	if sums["华东"] != 15 || sums["华北"] != 7 {
		t.Fatalf("汇总错误: %#v sql=%s", sums, grouped.Chart.SQL)
	}

	insertConversation(t, db, "一", "builtin", "2026-10-01 00:00:00")
	insertConversation(t, db, "二", "deepseek-chat", "2026-10-02 00:00:00")
	trend, ok, err := repo.Ask(context.Background(), "对话数量按天趋势，用折线图", "off")
	if err != nil || !ok || trend.Chart.Type != "line" {
		t.Fatalf("ok=%v err=%v chart=%#v", ok, err, trend.Chart)
	}
	days := map[string]float64{}
	for _, row := range trend.Chart.Rows {
		days[fmt.Sprint(row[0])] = asFloat(row[1])
	}
	if days["2026-10-01"] != 1 || days["2026-10-02"] != 1 {
		t.Fatalf("趋势: %#v sql=%s", days, trend.Chart.SQL)
	}
	pie, ok, err := repo.Ask(context.Background(), "对话按模型画饼图", "off")
	if err != nil || !ok || pie.Chart.Type != "pie" {
		t.Fatalf("ok=%v err=%v chart=%#v", ok, err, pie.Chart)
	}
	models := map[string]float64{}
	for _, row := range pie.Chart.Rows {
		models[fmt.Sprint(row[0])] = asFloat(row[1])
	}
	if models["builtin"] != 1 || models["deepseek-chat"] != 1 {
		t.Fatalf("饼图: %#v sql=%s", models, pie.Chart.SQL)
	}
	count, ok, err := repo.Ask(context.Background(), "对话有多少条", "off")
	if err != nil || !ok || count.Chart.Type != "metric" || asFloat(count.Chart.Rows[0][0]) != 2 {
		t.Fatalf("ok=%v err=%v chart=%#v", ok, err, count.Chart)
	}
}

func TestAskHidesSecretsAndRejectsWrites(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	if err := db.Exec(`INSERT INTO account (name, username, password_hash, created_at, updated_at) VALUES ('ada', 'ada', 'secret-hash', '2026-10-01 00:00:00', '2026-10-01 00:00:00')`).Error; err != nil {
		t.Fatal(err)
	}
	listed, ok, err := repo.Ask(context.Background(), "列出 account", "off")
	if err != nil || !ok {
		t.Fatal(ok, err)
	}
	for _, column := range listed.Chart.Columns {
		if strings.EqualFold(column, "password_hash") {
			t.Fatalf("明细不应返回密码列: %#v", listed.Chart.Columns)
		}
	}
	if strings.Contains(listed.Answer, "secret-hash") {
		t.Fatal("回答泄露了密码哈希")
	}
	selected, ok, err := repo.Ask(context.Background(), "SELECT password_hash, username FROM account", "off")
	if err != nil || !ok {
		t.Fatal(ok, err)
	}
	foundHidden := false
	for _, row := range selected.Chart.Rows {
		for _, cell := range row {
			if cell == "secret-hash" {
				t.Fatal("查询结果泄露了密码哈希")
			}
			if cell == "已隐藏" {
				foundHidden = true
			}
		}
	}
	if !foundHidden || !strings.Contains(fmt.Sprint(selected.Chart.Rows), "ada") {
		t.Fatalf("脱敏结果异常: %#v", selected.Chart.Rows)
	}
	rejected, ok, err := repo.Ask(context.Background(), "DROP TABLE account", "off")
	if err != nil || !ok || !strings.Contains(rejected.Answer, "只读") {
		t.Fatalf("ok=%v err=%v answer=%s", ok, err, rejected.Answer)
	}
	var left int64
	if err := db.Raw("SELECT COUNT(*) FROM account").Scan(&left).Error; err != nil {
		t.Fatal(err)
	}
	if left != 1 {
		t.Fatalf("写入语句被执行了: %d", left)
	}
	if err := db.Exec(`CREATE TABLE biz_order (id INTEGER PRIMARY KEY, region TEXT NOT NULL)`).Error; err != nil {
		t.Fatal(err)
	}
	if _, _, err := repo.Ask(context.Background(), `统计 biz_order"; DROP TABLE account; --`, "off"); err != nil {
		t.Fatal(err)
	}
	chained, ok, err := repo.Ask(context.Background(), "SELECT username FROM account; DROP TABLE account", "off")
	if err != nil || !ok || !strings.Contains(chained.Answer, "只读") {
		t.Fatalf("ok=%v err=%v answer=%s", ok, err, chained.Answer)
	}
	if err := db.Raw("SELECT COUNT(*) FROM account").Scan(&left).Error; err != nil {
		t.Fatal(err)
	}
	if left != 1 {
		t.Fatal("拼接进去的删除语句被执行了")
	}
	fields, ok, err := repo.Ask(context.Background(), "account 有哪些字段", "off")
	if err != nil || !ok || fields.Chart.Type != "table" {
		t.Fatal(ok, err, fields.Chart)
	}
	joined := fmt.Sprint(fields.Chart.Rows)
	if !strings.Contains(joined, "password_hash") || !strings.Contains(joined, "username") {
		t.Fatalf("字段列表不完整: %s", joined)
	}
	ordinary, ok, err := repo.Ask(context.Background(), "你好", "off")
	if err != nil || ok || ordinary.Chart != nil {
		t.Fatalf("普通聊天不应走问数 ok=%v err=%v", ok, err)
	}
}

func TestAskEndpointReturnsChart(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openTestDB(t)
	if err := db.Exec(`CREATE TABLE biz_order (id INTEGER PRIMARY KEY, region TEXT, amount INTEGER)`).Error; err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(db)
	resource := NewResource(repo, nil, fakeModels{}, "builtin")
	controller := NewController(resource, fakeModels{}, nil)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		ctx := domain.WithScope(c.Request.Context(), domain.Scope{ActorID: 1, RequestID: "ask"})
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	controller.Register(engine.Group("/api"))

	payload := postJSON(t, engine, "/api/ask", `{"question":"问数：读取系统全部数据表，并用图表展示","thinking_mode":"off"}`)
	if payload["code"].(float64) != 200 {
		t.Fatalf("ask: %#v", payload)
	}
	data := payload["data"].(map[string]any)
	if data["matched"] != true {
		t.Fatalf("应该识别为问数: %#v", data)
	}
	chart, _ := data["chart"].(map[string]any)
	if chart == nil || chart["type"] != "bar" {
		t.Fatalf("问数没有返回图表: %#v", data)
	}
	found := false
	for _, item := range chart["tables"].([]any) {
		if item.(map[string]any)["name"] == "biz_order" {
			found = true
		}
	}
	if !found {
		t.Fatalf("图表没有包含新建表: %#v", chart["tables"])
	}

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/chat/stream", strings.NewReader(`{"content":"问数：读取系统全部数据表，并用图表展示","model":"builtin","thinking_mode":"off"}`))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(recorder, req)
	body := recorder.Body.String()
	if recorder.Code != 200 || !strings.Contains(body, "这是测试回答") || strings.Contains(body, "event: chart") {
		t.Fatalf("普通对话不应再执行问数: %s", body)
	}
}

func insertConversation(t *testing.T, db *gorm.DB, title, model, at string) {
	t.Helper()
	err := db.Exec(
		`INSERT INTO conversation (title, model, thinking_mode, web_search, created_at, updated_at, account_id) VALUES (?, ?, 'off', 0, ?, ?, 1)`,
		title, model, at, at,
	).Error
	if err != nil {
		t.Fatal(err)
	}
}

func asFloat(value any) float64 {
	switch typed := value.(type) {
	case int64:
		return float64(typed)
	case int:
		return float64(typed)
	case float64:
		return typed
	case float32:
		return float64(typed)
	default:
		var out float64
		fmt.Sscan(fmt.Sprint(typed), &out)
		return out
	}
}
