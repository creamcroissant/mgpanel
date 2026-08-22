package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

type configTemplateRepo struct {
	db *sql.DB
}

func newConfigTemplateRepo(db *sql.DB) *configTemplateRepo {
	return &configTemplateRepo{db: db}
}

func (r *configTemplateRepo) Create(ctx context.Context, tpl *repository.ConfigTemplate) error {
	now := time.Now().Unix()
	tpl.CreatedAt = now
	tpl.UpdatedAt = now

	// Default values
	if tpl.SchemaVersion == 0 {
		tpl.SchemaVersion = 1
	}
	tpl.IsValid = true

	capsJSON, err := json.Marshal(tpl.Capabilities)
	if err != nil {
		return fmt.Errorf("encode template capabilities: %w", err)
	}
	if tpl.Capabilities == nil {
		capsJSON = []byte("[]")
	}

	result, err := r.db.ExecContext(ctx, `
		INSERT INTO config_templates (
			name, type, content, description, min_version, capabilities,
			schema_version, is_valid, validation_error, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		tpl.Name, tpl.Type, tpl.Content, tpl.Description, tpl.MinVersion, string(capsJSON),
		tpl.SchemaVersion, boolToInt(tpl.IsValid), tpl.ValidationError, tpl.CreatedAt, tpl.UpdatedAt,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	tpl.ID = id
	return nil
}

func (r *configTemplateRepo) Update(ctx context.Context, tpl *repository.ConfigTemplate) error {
	tpl.UpdatedAt = time.Now().Unix()

	capsJSON, err := json.Marshal(tpl.Capabilities)
	if err != nil {
		return fmt.Errorf("encode template capabilities: %w", err)
	}
	if tpl.Capabilities == nil {
		capsJSON = []byte("[]")
	}

	result, err := r.db.ExecContext(ctx, `
		UPDATE config_templates SET
			name = ?, type = ?, content = ?, description = ?, min_version = ?,
			capabilities = ?, schema_version = ?, is_valid = ?, validation_error = ?,
			updated_at = ?
		WHERE id = ?
	`,
		tpl.Name, tpl.Type, tpl.Content, tpl.Description, tpl.MinVersion,
		string(capsJSON), tpl.SchemaVersion, boolToInt(tpl.IsValid), tpl.ValidationError,
		tpl.UpdatedAt, tpl.ID,
	)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (r *configTemplateRepo) Delete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM config_templates WHERE id = ?`, id)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (r *configTemplateRepo) FindByID(ctx context.Context, id int64) (*repository.ConfigTemplate, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, name, type, content, description, min_version, capabilities,
		       schema_version, is_valid, validation_error, created_at, updated_at
		FROM config_templates WHERE id = ?
	`, id)

	return r.scanConfigTemplate(row)
}

func (r *configTemplateRepo) ListAll(ctx context.Context) ([]*repository.ConfigTemplate, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, type, content, description, min_version, capabilities,
		       schema_version, is_valid, validation_error, created_at, updated_at
		FROM config_templates ORDER BY name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tpls []*repository.ConfigTemplate
	for rows.Next() {
		tpl, err := r.scanConfigTemplateRow(rows)
		if err != nil {
			return nil, err
		}
		tpls = append(tpls, tpl)
	}
	return tpls, rows.Err()
}

func (r *configTemplateRepo) scanConfigTemplate(row *sql.Row) (*repository.ConfigTemplate, error) {
	var tpl repository.ConfigTemplate
	var capsJSON string
	var isValidInt int

	err := row.Scan(
		&tpl.ID, &tpl.Name, &tpl.Type, &tpl.Content, &tpl.Description, &tpl.MinVersion,
		&capsJSON, &tpl.SchemaVersion, &isValidInt, &tpl.ValidationError,
		&tpl.CreatedAt, &tpl.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	tpl.IsValid = isValidInt != 0
	if capsJSON != "" {
		if err := json.Unmarshal([]byte(capsJSON), &tpl.Capabilities); err != nil {
			return nil, fmt.Errorf("decode template capabilities: %w", err)
		}
	}
	if tpl.Capabilities == nil {
		tpl.Capabilities = []string{}
	}

	return &tpl, nil
}

func (r *configTemplateRepo) scanConfigTemplateRow(rows *sql.Rows) (*repository.ConfigTemplate, error) {
	var tpl repository.ConfigTemplate
	var capsJSON string
	var isValidInt int

	err := rows.Scan(
		&tpl.ID, &tpl.Name, &tpl.Type, &tpl.Content, &tpl.Description, &tpl.MinVersion,
		&capsJSON, &tpl.SchemaVersion, &isValidInt, &tpl.ValidationError,
		&tpl.CreatedAt, &tpl.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	tpl.IsValid = isValidInt != 0
	if capsJSON != "" {
		if err := json.Unmarshal([]byte(capsJSON), &tpl.Capabilities); err != nil {
			return nil, fmt.Errorf("decode template capabilities: %w", err)
		}
	}
	if tpl.Capabilities == nil {
		tpl.Capabilities = []string{}
	}

	return &tpl, nil
}
