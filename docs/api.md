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
| `POST /api/chat/stream` | 发送并流式回答，SSE |
| `POST /api/chat/optimize` | 优化提示词 |
| `POST /api/account/register` | 注册并写入会话 cookie |
| `POST /api/account/login` | 登录并写入会话 cookie |
| `POST /api/account/logout` | 退出并清除会话 |
| `POST /api/account/one` | 当前登录用户 |
| `POST /api/account/update` | 修改显示名称 |
| `POST /api/datasource/create` | 为当前账号创建数据源 |
| `POST /api/datasource/update` | 修改当前账号的数据源，密码留空表示不改 |
| `POST /api/datasource/one` | 数据源详情，不返回密码 |
| `POST /api/datasource/search` | 分页搜索当前账号的数据源 |
| `POST /api/datasource/delete` | 删除数据源，用量记录保留 |
| `POST /api/datasource/test` | 测试连接并返回可读表数量 |
| `POST /api/datasource/schema` | 读取表和字段 |
| `POST /api/chatbi/ask` | 按问题生成并执行只读 SQL |
| `POST /api/billing/one` | 余额；没有钱包时赠送额度 |
| `POST /api/billing/search` | 分页账单流水 |
| `POST /api/billing/topup` | 记一笔充值并增加额度 |
| `POST /api/usage/summary` | 按天和数据源汇总用量 |
| `POST /api/usage/search` | 分页问数记录 |
| `GET /health/live` | 存活 |
| `GET /health/ready` | 数据库可访问 |

流式事件为 `meta`、`thinking`、`delta`、`done`、`error`。标识以十进制字符串传输。
