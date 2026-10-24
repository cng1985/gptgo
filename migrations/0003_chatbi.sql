-- 个人数据源、问数用量和账单。每个账号只能看到自己的数据源。

CREATE TABLE IF NOT EXISTS datasource (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    driver TEXT NOT NULL,
    host TEXT NOT NULL,
    port INTEGER NOT NULL,
    database_name TEXT NOT NULL,
    username TEXT NOT NULL,
    password_cipher TEXT NOT NULL DEFAULT '',
    ssl_mode TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    CONSTRAINT fk_datasource_account FOREIGN KEY (account_id) REFERENCES account (id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_datasource_account_name ON datasource (account_id, name);

CREATE INDEX IF NOT EXISTS idx_datasource_account ON datasource (account_id, updated_at, id);

CREATE TABLE IF NOT EXISTS wallet (
    account_id INTEGER PRIMARY KEY,
    balance INTEGER NOT NULL,
    updated_at DATETIME NOT NULL,
    CONSTRAINT fk_wallet_account FOREIGN KEY (account_id) REFERENCES account (id)
);

CREATE TABLE IF NOT EXISTS billing_entry (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id INTEGER NOT NULL,
    kind TEXT NOT NULL,
    amount INTEGER NOT NULL,
    balance_after INTEGER NOT NULL,
    ref_type TEXT NOT NULL DEFAULT '',
    ref_id INTEGER NOT NULL DEFAULT 0,
    note TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL,
    CONSTRAINT fk_billing_entry_account FOREIGN KEY (account_id) REFERENCES account (id)
);

CREATE INDEX IF NOT EXISTS idx_billing_account ON billing_entry (account_id, id);

CREATE TABLE IF NOT EXISTS usage_record (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id INTEGER NOT NULL,
    datasource_id INTEGER NOT NULL,
    datasource_name TEXT NOT NULL,
    question TEXT NOT NULL,
    sql_text TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL,
    row_count INTEGER NOT NULL DEFAULT 0,
    duration_ms INTEGER NOT NULL DEFAULT 0,
    prompt_tokens INTEGER NOT NULL DEFAULT 0,
    completion_tokens INTEGER NOT NULL DEFAULT 0,
    cost INTEGER NOT NULL DEFAULT 0,
    error_message TEXT NOT NULL DEFAULT '',
    day TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    CONSTRAINT fk_usage_account FOREIGN KEY (account_id) REFERENCES account (id)
);

CREATE INDEX IF NOT EXISTS idx_usage_account_day ON usage_record (account_id, day, id);
