-- 智能体可选工具与 MCP 服务。请求头只存密文。

ALTER TABLE agent ADD COLUMN tools TEXT NOT NULL DEFAULT '[]';
ALTER TABLE agent ADD COLUMN mcp_ids TEXT NOT NULL DEFAULT '[]';
ALTER TABLE message ADD COLUMN tools TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS mcp_server (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    transport TEXT NOT NULL,
    url TEXT NOT NULL DEFAULT '',
    command TEXT NOT NULL DEFAULT '',
    args TEXT NOT NULL DEFAULT '[]',
    header_cipher TEXT NOT NULL DEFAULT '',
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_mcp_server_account ON mcp_server (account_id, updated_at, id);
