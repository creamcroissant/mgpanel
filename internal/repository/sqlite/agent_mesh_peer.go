package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

type agentMeshPeerRepo struct {
	db *sql.DB
}

func NewAgentMeshPeerRepository(db *sql.DB) repository.AgentMeshPeerRepository {
	return &agentMeshPeerRepo{db: db}
}

func (r *agentMeshPeerRepo) Upsert(ctx context.Context, peer *repository.AgentMeshPeer) error {
	now := time.Now().Unix()
	query := `INSERT INTO agent_mesh_peers (agent_host_id, wg_private_key, wg_public_key, wg_ip, wg_listen_port, network_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(agent_host_id) DO UPDATE SET
			wg_private_key = excluded.wg_private_key,
			wg_public_key = excluded.wg_public_key,
			wg_ip = excluded.wg_ip,
			wg_listen_port = excluded.wg_listen_port,
			network_id = excluded.network_id,
			updated_at = excluded.updated_at`
	_, err := execWithRetry(ctx, r.db, query,
		peer.AgentHostID, peer.WGPrivateKey, peer.WGPublicKey, peer.WGIP,
		peer.WGListenPort, peer.NetworkID, now, now)
	return err
}

func (r *agentMeshPeerRepo) FindByAgentHostID(ctx context.Context, agentHostID int64) (*repository.AgentMeshPeer, error) {
	query := `SELECT id, agent_host_id, wg_private_key, wg_public_key, wg_ip, wg_listen_port, network_id, created_at, updated_at
		FROM agent_mesh_peers WHERE agent_host_id = ?`
	row := r.db.QueryRowContext(ctx, query, agentHostID)
	peer := &repository.AgentMeshPeer{}
	err := row.Scan(&peer.ID, &peer.AgentHostID, &peer.WGPrivateKey, &peer.WGPublicKey,
		&peer.WGIP, &peer.WGListenPort, &peer.NetworkID, &peer.CreatedAt, &peer.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return peer, nil
}

func (r *agentMeshPeerRepo) ListByNetworkID(ctx context.Context, networkID string) ([]*repository.AgentMeshPeer, error) {
	query := `SELECT id, agent_host_id, wg_private_key, wg_public_key, wg_ip, wg_listen_port, network_id, created_at, updated_at
		FROM agent_mesh_peers WHERE network_id = ? ORDER BY wg_ip ASC`
	rows, err := r.db.QueryContext(ctx, query, networkID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var peers []*repository.AgentMeshPeer
	for rows.Next() {
		peer := &repository.AgentMeshPeer{}
		if err := rows.Scan(&peer.ID, &peer.AgentHostID, &peer.WGPrivateKey, &peer.WGPublicKey,
			&peer.WGIP, &peer.WGListenPort, &peer.NetworkID, &peer.CreatedAt, &peer.UpdatedAt); err != nil {
			return nil, err
		}
		peers = append(peers, peer)
	}
	return peers, rows.Err()
}

func (r *agentMeshPeerRepo) Delete(ctx context.Context, agentHostID int64) error {
	_, err := execWithRetry(ctx, r.db, "DELETE FROM agent_mesh_peers WHERE agent_host_id = ?", agentHostID)
	// idempotent: no error if not found
	return err
}
