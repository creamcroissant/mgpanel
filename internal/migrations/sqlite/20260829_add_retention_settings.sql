-- +goose Up
-- 为各膨胀表设置默认保留天数
INSERT INTO settings(key, value, category) VALUES ('access_log.retention_days', '7', 'retention') ON CONFLICT(key) DO NOTHING;
INSERT INTO settings(key, value, category) VALUES ('subscription_log.retention_days', '7', 'retention') ON CONFLICT(key) DO NOTHING;
INSERT INTO settings(key, value, category) VALUES ('login_log.retention_days', '30', 'retention') ON CONFLICT(key) DO NOTHING;
INSERT INTO settings(key, value, category) VALUES ('operation_log.retention_days', '90', 'retention') ON CONFLICT(key) DO NOTHING;
INSERT INTO settings(key, value, category) VALUES ('agent_operation_log.retention_days', '90', 'retention') ON CONFLICT(key) DO NOTHING;
INSERT INTO settings(key, value, category) VALUES ('traffic_report_dedup.retention_days', '7', 'retention') ON CONFLICT(key) DO NOTHING;

-- +goose Down
DELETE FROM settings WHERE key IN (
    'access_log.retention_days',
    'subscription_log.retention_days',
    'login_log.retention_days',
    'operation_log.retention_days',
    'agent_operation_log.retention_days',
    'traffic_report_dedup.retention_days'
);
