# ChatBI AI

一个按聊天首页实现的对话应用：左侧历史、空状态建议、底部输入框，以及连续的流式回答。

## 运行

```bash
cd web && npm install && npm run build && cd ..
go run ./cmd/server
```

浏览器打开 `http://127.0.0.1:8080`。

首次启动使用 SQLite `data/app.db`，并写入一组示例对话，方便直接看到「今天 / 昨天 / 最近 7 天」。生产配置 `application-pro.yaml` 关闭示例数据。

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

## 项目约定

- 后端是 Go、Gin、GORM 和 Fx。业务输入输出保持一个 Request/Search 对一个 Response/PageResponse。
- 前端是 Vue 3、JavaScript 和 Vite。
- 数据库默认且当前唯一完成验证的是 SQLite。MySQL、PostgreSQL、SQL Server 只保留配置位，连接时会明确失败。
- `auth.enabled` 必须为 false。页面里的 ada 是本机单用户，不表示已经实现登录会话。
- `app.seed_demo` 是本项目增加的配置。它只在空库时写入示例对话，关闭后不影响已有数据。

配置目录固定在 `configs/`。启动参数支持 `--base-dir`、`--config-dir`、`--data-dir` 和 `--profile`。

## 构建与安装

Windows 入口是 `scripts/build-platforms.cmd`，它调用 `scripts/build-platforms.ps1` 构建前端和三个平台的二进制。Linux 安装脚本是 `scripts/install-linux.sh`，安装到 `/opt/chatbi`，数据放在 `/var/lib/chatbi`。
