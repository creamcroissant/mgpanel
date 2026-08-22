-- +goose Up
-- 创建 user_notice_reads 表，用于追踪用户公告已读状态
CREATE TABLE IF NOT EXISTS user_notice_reads (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,             -- 用户 ID
    notice_id INTEGER NOT NULL,           -- 公告 ID
    read_at INTEGER NOT NULL,             -- 阅读时间 (Unix timestamp)
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (notice_id) REFERENCES notices(id) ON DELETE CASCADE,
    UNIQUE(user_id, notice_id)            -- 确保每个用户对每个公告只有一条记录
);

-- 创建索引
CREATE INDEX IF NOT EXISTS idx_user_notice_reads_user ON user_notice_reads(user_id);
CREATE INDEX IF NOT EXISTS idx_user_notice_reads_notice ON user_notice_reads(notice_id);

-- +goose Down
DROP INDEX IF EXISTS idx_user_notice_reads_notice;
DROP INDEX IF EXISTS idx_user_notice_reads_user;
DROP TABLE IF EXISTS user_notice_reads;
