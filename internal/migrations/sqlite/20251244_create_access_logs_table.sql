-- +goose Up
-- 创建 access_logs 表
CREATE TABLE IF NOT EXISTS access_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER,
    user_email TEXT,
    agent_host_id INTEGER NOT NULL,
    source_ip TEXT,
    target_domain TEXT,
    target_ip TEXT,
    target_port INTEGER,
    protocol TEXT,
    upload INTEGER DEFAULT 0,
    download INTEGER DEFAULT 0,
    connection_start DATETIME,
    connection_end DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL,
    FOREIGN KEY (agent_host_id) REFERENCES agent_hosts(id) ON DELETE CASCADE
);

-- 创建索引
CREATE INDEX IF NOT EXISTS idx_access_logs_user_id ON access_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_access_logs_created_at ON access_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_access_logs_agent_host_id ON access_logs(agent_host_id);
CREATE INDEX IF NOT EXISTS idx_access_logs_target_domain ON access_logs(target_domain);

-- +goose Down
DROP TABLE IF NOT EXISTS access_logs;
