-- +goose Up
-- 为清理查询（按时间范围删除）补充 created_at 索引
CREATE INDEX IF NOT EXISTS idx_agent_operation_logs_created ON agent_operation_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_traffic_report_dedups_handled_at ON traffic_report_dedups(handled_at);

-- +goose Down
DROP INDEX IF EXISTS idx_agent_operation_logs_created;
DROP INDEX IF EXISTS idx_traffic_report_dedups_handled_at;
