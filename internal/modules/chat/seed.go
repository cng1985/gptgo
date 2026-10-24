package chat

import (
	"time"

	"gorm.io/gorm"
	aiplat "gptgo/internal/platform/ai"
)

func SeedDemo(db *gorm.DB) error {
	var count int64
	if err := db.Model(&Conversation{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	now := time.Now()
	localNoon := time.Date(now.Year(), now.Month(), now.Day(), 15, 0, 0, 0, now.Location())
	yesterday := localNoon.Add(-24 * time.Hour)
	type item struct {
		title string
		at    time.Time
		ask   string
	}
	items := []item{
		{title: "怎么设计针对用户限流，整体对接口做分层防护", at: now, ask: "怎么设计针对用户限流，整体对接口做分层防护"},
		{title: "帮我规划一下今天最重要的三件事", at: yesterday, ask: "帮我规划一下今天最重要的三件事"},
		{title: "使用jpa以后jar大了好多", at: yesterday.Add(-20 * time.Minute), ask: "使用jpa以后jar大了好多，怎么看是哪里变大的"},
		{title: "go java 开发erp那个更好", at: yesterday.Add(-40 * time.Minute), ask: "go java 开发erp那个更好"},
		{title: "sql server 取整", at: yesterday.Add(-60 * time.Minute), ask: "sql server 取整有哪些写法"},
		{title: "go fx用法", at: yesterday.Add(-80 * time.Minute), ask: "go fx用法，构造函数里能做什么"},
		{title: "go 函数返回值", at: yesterday.Add(-100 * time.Minute), ask: "go 函数返回值怎么写，多个返回值如何处理错误"},
		{title: "go 函数指针", at: yesterday.Add(-120 * time.Minute), ask: "go 函数指针和函数值有什么区别"},
		{title: "go 管道", at: yesterday.Add(-140 * time.Minute), ask: "go 管道怎么创建、发送和关闭"},
		{title: "go chan struct{} 是什么意思", at: yesterday.Add(-160 * time.Minute), ask: "go chan struct{} 是什么意思"},
		{title: "go 定义一个函数", at: yesterday.Add(-180 * time.Minute), ask: "go 定义一个函数的基本写法"},
		{title: "go context.Context 怎么在请求里传递", at: yesterday.Add(-200 * time.Minute), ask: "go context.Context 怎么在请求里传递"},
		{title: "设计一个go项目结构", at: localNoon.Add(-72 * time.Hour), ask: "设计一个go项目结构，适合业务系统"},
		{title: "用生活中的例子解释大语言模型是怎么工作的", at: localNoon.Add(-120 * time.Hour), ask: "用生活中的例子解释大语言模型是怎么工作的"},
	}
	return db.Transaction(func(tx *gorm.DB) error {
		for i, item := range items {
			at := item.at.UTC()
			mode := "off"
			switch i {
			case 0:
				mode = "deep"
			case 1:
				mode = "light"
			}
			conversation := Conversation{
				AccountID:    1,
				Title:        item.title,
				Model:        "minimax-m3.1-flash",
				ThinkingMode: mode,
				CreatedAt:    at,
				UpdatedAt:    at,
			}
			if err := tx.Create(&conversation).Error; err != nil {
				return err
			}
			thinking, answer := aiplat.Compose(item.ask, "", mode)
			user := Message{
				ConversationID: conversation.ID,
				Role:           "user",
				Content:        item.ask,
				Status:         "done",
				CreatedAt:      at,
				UpdatedAt:      at,
			}
			assistant := Message{
				ConversationID: conversation.ID,
				Role:           "assistant",
				Content:        answer,
				Thinking:       thinking,
				ThinkingMode:   mode,
				Status:         "done",
				CreatedAt:      at.Add(time.Second),
				UpdatedAt:      at.Add(time.Second),
			}
			if err := tx.Select("ConversationID", "Role", "Content", "Thinking", "Status", "CreatedAt", "UpdatedAt").Create(&user).Error; err != nil {
				return err
			}
			if err := tx.Select("ConversationID", "Role", "Content", "Thinking", "ThinkingMode", "Status", "CreatedAt", "UpdatedAt").Create(&assistant).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
