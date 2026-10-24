package chat

import (
	"context"
	"encoding/json"
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

	aiplat "gptgo/internal/platform/ai"
	"gptgo/internal/platform/database"
	"gptgo/internal/platform/domain"
)

type fakeModels struct{}

func (fakeModels) Search(context.Context, string) (string, error) { return "摘要", nil }
func (fakeModels) Optimize(context.Context, string, string) (string, error) {
	return "优化后的提示词", nil
}
func (fakeModels) Stream(_ context.Context, _ aiplat.Request) (<-chan aiplat.Delta, error) {
	ch := make(chan aiplat.Delta, 2)
	ch <- aiplat.Delta{Content: "这是测试回答"}
	close(ch)
	return ch, nil
}

func TestConversationFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openTestDB(t)
	repo := NewRepository(db)
	resource := NewResource(repo, fakeModels{}, fakeModels{}, "builtin")
	controller := NewController(resource, fakeModels{})
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		ctx := domain.WithScope(c.Request.Context(), domain.Scope{ActorID: 1, RequestID: "test"})
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	api := engine.Group("/api")
	controller.Register(api)

	ctx := domain.WithScope(context.Background(), domain.Scope{ActorID: 1, RequestID: "test"})
	if _, err := resource.Prepare(ctx, &ChatStreamRequest{Content: "   "}); err == nil {
		t.Fatal("空消息应该被拒绝")
	}

	created := postJSON(t, engine, "/api/conversation/create", `{"title":"项目周会"}`)
	if created["code"].(float64) != 200 {
		t.Fatalf("create: %#v", created)
	}
	id := created["data"].(map[string]any)["id"].(string)

	page := postJSON(t, engine, "/api/conversation/search", `{"keyword":"周会","page":1,"page_size":20}`)
	if _, ok := page["code"]; ok {
		t.Fatalf("分页结果不应该包一层 code: %#v", page)
	}
	if page["total"].(float64) != 1 || page["page"].(float64) != 1 {
		t.Fatalf("分页形状错误: %#v", page)
	}

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/chat/stream", strings.NewReader(`{"conversation_id":"`+id+`","content":"你好","model":"builtin","thinking_mode":"off"}`))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(recorder, req)
	body := recorder.Body.String()
	if !strings.Contains(body, "event: meta") || !strings.Contains(body, "这是测试回答") || !strings.Contains(body, "event: done") {
		t.Fatalf("流式输出不完整: %s", body)
	}

	missing := postJSON(t, engine, "/api/conversation/delete", `{"id":"999999"}`)
	if missing["code"].(float64) != 404 {
		t.Fatalf("缺少的对话应返回 404 业务码: %#v", missing)
	}

	messages := postJSON(t, engine, "/api/message/search", `{"conversation_id":"`+id+`","page":1,"page_size":20}`)
	if messages["total"].(float64) < 2 {
		t.Fatalf("消息没有保存: %#v", messages)
	}
}

func TestDeleteRollsBackWhenMessageDeleteFails(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := domain.WithScope(context.Background(), domain.Scope{ActorID: 1, RequestID: "test"})
	created, err := repo.Create(ctx, &ConversationRequest{Title: "保留", Model: "builtin", ThinkingMode: "off"})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := db.Select("ConversationID", "Role", "Content", "Thinking", "Status", "CreatedAt", "UpdatedAt").Create(&Message{
		ConversationID: created.Data.ID,
		Role:           "user",
		Content:        "hello",
		Status:         "done",
		CreatedAt:      now,
		UpdatedAt:      now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sqlDB.Exec("CREATE TRIGGER block_message_delete BEFORE DELETE ON message BEGIN SELECT RAISE(ABORT, 'blocked'); END"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Delete(ctx, &ConversationRequest{ID: created.Data.ID}); err == nil {
		t.Fatal("删除消息失败时应该返回错误")
	}
	var count int64
	if err := db.Model(&Conversation{}).Where("id = ?", created.Data.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("事务失败后对话不应该被删除")
	}
}

func TestConversationsStayWithOwner(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx1 := domain.WithScope(context.Background(), domain.Scope{ActorID: 1, RequestID: "a"})
	ctx2 := domain.WithScope(context.Background(), domain.Scope{ActorID: 2, RequestID: "b"})
	created, err := repo.Create(ctx1, &ConversationRequest{Title: "仅用户一", Model: "builtin", ThinkingMode: "off"})
	if err != nil {
		t.Fatal(err)
	}
	page, err := repo.Search(ctx2, &ConversationSearch{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 0 || len(page.Data) != 0 {
		t.Fatalf("其他用户不应看到对话: %#v", page)
	}
	if _, err := repo.One(ctx2, &ConversationRequest{ID: created.Data.ID}); err == nil {
		t.Fatal("其他用户不能读取对话")
	}
	if _, err := repo.Clear(ctx2, &ConversationClearRequest{Confirm: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.One(ctx1, &ConversationRequest{ID: created.Data.ID}); err != nil {
		t.Fatal("清空其他用户的对话时不应删掉自己的")
	}
}

func postJSON(t *testing.T, engine *gin.Engine, path, body string) map[string]any {
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

func openTestDB(t *testing.T) *gorm.DB {
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
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	})
	return db
}
