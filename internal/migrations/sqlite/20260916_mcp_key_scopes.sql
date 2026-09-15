-- +goose Up
-- +goose StatementBegin
-- MCP API Key 作用域：逗号分隔的权限集合。
--   read = 只读工具（系统状态/节点/用户/日志等既有工具）
--   ops  = 写操作工具（配置渲染与发布、出口集分发模式、路由策略与出口集维护）
-- 既有 Key 默认 read，行为与升级前完全一致（升级前不存在写工具）。
ALTER TABLE mcp_api_keys ADD COLUMN scopes TEXT NOT NULL DEFAULT 'read';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- SQLite 不支持 DROP COLUMN（旧版本），下行迁移留空保持兼容。
-- +goose StatementEnd
