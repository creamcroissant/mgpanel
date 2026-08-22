-- +goose Up
-- Create plan_server_groups junction table
CREATE TABLE plan_server_groups (
    plan_id INTEGER NOT NULL,
    group_id INTEGER NOT NULL,
    PRIMARY KEY (plan_id, group_id),
    FOREIGN KEY (plan_id) REFERENCES plans(id) ON DELETE CASCADE,
    FOREIGN KEY (group_id) REFERENCES server_groups(id) ON DELETE CASCADE
);

CREATE INDEX idx_plan_server_groups_plan_id ON plan_server_groups(plan_id);
CREATE INDEX idx_plan_server_groups_group_id ON plan_server_groups(group_id);

-- +goose Down
DROP TABLE plan_server_groups;