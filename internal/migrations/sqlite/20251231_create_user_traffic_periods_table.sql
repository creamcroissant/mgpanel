-- +goose Up
-- Create user_traffic_periods table for tracking user traffic quota and usage per period
CREATE TABLE IF NOT EXISTS user_traffic_periods (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    period_start INTEGER NOT NULL,
    period_end INTEGER NOT NULL,
    upload_bytes INTEGER NOT NULL DEFAULT 0,
    download_bytes INTEGER NOT NULL DEFAULT 0,
    quota_bytes INTEGER NOT NULL,
    exceeded INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL DEFAULT (strftime('%s','now')),
    updated_at INTEGER NOT NULL DEFAULT (strftime('%s','now')),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    UNIQUE(user_id, period_start)
);

-- Indexes for faster lookups
CREATE INDEX IF NOT EXISTS idx_user_traffic_periods_user_id ON user_traffic_periods(user_id);
CREATE INDEX IF NOT EXISTS idx_user_traffic_periods_period ON user_traffic_periods(period_start, period_end);

-- +goose Down
DROP INDEX IF EXISTS idx_user_traffic_periods_period;
DROP INDEX IF EXISTS idx_user_traffic_periods_user_id;
DROP TABLE IF EXISTS user_traffic_periods;
