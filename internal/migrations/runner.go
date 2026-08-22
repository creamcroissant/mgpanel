// 文件路径: internal/migrations/runner.go
// 模块说明: 这是 internal 模块里的 runner 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package migrations

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/pressly/goose/v3"
)

func setup() {
	goose.SetDialect("sqlite3")
	goose.SetBaseFS(SQLite)
}

// Up migrates the SQLite schema to the latest version.
func Up(db *sql.DB) error {
	setup()
	if err := goose.Up(db, "sqlite"); err != nil {
		return err
	}
	return ensureAgentHostsProvisionStatusSchema(db)
}

// Down rolls back a single migration.
func Down(db *sql.DB) error {
	setup()
	return goose.Down(db, "sqlite")
}

// Status prints migration status.
func Status(db *sql.DB) error {
	setup()
	return goose.Status(db, "sqlite")
}

func ensureAgentHostsProvisionStatusSchema(db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("nil db")
	}
	columnExists, err := hasAgentHostsProvisionStatusColumn(db)
	if err != nil {
		return err
	}
	if !columnExists {
		if _, err := db.Exec("ALTER TABLE agent_hosts ADD COLUMN provision_status INTEGER NOT NULL DEFAULT 0"); err != nil {
			return fmt.Errorf("add agent_hosts.provision_status column: %w", err)
		}
	}
	if _, err := db.Exec("CREATE INDEX IF NOT EXISTS idx_agent_hosts_provision_status ON agent_hosts(provision_status)"); err != nil {
		return fmt.Errorf("create agent_hosts.provision_status index: %w", err)
	}
	columnExists, err = hasAgentHostsProvisionStatusColumn(db)
	if err != nil {
		return err
	}
	if !columnExists {
		return fmt.Errorf("agent_hosts.provision_status column missing after migrations")
	}
	return nil
}

func hasAgentHostsProvisionStatusColumn(db *sql.DB) (bool, error) {
	rows, err := db.Query("PRAGMA table_info(agent_hosts)")
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name string
		var dataType string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk); err != nil {
			return false, err
		}
		if strings.EqualFold(name, "provision_status") {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return false, nil
}
