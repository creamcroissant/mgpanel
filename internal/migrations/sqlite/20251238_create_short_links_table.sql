-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS short_links (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    code TEXT NOT NULL UNIQUE,
    user_id INTEGER NOT NULL,
    target_path TEXT NOT NULL DEFAULT '/api/v1/client/subscribe',
    custom_params TEXT,
    expires_at INTEGER,
    access_count INTEGER NOT NULL DEFAULT 0,
    last_accessed_at INTEGER,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX idx_short_links_code ON short_links(code);
CREATE INDEX idx_short_links_user_id ON short_links(user_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_short_links_user_id;
DROP INDEX IF EXISTS idx_short_links_code;
DROP TABLE IF EXISTS short_links;
-- +goose StatementEnd
