-- +goose Up
-- Create subscription_logs table
CREATE TABLE subscription_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    client_ip TEXT NOT NULL,
    user_agent TEXT NOT NULL,
    request_type TEXT NOT NULL DEFAULT '',
    request_url TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX idx_subscription_logs_user_id ON subscription_logs(user_id);
CREATE INDEX idx_subscription_logs_created_at ON subscription_logs(created_at);

-- +goose Down
DROP TABLE subscription_logs;
