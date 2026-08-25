-- +goose Up
-- 路由策略支持绑定到具体入站 spec（入站规则优先于全局规则分流）。
-- spec_id 为 NULL 表示全局策略（对所有入站生效）；
-- 非 NULL 表示仅对绑定的 inbound_spec 流量生效，渲染时排在全局规则之前。
ALTER TABLE routing_policies ADD COLUMN spec_id INTEGER REFERENCES inbound_specs(id) ON DELETE CASCADE;

-- +goose Down
SQLite 3.35+ 支持 DROP COLUMN；老版本需重建表，此处仅保留回滚语义说明。
