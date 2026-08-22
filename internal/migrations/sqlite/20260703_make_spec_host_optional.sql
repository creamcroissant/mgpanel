-- +goose Up
-- Spec 模板化：支持全局模板 spec 并通过绑定表关联到多台主机。
-- agent_host_id 改为可空（NULL = 模板 spec），唯一索引改为全局 (core_type, tag)。

-- 1. 创建 spec-host 绑定表
CREATE TABLE IF NOT EXISTS spec_host_bindings (
    spec_id INTEGER NOT NULL,
    agent_host_id INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    PRIMARY KEY (spec_id, agent_host_id),
    FOREIGN KEY (spec_id) REFERENCES inbound_specs(id) ON DELETE CASCADE,
    FOREIGN KEY (agent_host_id) REFERENCES agent_hosts(id) ON DELETE CASCADE
);

-- 2. 重建 inbound_specs 使 agent_host_id 可空
PRAGMA foreign_keys=off;

ALTER TABLE inbound_specs RENAME TO inbound_specs_old;

CREATE TABLE inbound_specs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_host_id INTEGER DEFAULT NULL,
    core_type TEXT NOT NULL,
    tag TEXT NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 1,
    semantic_spec TEXT NOT NULL DEFAULT '{}',
    core_specific TEXT NOT NULL DEFAULT '{}',
    desired_revision INTEGER NOT NULL DEFAULT 1,
    created_by INTEGER NOT NULL DEFAULT 0,
    updated_by INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY (agent_host_id) REFERENCES agent_hosts(id) ON DELETE SET NULL
);

INSERT INTO inbound_specs (
    id, agent_host_id, core_type, tag, enabled, semantic_spec, core_specific,
    desired_revision, created_by, updated_by, created_at, updated_at
)
SELECT
    id, agent_host_id, core_type, tag, enabled, semantic_spec, core_specific,
    desired_revision, created_by, updated_by, created_at, updated_at
FROM inbound_specs_old;

DROP TABLE inbound_specs_old;

-- 全局唯一：同 core_type + tag 不能重复
DROP INDEX IF EXISTS idx_inbound_specs_unique_tag;
CREATE UNIQUE INDEX IF NOT EXISTS idx_inbound_specs_unique_tag ON inbound_specs(core_type, tag);

-- 其他索引维持不变
CREATE INDEX IF NOT EXISTS idx_inbound_specs_agent_host ON inbound_specs(agent_host_id);
CREATE INDEX IF NOT EXISTS idx_inbound_specs_core_type ON inbound_specs(core_type);

PRAGMA foreign_keys=on;

-- +goose Down
DROP TABLE IF EXISTS spec_host_bindings;

-- 回退：恢复 agent_host_id NOT NULL
PRAGMA foreign_keys=off;

ALTER TABLE inbound_specs RENAME TO inbound_specs_new;

CREATE TABLE inbound_specs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_host_id INTEGER NOT NULL,
    core_type TEXT NOT NULL,
    tag TEXT NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 1,
    semantic_spec TEXT NOT NULL DEFAULT '{}',
    core_specific TEXT NOT NULL DEFAULT '{}',
    desired_revision INTEGER NOT NULL DEFAULT 1,
    created_by INTEGER NOT NULL DEFAULT 0,
    updated_by INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY (agent_host_id) REFERENCES agent_hosts(id) ON DELETE CASCADE
);

INSERT INTO inbound_specs (
    id, agent_host_id, core_type, tag, enabled, semantic_spec, core_specific,
    desired_revision, created_by, updated_by, created_at, updated_at
)
SELECT
    id, COALESCE(agent_host_id, 0), core_type, tag, enabled, semantic_spec, core_specific,
    desired_revision, created_by, updated_by, created_at, updated_at
FROM inbound_specs_new
WHERE agent_host_id IS NOT NULL;

DROP TABLE IF EXISTS inbound_specs_new;

DROP INDEX IF EXISTS idx_inbound_specs_unique_tag;
CREATE UNIQUE INDEX IF NOT EXISTS idx_inbound_specs_unique_tag ON inbound_specs(agent_host_id, core_type, tag);
CREATE INDEX IF NOT EXISTS idx_inbound_specs_agent_host ON inbound_specs(agent_host_id);
CREATE INDEX IF NOT EXISTS idx_inbound_specs_core_type ON inbound_specs(core_type);

PRAGMA foreign_keys=on;
