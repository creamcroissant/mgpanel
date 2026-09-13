-- +goose Up
-- 出口集合策略词表对齐核心真实能力（sing-box fork loadbalance）：
--   新词表：round_robin | least_connections | source_hash | consistent_hash
--   旧值映射：weighted_random → round_robin、least_ping → least_connections、random → round_robin
-- 旧值核心不支持（无权重/延迟探测/纯随机），保留会造成 UI 选项空转，故统一改写。
UPDATE exit_node_sets SET strategy = 'round_robin' WHERE strategy IN ('weighted_random', 'random');
UPDATE exit_node_sets SET strategy = 'least_connections' WHERE strategy = 'least_ping';
-- 兜底：任何非预期值（含空值）收敛到默认策略，避免渲染出核心不认的策略串。
UPDATE exit_node_sets
SET strategy = 'round_robin'
WHERE strategy IS NULL OR strategy NOT IN ('round_robin', 'least_connections', 'source_hash', 'consistent_hash');

-- +goose Down
-- 回滚语义：旧词表（加权随机/最低延迟/纯随机）核心并不支持，映射后信息不可逆，
-- 此处保留说明，不做数据回写（需回滚请手工按业务意图选择策略）。
