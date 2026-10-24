-- 内置工具开关、HTTP 工具、MCP 服务，以及消息上的工具调用记录。

CREATE TABLE IF NOT EXISTS tool_pref (
    account_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    enabled INTEGER NOT NULL,
    PRIMARY KEY (account_id, name)
);

CREATE TABLE IF NOT EXISTS http_tool (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL,
    parameters TEXT NOT NULL DEFAULT '{}',
    method TEXT NOT NULL,
    url TEXT NOT NULL,
    headers TEXT NOT NULL DEFAULT '',
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_http_tool_name ON http_tool (account_id, name);

CREATE TABLE IF NOT EXISTS mcp_server (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    transport TEXT NOT NULL,
    command TEXT NOT NULL DEFAULT '',
    args TEXT NOT NULL DEFAULT '[]',
    env_cipher TEXT NOT NULL DEFAULT '',
    url TEXT NOT NULL DEFAULT '',
    headers_cipher TEXT NOT NULL DEFAULT '',
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_mcp_server_account ON mcp_server (account_id, updated_at, id);

ALTER TABLE message ADD COLUMN tools TEXT NOT NULL DEFAULT '';
