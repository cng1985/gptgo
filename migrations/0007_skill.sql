-- 账号自己的技能。对话按调用名执行已保存的脚本，参数只从标准输入进入。

CREATE TABLE IF NOT EXISTS skill (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id INTEGER NOT NULL,
    title TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL,
    instructions TEXT NOT NULL DEFAULT '',
    runtime TEXT NOT NULL,
    script TEXT NOT NULL,
    parameters TEXT NOT NULL DEFAULT '{}',
    timeout_sec INTEGER NOT NULL DEFAULT 10,
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_skill_name ON skill (account_id, name);

CREATE INDEX IF NOT EXISTS idx_skill_account ON skill (account_id, updated_at, id);
