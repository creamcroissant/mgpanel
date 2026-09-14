-- +goose Up
-- +goose StatementBegin
-- 出口集内核分发（B 方案）开关：inherit = 跟随全局默认，socks = 传统 socks over mesh，l3 = 内核 mark 分发。
ALTER TABLE agent_hosts ADD COLUMN egress_dispatch TEXT NOT NULL DEFAULT 'inherit';
-- +goose StatementEnd

-- +goose StatementBegin
-- 最近一次成功拉取 /api/v1/agent/egress-routes 的时间（unix 秒）。
-- 面板以「新鲜度」作为能力门控：从未拉取或已过期 → 一律回落 socks（避免对不支持 egress 的 agent 渲染 mark 出站）。
ALTER TABLE agent_hosts ADD COLUMN egress_synced_at INTEGER NOT NULL DEFAULT 0;
-- +goose StatementEnd

-- +goose StatementBegin
-- (入口, 成员) 隧道配对：seq 由面板分配并持久化（稳定不变，避免重配抖动），
-- 决定隧道网段 10.220.<seq/64>.<(seq%64)*4>/30 与监听端口 32000+seq。
CREATE TABLE IF NOT EXISTS egress_dispatch_pairs (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    entry_agent_id   INTEGER NOT NULL,
    member_agent_id  INTEGER NOT NULL,
    seq              INTEGER NOT NULL UNIQUE,
    listen_port      INTEGER NOT NULL UNIQUE,
    local_net        TEXT    NOT NULL,
    enabled          INTEGER NOT NULL DEFAULT 1,
    created_at       INTEGER NOT NULL,
    updated_at       INTEGER NOT NULL,
    UNIQUE(entry_agent_id, member_agent_id)
);
CREATE INDEX IF NOT EXISTS idx_egress_pairs_entry  ON egress_dispatch_pairs(entry_agent_id);
CREATE INDEX IF NOT EXISTS idx_egress_pairs_member ON egress_dispatch_pairs(member_agent_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS egress_dispatch_pairs;
-- +goose StatementEnd

-- +goose StatementBegin
-- SQLite 3.35+ 支持 DROP COLUMN；老版本需重建表。
ALTER TABLE agent_hosts DROP COLUMN egress_synced_at;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE agent_hosts DROP COLUMN egress_dispatch;
-- +goose StatementEnd
