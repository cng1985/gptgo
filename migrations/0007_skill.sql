-- 当前账号安装的技能。文件放在数据目录，不进数据库。

CREATE TABLE IF NOT EXISTS skill (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    display_name TEXT NOT NULL,
    description TEXT NOT NULL,
    version TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL,
    namespace TEXT NOT NULL DEFAULT '',
    slug TEXT NOT NULL DEFAULT '',
    registry TEXT NOT NULL DEFAULT '',
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_skill_name ON skill (account_id, name);
CREATE INDEX IF NOT EXISTS idx_skill_account ON skill (account_id, updated_at, id);
