-- 账号登录字段与会话。已有本地用户的 username 为空，不参与唯一约束。

ALTER TABLE account ADD COLUMN username TEXT NOT NULL DEFAULT '';

ALTER TABLE account ADD COLUMN password_hash TEXT NOT NULL DEFAULT '';

CREATE UNIQUE INDEX IF NOT EXISTS idx_account_username ON account (username) WHERE username <> '';

ALTER TABLE conversation ADD COLUMN account_id INTEGER NOT NULL DEFAULT 1;

CREATE INDEX IF NOT EXISTS idx_conversation_account ON conversation (account_id, updated_at, id);

CREATE TABLE IF NOT EXISTS login_session (
    token TEXT PRIMARY KEY,
    account_id INTEGER NOT NULL,
    expires_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL,
    CONSTRAINT fk_login_session_account FOREIGN KEY (account_id) REFERENCES account (id)
);

CREATE INDEX IF NOT EXISTS idx_login_session_account ON login_session (account_id);
