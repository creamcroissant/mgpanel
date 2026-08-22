-- +goose Up
-- Legacy order/coupon tables were fully removed from the codebase.
DROP INDEX IF EXISTS idx_orders_coupon_user;
DROP INDEX IF EXISTS idx_orders_user_id;
DROP INDEX IF EXISTS idx_orders_trade_no;
DROP TABLE IF EXISTS orders;

DROP INDEX IF EXISTS idx_coupons_code;
DROP TABLE IF EXISTS coupons;

-- +goose Down
-- Deprecated commerce tables are intentionally not recreated on rollback.
SELECT 1;
