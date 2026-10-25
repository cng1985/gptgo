# ChatBI AI

一个按聊天首页实现的对话应用：左侧历史、空状态建议、底部输入框，以及连续的流式回答。

## 运行

```bash
cd web && npm install && npm run build && cd ..
go run ./cmd/server
```

浏览器打开 `http://127.0.0.1:8080`。

首次启动使用 SQLite `data/app.db`。默认开启登录注册，打开页面后先注册或登录，对话按账号分开保存。把 `auth.enabled` 设为 `false` 会回到本机单用户：自动使用显示名 ada，并在空库时写入示例对话。生产配置 `application-pro.yaml` 关闭示例数据。

```bash
go test ./...
go run ./cmd/server --version
```

## 模型

默认 `ai.provider=auto`。没有密钥时使用内置回答器，对话、历史和流式输出仍然可用。接入 OpenAI 兼容接口：

```bash
APP_AI_API_KEY=你的密钥 APP_AI_BASE_URL=https://api.openai.com/v1 APP_AI_UPSTREAM_MODEL=gpt-4o-mini go run ./cmd/server
```

模型列表里的「内置助手」始终走本地回答器。

## 问数

问数是单独的页面，不从普通对话里触发。打开侧栏里的「问数」，可以查询当前应用库的全部数据表，例如「读取系统全部数据表，并用图表展示」「对话按模型画饼图」「列出 account」。结果直接画成柱状图、折线图、饼图或表格。密码哈希和登录令牌不会显示原文。

同一页下面可以继续查询自己配置的 MySQL、PostgreSQL 或 SQL Server。那部分只生成并执行单条只读 SQL。

## 技能、工具与 MCP

对话可以同时使用技能、内置工具、HTTP 工具和 MCP。内置的当前时间和计算器默认开启，在「技能与工具」里可以关闭。

技能是带 `SKILL.md` 的说明包。页面可以上传 `.zip` 或单个 `SKILL.md`，也可以按关键词搜索 SkillHub 并安装公开技能。安装后默认启用。模型需要时会调用 `load_skill` 读取说明，再按说明回答。包里的脚本只作为文件保存，不会执行。

同一页可以添加 HTTP 工具。模型调用时按配置的地址发送 GET 或 POST，参数来自模型，请求头加密后保存在本机。

MCP 支持两种传输。stdio 直接启动命令，不经过 shell。HTTP 使用 JSON-RPC。保存后可以测试连接，并看到对方提供的工具。已启用的 MCP 工具会在对话里交给模型。调用过程显示在回答上方，并随消息保存。

没有模型密钥时，内置助手会在要求计算、询问当前时间、写明「调用 工具名」，或写明「使用技能 名称」时使用这些能力。

SkillHub 地址默认是 `https://skill.xfyun.cn`。私有注册中心可以改配置，令牌只放在服务端：

```yaml
skillhub:
  base_url: https://skill.example
  token: "你的令牌"
  timeout: 20s
```

环境变量是 `APP_SKILLHUB_BASE_URL`、`APP_SKILLHUB_TOKEN` 和 `APP_SKILLHUB_TIMEOUT`。把 `skillhub.base_url` 留空后，上传仍然可用，搜索和安装会提示还没有配置地址。

## 联网搜索

输入框里的「联网」打开后，这次提问会先检索公开网页，再把摘要交给模型。来源显示在回答上方，并随消息保存。默认使用 DuckDuckGo 和中文维基百科，不需要密钥。检索只访问这两个服务，不会打开结果链接。

可以改成 Bing 或 Tavily：

```yaml
search:
  provider: tavily
  api_key: "你的密钥"
  timeout: 8s
```

环境变量是 `APP_SEARCH_PROVIDER`、`APP_SEARCH_API_KEY` 和 `APP_SEARCH_TIMEOUT`。把 `search.provider` 设为 `off` 后，开关不会拿到外部结果。

## 技能

技能保存在当前账号下。在「技能」页写下 Python、Node.js 或 Bash 脚本，保存后对话可以按调用名执行它。模型只传入 JSON，脚本从标准输入读取，标准输出回到回答里。页面上可以先试运行，不必先保存。

没有模型密钥时，内置助手会在写明「调用 技能名」时执行已启用的技能。

## 智能体

智能体保存在当前账号下，包含名称、简介、模型、设定、开场白和建议问题。在智能体页面用一句话描述需求，可以生成一份草稿并填进表单，确认后才保存。没有密钥或选择内置助手时，草稿在本地生成。从侧栏或智能体页面开始对话后，这次对话会一直带上该设定。删除智能体不会删除已经产生的对话。

## 项目约定

- 后端是 Go、Gin、GORM 和 Fx。业务输入输出保持一个 Request/Search 对一个 Response/PageResponse。
- 前端是 Vue 3、JavaScript 和 Vite。
- 应用库默认且当前唯一完成验证的是 SQLite。配置里的 MySQL、PostgreSQL、SQL Server 只保留应用库配置位，连接时会明确失败。页面里的问数数据源是另一套连接，支持这三种数据库。
- `auth.enabled` 为 true 时使用 HttpOnly 会话 cookie。注册、登录和退出走 `/api/account/register`、`/api/account/login`、`/api/account/logout`。设为 false 时不校验会话，页面里的 ada 仍是本机单用户。
- `app.seed_demo` 是本项目增加的配置。它只在空库时写入示例对话，关闭后不影响已有数据。

配置目录固定在 `configs/`。启动参数支持 `--base-dir`、`--config-dir`、`--data-dir` 和 `--profile`。

## 构建与安装

Windows 入口是 `scripts/build-platforms.cmd`，它调用 `scripts/build-platforms.ps1` 构建前端和三个平台的二进制。Linux 安装脚本是 `scripts/install-linux.sh`，安装到 `/opt/chatbi`，数据放在 `/var/lib/chatbi`。
