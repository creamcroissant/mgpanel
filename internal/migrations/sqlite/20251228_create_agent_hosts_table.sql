-- +goose Up
-- 创建 agent_hosts 表，用于记录部署 Agent 的服务器信息
CREATE TABLE IF NOT EXISTS agent_hosts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,                          -- 服务器名称（如: Tokyo-VPS-01）
    host TEXT NOT NULL UNIQUE,                   -- 服务器 IP 或域名（唯一标识）
    token TEXT NOT NULL,                         -- Agent 认证令牌
    status INTEGER DEFAULT 0,                    -- 0: 离线, 1: 在线, 2: 警告
    provision_status INTEGER NOT NULL DEFAULT 0, -- 0: 未部署, 1: 已引导注册
    cpu_total REAL DEFAULT 0,                    -- CPU 核心数
    cpu_used REAL DEFAULT 0,                     -- CPU 使用率 (%)
    mem_total INTEGER DEFAULT 0,                 -- 内存总量 (bytes)
    mem_used INTEGER DEFAULT 0,                  -- 内存使用量 (bytes)
    disk_total INTEGER DEFAULT 0,                -- 磁盘总量 (bytes)
    disk_used INTEGER DEFAULT 0,                 -- 磁盘使用量 (bytes)
    upload_total INTEGER DEFAULT 0,              -- 累计上传流量 (bytes)
    download_total INTEGER DEFAULT 0,            -- 累计下载流量 (bytes)
    last_heartbeat_at INTEGER DEFAULT 0,         -- 最后心跳时间
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

-- 创建索引
CREATE INDEX IF NOT EXISTS idx_agent_hosts_host ON agent_hosts(host);
CREATE INDEX IF NOT EXISTS idx_agent_hosts_status ON agent_hosts(status);
CREATE INDEX IF NOT EXISTS idx_agent_hosts_provision_status ON agent_hosts(provision_status);

-- 为 servers 表添加 agent_host_id 外键字段
ALTER TABLE servers ADD COLUMN agent_host_id INTEGER DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_servers_agent_host_id ON servers(agent_host_id);

-- +goose Down
DROP INDEX IF EXISTS idx_servers_agent_host_id;
ALTER TABLE servers DROP COLUMN agent_host_id;
DROP INDEX IF EXISTS idx_agent_hosts_provision_status;
DROP INDEX IF EXISTS idx_agent_hosts_status;
DROP INDEX IF EXISTS idx_agent_hosts_host;
DROP TABLE IF EXISTS agent_hosts;
