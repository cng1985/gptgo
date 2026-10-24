# HTTP 接口

普通业务接口使用 `POST` 和 JSON。成功的单条结果为 `{"code":200,"msg":"success","data":...}`，分页结果直接为 `{"data":[],"total":0,"page":1,"page_size":20}`。业务错误仍返回 HTTP 200，用 `code` 和 `msg` 表示。未知的 `/api` 路径返回 HTTP 404。

| 路径 | 作用 |
| --- | --- |
| `POST /api/conversation/create` | 创建空对话 |
| `POST /api/conversation/update` | 重命名 |
| `POST /api/conversation/one` | 对话详情 |
| `POST /api/conversation/search` | 分页搜索 |
| `POST /api/conversation/delete` | 删除对话及消息 |
| `POST /api/conversation/clear` | 确认后清空 |
| `POST /api/message/search` | 分页读取消息 |
| `POST /api/chat/stream` | 发送并流式回答。普通对话和问数都走这条 SSE |
| `POST /api/chat/optimize` | 优化提示词 |
| `POST /api/account/register` | 注册并写入会话 cookie |
| `POST /api/account/login` | 登录并写入会话 cookie |
| `POST /api/account/logout` | 退出并清除会话 |
| `POST /api/account/one` | 当前登录用户 |
| `POST /api/account/update` | 修改显示名称 |
| `GET /health/live` | 存活 |
| `GET /health/ready` | 数据库可访问 |

流式事件为 `meta`、`thinking`、`delta`、`chart`、`done`、`error`。标识以十进制字符串传输。

问数会读取当前 SQLite 里的每一张业务表，而不是一份写死的表白名单。问题命中问数时，`chart` 事件带上图表类型、列、行，以及本次读到的全部表。图表类型可以是 `bar`、`line`、`pie`、`metric` 或 `table`。同一份结果会写入助手消息的 `chart` 字段，重新打开对话时仍然能画出来。`delta` 里是结论文字。密码哈希和登录令牌只显示为「已隐藏」。问数只执行单条只读 `SELECT`。
