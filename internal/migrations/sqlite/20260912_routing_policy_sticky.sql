-- +goose Up
-- 路由策略粘性开关：规则命中流量在出口池内的分配方式。
-- sticky=1（默认）：源地址哈希粘性 —— 同一来源的连接/会话固定到同一个出口成员，
--   避免一条请求在多个出口机器/IP 间随机漂移（配合编译器 loadbalance source-hash）。
-- sticky=0：轮询分摊 —— 在出口池成员间依次分配。
ALTER TABLE routing_policies ADD COLUMN sticky INTEGER NOT NULL DEFAULT 1;

-- +goose Down
-- SQLite 3.35+ 支持 DROP COLUMN；老版本需重建表，此处仅保留回滚语义说明。
ALTER TABLE routing_policies DROP COLUMN sticky;
