-- 初始表结构。SQLite 已验证；其他数据库方言未在本构建中执行。

CREATE TABLE IF NOT EXISTS account (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS conversation (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    model TEXT NOT NULL,
    thinking_mode TEXT NOT NULL,
    web_search INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_conversation_updated ON conversation (updated_at, id);

CREATE TABLE IF NOT EXISTS message (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    conversation_id INTEGER NOT NULL,
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    thinking TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    CONSTRAINT fk_message_conversation FOREIGN KEY (conversation_id) REFERENCES conversation (id)
);

CREATE INDEX IF NOT EXISTS idx_message_conversation ON message (conversation_id, id);

CREATE TABLE IF NOT EXISTS attachment (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    message_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    media_type TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    CONSTRAINT fk_attachment_message FOREIGN KEY (message_id) REFERENCES message (id)
);

CREATE INDEX IF NOT EXISTS idx_attachment_message ON attachment (message_id);
