// 文件路径: internal/repository/sqlite/server.go
// 模块说明: 这是 internal 模块里的 server 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/creamcroissant/xboard/internal/repository"
)

type serverRepo struct {
	db *sql.DB
}

func (r *serverRepo) FindAllVisible(ctx context.Context) ([]*repository.Server, error) {
	const query = `SELECT id, code, group_id, route_id, parent_id, agent_host_id, tags, name, rate, host, port, server_port,
		cipher, obfs, obfs_settings, "show", sort, status, type, settings, last_heartbeat_at, created_at, updated_at
        FROM servers
        WHERE "show" = 1
        ORDER BY sort DESC, id ASC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var servers []*repository.Server
	for rows.Next() {
		server, err := scanServer(rows)
		if err != nil {
			return nil, err
		}
		servers = append(servers, server)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return servers, nil
}

func (r *serverRepo) ListAll(ctx context.Context) ([]*repository.Server, error) {
	const query = `SELECT id, code, group_id, route_id, parent_id, agent_host_id, tags, name, rate, host, port, server_port,
		cipher, obfs, obfs_settings, "show", sort, status, type, settings, last_heartbeat_at, created_at, updated_at
        FROM servers
        ORDER BY sort DESC, id ASC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var servers []*repository.Server
	for rows.Next() {
		server, err := scanServer(rows)
		if err != nil {
			return nil, err
		}
		servers = append(servers, server)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return servers, nil
}

func (r *serverRepo) FindByID(ctx context.Context, id int64) (*repository.Server, error) {
	const query = `SELECT id, code, group_id, route_id, parent_id, agent_host_id, tags, name, rate, host, port, server_port,
		cipher, obfs, obfs_settings, "show", sort, status, type, settings, last_heartbeat_at, created_at, updated_at
        FROM servers
        WHERE id = ?`
	row := r.db.QueryRowContext(ctx, query, id)
	server, err := scanServer(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return server, nil
}

func (r *serverRepo) FindByGroupIDs(ctx context.Context, groupIDs []int64) ([]*repository.Server, error) {
	if len(groupIDs) == 0 {
		return []*repository.Server{}, nil
	}
	placeholders := make([]string, len(groupIDs))
	args := make([]any, len(groupIDs))
	for i, id := range groupIDs {
		placeholders[i] = "?"
		args[i] = id
	}
	query := `SELECT id, code, group_id, route_id, parent_id, agent_host_id, tags, name, rate, host, port, server_port,
		cipher, obfs, obfs_settings, "show", sort, status, type, settings, last_heartbeat_at, created_at, updated_at
        FROM servers
        WHERE group_id IN (` + strings.Join(placeholders, ",") + `) AND "show" = 1
        ORDER BY sort DESC, id ASC`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var servers []*repository.Server
	for rows.Next() {
		server, err := scanServer(rows)
		if err != nil {
			return nil, err
		}
		servers = append(servers, server)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return servers, nil
}

func (r *serverRepo) Create(ctx context.Context, server *repository.Server) error {
	const query = `INSERT INTO servers (
		code, group_id, route_id, parent_id, agent_host_id, tags, name, rate, host, port, server_port,
		cipher, obfs, obfs_settings, "show", sort, status, type, settings, last_heartbeat_at, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	now := time.Now().Unix()
	server.CreatedAt = now
	server.UpdatedAt = now

	res, err := r.db.ExecContext(ctx, query,
		server.Code,
		server.GroupID,
		server.RouteID,
		server.ParentID,
		server.AgentHostID,
		server.Tags,
		server.Name,
		server.Rate,
		server.Host,
		server.Port,
		server.ServerPort,
		server.Cipher,
		server.Obfs,
		server.ObfsSettings,
		server.Show,
		server.Sort,
		server.Status,
		server.Type,
		server.Settings,
		server.LastHeartbeatAt,
		server.CreatedAt,
		server.UpdatedAt,
	)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	server.ID = id
	return nil
}

func (r *serverRepo) Update(ctx context.Context, server *repository.Server) error {
	const query = `UPDATE servers SET
		code=?, group_id=?, route_id=?, parent_id=?, agent_host_id=?, tags=?, name=?, rate=?, host=?, port=?, server_port=?,
		cipher=?, obfs=?, obfs_settings=?, "show"=?, sort=?, status=?, type=?, settings=?, last_heartbeat_at=?, updated_at=?
		WHERE id = ?`

	server.UpdatedAt = time.Now().Unix()

	result, err := r.db.ExecContext(ctx, query,
		server.Code,
		server.GroupID,
		server.RouteID,
		server.ParentID,
		server.AgentHostID,
		server.Tags,
		server.Name,
		server.Rate,
		server.Host,
		server.Port,
		server.ServerPort,
		server.Cipher,
		server.Obfs,
		server.ObfsSettings,
		server.Show,
		server.Sort,
		server.Status,
		server.Type,
		server.Settings,
		server.LastHeartbeatAt,
		server.UpdatedAt,
		server.ID,
	)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (r *serverRepo) UpdateHeartbeat(ctx context.Context, id int64, heartbeatAt int64) error {
	const query = `UPDATE servers SET last_heartbeat_at = ?, updated_at = ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, heartbeatAt, time.Now().Unix(), id)
	return err
}

func (r *serverRepo) Delete(ctx context.Context, id int64) error {
	const query = `DELETE FROM servers WHERE id = ?`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (r *serverRepo) FindByAgentHostID(ctx context.Context, agentHostID int64) ([]*repository.Server, error) {
	const query = `SELECT id, code, group_id, route_id, parent_id, agent_host_id, tags, name, rate, host, port, server_port,
		cipher, obfs, obfs_settings, "show", sort, status, type, settings, last_heartbeat_at, created_at, updated_at
        FROM servers
        WHERE agent_host_id = ?
        ORDER BY sort DESC, id ASC`
	rows, err := r.db.QueryContext(ctx, query, agentHostID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var servers []*repository.Server
	for rows.Next() {
		server, err := scanServer(rows)
		if err != nil {
			return nil, err
		}
		servers = append(servers, server)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return servers, nil
}

func (r *serverRepo) Count(ctx context.Context) (int64, error) {
	var count int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM servers").Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

type serverGroupRepo struct {
	db *sql.DB
}

func (r *serverGroupRepo) List(ctx context.Context) ([]*repository.ServerGroup, error) {
	const query = `SELECT id, name, type, sort, created_at, updated_at FROM server_groups ORDER BY sort DESC, id ASC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []*repository.ServerGroup
	for rows.Next() {
		group := new(repository.ServerGroup)
		if err := rows.Scan(&group.ID, &group.Name, &group.Type, &group.Sort, &group.CreatedAt, &group.UpdatedAt); err != nil {
			return nil, err
		}
		groups = append(groups, group)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return groups, nil
}

type serverRouteRepo struct {
	db *sql.DB
}

func (r *serverRouteRepo) List(ctx context.Context) ([]*repository.ServerRoute, error) {
	const query = `SELECT id, remarks, match, action, action_value, created_at, updated_at FROM server_routes ORDER BY id ASC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var routes []*repository.ServerRoute
	for rows.Next() {
		route := new(repository.ServerRoute)
		if err := rows.Scan(&route.ID, &route.Remarks, &route.Match, &route.Action, &route.ActionValue, &route.CreatedAt, &route.UpdatedAt); err != nil {
			return nil, err
		}
		routes = append(routes, route)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return routes, nil
}

func (r *serverRouteRepo) FindByIDs(ctx context.Context, ids []int64) ([]*repository.ServerRoute, error) {
	if len(ids) == 0 {
		return []*repository.ServerRoute{}, nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	query := `SELECT id, remarks, match, action, action_value, created_at, updated_at
		       FROM server_routes
		       WHERE id IN (` + strings.Join(placeholders, ",") + `)
		       ORDER BY id ASC`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var routes []*repository.ServerRoute
	for rows.Next() {
		route := new(repository.ServerRoute)
		if err := rows.Scan(&route.ID, &route.Remarks, &route.Match, &route.Action, &route.ActionValue, &route.CreatedAt, &route.UpdatedAt); err != nil {
			return nil, err
		}
		routes = append(routes, route)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return routes, nil
}

type serverScanner interface {
	Scan(dest ...any) error
}

func scanServer(scanner serverScanner) (*repository.Server, error) {
	var (
		server       repository.Server
		code         sql.NullString
		tags         sql.NullString
		obfsSettings sql.NullString
		settings     sql.NullString
		cipher       sql.NullString
		obfs         sql.NullString
		agentHostID  sql.NullInt64
	)

	if err := scanner.Scan(
		&server.ID,
		&code,
		&server.GroupID,
		&server.RouteID,
		&server.ParentID,
		&agentHostID,
		&tags,
		&server.Name,
		&server.Rate,
		&server.Host,
		&server.Port,
		&server.ServerPort,
		&cipher,
		&obfs,
		&obfsSettings,
		&server.Show,
		&server.Sort,
		&server.Status,
		&server.Type,
		&settings,
		&server.LastHeartbeatAt,
		&server.CreatedAt,
		&server.UpdatedAt,
	); err != nil {
		return nil, err
	}

	if agentHostID.Valid {
		server.AgentHostID = agentHostID.Int64
	}
	if cipher.Valid {
		server.Cipher = cipher.String
	}
	if obfs.Valid {
		server.Obfs = obfs.String
	}
	if code.Valid {
		server.Code = code.String
	}
	if tags.Valid {
		server.Tags = json.RawMessage(tags.String)
	}
	if obfsSettings.Valid {
		server.ObfsSettings = json.RawMessage(obfsSettings.String)
	}
	if settings.Valid {
		server.Settings = json.RawMessage(settings.String)
	}

	return &server, nil
}

func (r *serverRepo) FindByIdentifier(ctx context.Context, identifier string, nodeType string) (*repository.Server, error) {
	trimmed := strings.TrimSpace(identifier)
	if trimmed == "" {
		return nil, repository.ErrNotFound
	}
	const baseQuery = `SELECT id, code, group_id, route_id, parent_id, agent_host_id, tags, name, rate, host, port, server_port,
		cipher, obfs, obfs_settings, "show", sort, status, type, settings, last_heartbeat_at, created_at, updated_at FROM servers`
	conditions := make([]string, 0, 3)
	args := make([]any, 0, 4)
	if id, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
		conditions = append(conditions, "(code = ? OR id = ?)")
		args = append(args, trimmed, id)
		if nodeType != "" {
			conditions = append(conditions, "LOWER(type) = LOWER(?)")
			args = append(args, nodeType)
		}
		query := baseQuery + " WHERE " + strings.Join(conditions, " AND ") + " ORDER BY CASE WHEN code = ? THEN 0 ELSE 1 END LIMIT 1"
		args = append(args, trimmed)
		row := r.db.QueryRowContext(ctx, query, args...)
		server, err := scanServer(row)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, repository.ErrNotFound
			}
			return nil, err
		}
		return server, nil
	}
	conditions = append(conditions, "code = ?")
	args = append(args, trimmed)
	if nodeType != "" {
		conditions = append(conditions, "LOWER(type) = LOWER(?)")
		args = append(args, nodeType)
	}
	query := baseQuery + " WHERE " + strings.Join(conditions, " AND ") + " LIMIT 1"
	row := r.db.QueryRowContext(ctx, query, args...)
	server, err := scanServer(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return server, nil
}
