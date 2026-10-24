# 架构

ChatBI AI 是一个本地可运行的对话应用。页面按提供的聊天首页实现，能力包括新建对话、历史分组、流式回答、停止生成、模型与思考模式、联网检索、提示词优化、文本文件附件和导出。

- `cmd/server` 只调用 `app.Run()`。
- `internal/platform` 提供配置、SQLite、日志、HTTP、错误和模型调用。
- `internal/modules/chat` 保存对话和消息。一组消息写入放在 Repository 的同一个事务里，模型调用发生在事务提交之后。
- `internal/modules/account` 保存本地显示名称。
- `web` 是 Vue 3 页面，构建后由 Go 嵌入并提供 SPA 回退。

默认不配置密钥时使用内置回答器，保证页面和流式对话可以离线演示。设置 `APP_AI_API_KEY` 后，除「内置助手」以外的模型会请求 OpenAI 兼容接口。

当前构建只打开 SQLite。配置里可以写其他 driver，但连接阶段会拒绝并说明原因，不会悄悄退回 SQLite。
