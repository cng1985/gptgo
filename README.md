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

## ChatBI

每个登录账号可以配置自己的 MySQL、PostgreSQL 或 SQL Server 数据源。问数只生成并执行单条只读查询，结果里带表格和简单图表。密码加密后保存在本机库里，接口不会回传明文。

首次打开账单时赠送 1000 额度，每次成功问数扣 2。账单页可以记一笔充值，用量页按日期和数据源汇总查询次数、估算令牌和扣费。失败的查询会留下记录，但不扣额度。

没有模型密钥时，选择「内置助手」会按表结构在本地生成 SQL。配置了 `APP_AI_API_KEY` 后，其他模型会先生成 SQL，校验不通过再回退到本地规则。

## 项目约定

- 后端是 Go、Gin、GORM 和 Fx。业务输入输出保持一个 Request/Search 对一个 Response/PageResponse。
- 前端是 Vue 3、JavaScript 和 Vite。
- 应用库默认且当前唯一完成验证的是 SQLite。配置里的 MySQL、PostgreSQL、SQL Server 只保留应用库配置位，连接时会明确失败。页面里的问数数据源是另一套连接，支持这三种数据库。
- `auth.enabled` 为 true 时使用 HttpOnly 会话 cookie。注册、登录和退出走 `/api/account/register`、`/api/account/login`、`/api/account/logout`。设为 false 时不校验会话，页面里的 ada 仍是本机单用户。
- `app.seed_demo` 是本项目增加的配置。它只在空库时写入示例对话，关闭后不影响已有数据。

配置目录固定在 `configs/`。启动参数支持 `--base-dir`、`--config-dir`、`--data-dir` 和 `--profile`。

## 构建与安装

Windows 入口是 `scripts/build-platforms.cmd`，它调用 `scripts/build-platforms.ps1` 构建前端和三个平台的二进制。Linux 安装脚本是 `scripts/install-linux.sh`，安装到 `/opt/chatbi`，数据放在 `/var/lib/chatbi`。
