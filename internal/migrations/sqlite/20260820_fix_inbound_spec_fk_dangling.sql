-- +goose Up
-- 修复 20260703 表重建导致的外键悬空引用。
-- 20260703 中 ALTER TABLE inbound_specs RENAME TO inbound_specs_old 会让 SQLite
-- 自动把引用 inbound_specs 的外键改指向 inbound_specs_old，随后 DROP 旧表，
-- 导致 spec_host_bindings / inbound_spec_revisions 的外键指向不存在的表，
-- 插入绑定时报错（bind 500）。
PRAGMA foreign_keys=off;

-- 1. 重建 inbound_spec_revisions，外键指向 inbound_specs
CREATE TABLE IF NOT EXISTS inbound_spec_revisions_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    spec_id INTEGER NOT NULL,
    revision INTEGER NOT NULL,
    snapshot TEXT NOT NULL,                     -- JSON snapshot
    change_note TEXT NOT NULL DEFAULT '',
    operator_id INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    FOREIGN KEY (spec_id) REFERENCES inbound_specs(id) ON DELETE CASCADE
);
INSERT INTO inbound_spec_revisions_new (id, spec_id, revision, snapshot, change_note, operator_id, created_at)
    SELECT id, spec_id, revision, snapshot, change_note, operator_id, created_at FROM inbound_spec_revisions;
DROP TABLE IF EXISTS inbound_spec_revisions;
ALTER TABLE inbound_spec_revisions_new RENAME TO inbound_spec_revisions;

CREATE INDEX IF NOT EXISTS idx_inbound_spec_revisions_spec_id ON inbound_spec_revisions(spec_id);
CREATE INDEX IF NOT EXISTS idx_inbound_spec_revisions_created_at ON inbound_spec_revisions(created_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_inbound_spec_revisions_unique ON inbound_spec_revisions(spec_id, revision);

-- 2. 重建 spec_host_bindings，外键指向 inbound_specs
CREATE TABLE IF NOT EXISTS spec_host_bindings_new (
    spec_id INTEGER NOT NULL,
    agent_host_id INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    PRIMARY KEY (spec_id, agent_host_id),
    FOREIGN KEY (spec_id) REFERENCES inbound_specs(id) ON DELETE CASCADE,
    FOREIGN KEY (agent_host_id) REFERENCES agent_hosts(id) ON DELETE CASCADE
);
INSERT INTO spec_host_bindings_new (spec_id, agent_host_id, created_at)
    SELECT spec_id, agent_host_id, created_at FROM spec_host_bindings;
DROP TABLE IF EXISTS spec_host_bindings;
ALTER TABLE spec_host_bindings_new RENAME TO spec_host_bindings;

PRAGMA foreign_keys=on;

-- +goose Down
-- 回退：重建为引用 inbound_specs（与 Up 结果相同，此处仅恢复表名结构占位）
PRAGMA foreign_keys=off;

CREATE TABLE IF NOT EXISTS inbound_spec_revisions_old (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    spec_id INTEGER NOT NULL,
    revision INTEGER NOT NULL,
    snapshot TEXT NOT NULL,
    change_note TEXT NOT NULL DEFAULT '',
    operator_id INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    FOREIGN KEY (spec_id) REFERENCES inbound_specs(id) ON DELETE CASCADE
);
INSERT INTO inbound_spec_revisions_old (id, spec_id, revision, snapshot, change_note, operator_id, created_at)
    SELECT id, spec_id, revision, snapshot, change_note, operator_id, created_at FROM inbound_spec_revisions;
DROP TABLE IF EXISTS inbound_spec_revisions;
ALTER TABLE inbound_spec_revisions_old RENAME TO inbound_spec_revisions;

CREATE INDEX IF NOT EXISTS idx_inbound_spec_revisions_spec_id ON inbound_spec_revisions(spec_id);
CREATE INDEX IF NOT EXISTS idx_inbound_spec_revisions_created_at ON inbound_spec_revisions(created_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_inbound_spec_revisions_unique ON inbound_spec_revisions(spec_id, revision);

CREATE TABLE IF NOT EXISTS spec_host_bindings_old (
    spec_id INTEGER NOT NULL,
    agent_host_id INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    PRIMARY KEY (spec_id, agent_host_id),
    FOREIGN KEY (spec_id) REFERENCES inbound_specs(id) ON DELETE CASCADE,
    FOREIGN KEY (agent_host_id) REFERENCES agent_hosts(id) ON DELETE CASCADE
);
INSERT INTO spec_host_bindings_old (spec_id, agent_host_id, created_at)
    SELECT spec_id, agent_host_id, created_at FROM spec_host_bindings;
DROP TABLE IF EXISTS spec_host_bindings;
ALTER TABLE spec_host_bindings_old RENAME TO spec_host_bindings;

PRAGMA foreign_keys=on;
