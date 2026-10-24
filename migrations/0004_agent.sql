-- 智能体，以及对话归属的智能体。已有对话的 agent_id 为 0，表示普通对话。

CREATE TABLE IF NOT EXISTS agent (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    instructions TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL,
    opening TEXT NOT NULL DEFAULT '',
    suggestions TEXT NOT NULL DEFAULT '[]',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_agent_account ON agent (account_id, updated_at, id);

ALTER TABLE conversation ADD COLUMN agent_id INTEGER NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_conversation_agent ON conversation (account_id, agent_id);
