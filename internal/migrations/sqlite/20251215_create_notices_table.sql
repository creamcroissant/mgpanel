-- +goose Up
CREATE TABLE IF NOT EXISTS notices (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    sort INTEGER,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    img_url TEXT,
    tags TEXT,
    show INTEGER NOT NULL DEFAULT 0,
    popup INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL DEFAULT (strftime('%s','now')),
    updated_at INTEGER NOT NULL DEFAULT (strftime('%s','now'))
);

CREATE INDEX IF NOT EXISTS idx_notices_sort ON notices(sort);
CREATE INDEX IF NOT EXISTS idx_notices_show ON notices(show);

-- +goose Down
DROP TABLE IF EXISTS notices;
