-- +goose Up
CREATE TABLE IF NOT EXISTS knowledge (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    language TEXT NOT NULL,
    category TEXT NOT NULL,
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    sort INTEGER,
    show INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL DEFAULT (strftime('%s','now')),
    updated_at INTEGER NOT NULL DEFAULT (strftime('%s','now'))
);

CREATE INDEX IF NOT EXISTS idx_knowledge_language ON knowledge(language);
CREATE INDEX IF NOT EXISTS idx_knowledge_sort ON knowledge(sort);
CREATE INDEX IF NOT EXISTS idx_knowledge_show ON knowledge(show);

-- +goose Down
DROP TABLE IF EXISTS knowledge;
