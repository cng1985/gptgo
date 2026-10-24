# 架构

ChatBI AI 是一个本地可运行的对话应用。页面按提供的聊天首页实现，能力包括新建对话、历史分组、流式回答、停止生成、模型与思考模式、智能体、提示词优化、文本文件附件和导出。问数是单独的页面，不从普通对话里触发。

- `cmd/server` 只调用 `app.Run()`。
- `internal/platform` 提供配置、SQLite、日志、HTTP、错误和模型调用。
- `internal/modules/chat` 保存对话和消息。一组消息写入放在 Repository 的同一个事务里，模型调用发生在事务提交之后。
- `internal/modules/account` 保存账号。开启认证后负责注册、登录、退出和会话；关闭时仍使用本机单用户。对话按 `account_id` 归属当前账号。
- `internal/modules/agent` 保存每个人自己的智能体。对话可以绑定一个智能体，模型调用时带上它的设定。
- `internal/modules/chatbi` 保存每个人自己的 MySQL、PostgreSQL、SQL Server 数据源，并把自然语言问题变成单条只读 SQL。外部数据源问数、赠送、充值和扣费写入用量与账单，查询都带 `account_id`。
- `web` 是 Vue 3 页面，构建后由 Go 嵌入并提供 SPA 回退。

默认不配置密钥时使用内置回答器，保证页面和流式对话可以离线演示。设置 `APP_AI_API_KEY` 后，除「内置助手」以外的模型会请求 OpenAI 兼容接口。

问数走 `POST /api/ask`，只在问数页面调用。它从 `sqlite_master` 读取全部业务表和字段，按问题生成只读查询，并把图表放进响应。普通对话只调用模型，不会执行问数。智能体的设定会写进模型的系统提示，内置回答器也会标明当前智能体。

当前构建只打开 SQLite。配置里可以写其他 driver，但连接阶段会拒绝并说明原因，不会悄悄退回 SQLite。
