-- +goose Up
DELETE FROM users
WHERE email = 'admin@example.com'
  AND password = 'REPLACE_WITH_HASH';

-- +goose Down
INSERT INTO users(id, email, password, status)
SELECT 1, 'admin@example.com', 'REPLACE_WITH_HASH', 1
WHERE NOT EXISTS (
    SELECT 1 FROM users WHERE email = 'admin@example.com'
);
