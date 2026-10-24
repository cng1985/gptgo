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
| `POST /api/chat/stream` | 发送并流式回答。只调用模型，不执行问数 |
| `POST /api/chat/optimize` | 优化提示词 |
| `POST /api/ask` | 本库问数，返回结论文字和图表 |
| `POST /api/agent/create` | 创建智能体 |
| `POST /api/agent/draft` | 按一句话生成智能体草稿，不保存 |
| `POST /api/agent/update` | 修改智能体 |
| `POST /api/agent/one` | 智能体详情 |
| `POST /api/agent/search` | 分页搜索当前账号的智能体 |
| `POST /api/agent/delete` | 删除智能体，已有对话保留 |
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

流式事件为 `meta`、`thinking`、`delta`、`done`、`error`。标识以十进制字符串传输。新建对话可以带 `agent_id`，之后的回答会使用该智能体的设定。

`POST /api/agent/draft` 接收 `prompt` 和 `model`。成功时 `data` 包含名称、简介、设定、模型、开场白和建议问题，不会写入数据库。选择内置助手，或没有模型密钥时，在本地生成。描述里点名 DeepSeek 或 MiniMax 时，草稿里的模型会改成对应项。确认后仍走 `POST /api/agent/create`。

`POST /api/ask` 读取当前 SQLite 里的每一张业务表，而不是一份写死的表白名单。命中问数时，`data.chart` 带上图表类型、列、行，以及本次读到的全部表。图表类型可以是 `bar`、`line`、`pie`、`metric` 或 `table`。`data.answer` 是结论文字。密码哈希和登录令牌只显示为「已隐藏」。问数只执行单条只读 `SELECT`。没有识别成问数时 `matched` 为 false，不会查询。
