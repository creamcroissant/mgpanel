package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

// egressListenPortBase 出口集分发隧道监听端口基址（I5：32000 + seq）。
const egressListenPortBase = 32000

// egressPairSeqMax seq 上限：seq/64 < 256 且 (seq%64)*4 + 3 <= 255，
// 保证隧道网段始终落在 10.220.<0..63>.<...>/30 内（I4）。
const egressPairSeqMax = 4095

// egressPairAllocAttempts seq 分配在 UNIQUE(seq)/UNIQUE(listen_port) 冲突时的重试次数。
const egressPairAllocAttempts = 8

type egressDispatchPairRepo struct {
	db *sql.DB
}

func newEgressDispatchPairRepo(db *sql.DB) *egressDispatchPairRepo {
	return &egressDispatchPairRepo{db: db}
}

// EgressPairLocalNet 由 seq 推导隧道网段（I4，供服务侧与测试复用）。
func EgressPairLocalNet(seq int64) string {
	return fmt.Sprintf("10.220.%d.%d/30", seq/64, (seq%64)*4)
}

// EgressPairListenPort 由 seq 推导隧道监听端口（I5）。
func EgressPairListenPort(seq int64) int { return egressListenPortBase + int(seq) }

// EnsurePair 幂等地返回 (entry, member) 的配对；不存在时分配最小空闲 seq（单事务内
// 「读取已用 seq → 取最小空闲 → 插入」），UNIQUE 冲突时重试。
func (r *egressDispatchPairRepo) EnsurePair(ctx context.Context, entryAgentID, memberAgentID int64) (*repository.EgressDispatchPair, error) {
	if entryAgentID <= 0 || memberAgentID <= 0 {
		return nil, fmt.Errorf("egress pair requires positive agent ids")
	}
	if entryAgentID == memberAgentID {
		return nil, fmt.Errorf("egress pair cannot pair agent %d with itself", entryAgentID)
	}
	if existing, err := r.findByKey(ctx, entryAgentID, memberAgentID); err != nil {
		return nil, err
	} else if existing != nil {
		return existing, nil
	}

	var lastErr error
	for attempt := 0; attempt < egressPairAllocAttempts; attempt++ {
		pair, err := r.insertWithSmallestFreeSeq(ctx, entryAgentID, memberAgentID)
		if err == nil {
			return pair, nil
		}
		if !isUniqueConstraintErr(err) {
			return nil, err
		}
		lastErr = err
		// 另一写者刚抢走 seq：重读已用集合后重试。
		if existing, ferr := r.findByKey(ctx, entryAgentID, memberAgentID); ferr == nil && existing != nil {
			return existing, nil
		}
	}
	return nil, fmt.Errorf("allocate egress pair (%d->%d): %w", entryAgentID, memberAgentID, lastErr)
}

func (r *egressDispatchPairRepo) insertWithSmallestFreeSeq(ctx context.Context, entryAgentID, memberAgentID int64) (*repository.EgressDispatchPair, error) {
	var created *repository.EgressDispatchPair
	err := txRunner(ctx, r.db, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, `
			SELECT id, entry_agent_id, member_agent_id, seq, listen_port, local_net, enabled, created_at, updated_at
			FROM egress_dispatch_pairs WHERE entry_agent_id = ? AND member_agent_id = ?
		`, entryAgentID, memberAgentID)
		existing, err := scanEgressDispatchPairRow(row)
		if err != nil && err != repository.ErrNotFound {
			return err
		}
		if existing != nil {
			created = existing
			return nil
		}

		rows, err := tx.QueryContext(ctx, `SELECT seq FROM egress_dispatch_pairs`)
		if err != nil {
			return fmt.Errorf("list used seq: %w", err)
		}
		used := make(map[int64]struct{})
		for rows.Next() {
			var seq int64
			if err := rows.Scan(&seq); err != nil {
				rows.Close()
				return fmt.Errorf("scan used seq: %w", err)
			}
			used[seq] = struct{}{}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return fmt.Errorf("iterate used seq: %w", err)
		}
		rows.Close()

		seq := int64(-1)
		for candidate := int64(0); candidate <= egressPairSeqMax; candidate++ {
			if _, taken := used[candidate]; !taken {
				seq = candidate
				break
			}
		}
		if seq < 0 {
			return fmt.Errorf("egress pair seq space exhausted (max %d)", egressPairSeqMax)
		}

		now := time.Now().Unix()
		localNet := EgressPairLocalNet(seq)
		listenPort := EgressPairListenPort(seq)
		res, err := tx.ExecContext(ctx, `
			INSERT INTO egress_dispatch_pairs (entry_agent_id, member_agent_id, seq, listen_port, local_net, enabled, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, 1, ?, ?)
		`, entryAgentID, memberAgentID, seq, listenPort, localNet, now, now)
		if err != nil {
			return err
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		created = &repository.EgressDispatchPair{
			ID:            id,
			EntryAgentID:  entryAgentID,
			MemberAgentID: memberAgentID,
			Seq:           seq,
			ListenPort:    listenPort,
			LocalNet:      localNet,
			Enabled:       true,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (r *egressDispatchPairRepo) findByKey(ctx context.Context, entryAgentID, memberAgentID int64) (*repository.EgressDispatchPair, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, entry_agent_id, member_agent_id, seq, listen_port, local_net, enabled, created_at, updated_at
		FROM egress_dispatch_pairs WHERE entry_agent_id = ? AND member_agent_id = ?
	`, entryAgentID, memberAgentID)
	pair, err := scanEgressDispatchPairRow(row)
	if err == repository.ErrNotFound {
		return nil, nil
	}
	return pair, err
}

func (r *egressDispatchPairRepo) ListByHost(ctx context.Context, agentHostID int64) ([]*repository.EgressDispatchPair, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, entry_agent_id, member_agent_id, seq, listen_port, local_net, enabled, created_at, updated_at
		FROM egress_dispatch_pairs WHERE entry_agent_id = ? OR member_agent_id = ? ORDER BY seq ASC
	`, agentHostID, agentHostID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEgressDispatchPairs(rows)
}

func (r *egressDispatchPairRepo) ListAll(ctx context.Context) ([]*repository.EgressDispatchPair, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, entry_agent_id, member_agent_id, seq, listen_port, local_net, enabled, created_at, updated_at
		FROM egress_dispatch_pairs ORDER BY seq ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEgressDispatchPairs(rows)
}

// DeleteUnused 删除不在 alive 集合中的配对（幂等全量对账，同 relay 语义），返回删除行数。
func (r *egressDispatchPairRepo) DeleteUnused(ctx context.Context, alive []repository.EgressDispatchPairKey) (int64, error) {
	keep := make(map[repository.EgressDispatchPairKey]struct{}, len(alive))
	for _, key := range alive {
		keep[key] = struct{}{}
	}

	var deleted int64
	err := txRunner(ctx, r.db, func(tx *sql.Tx) error {
		deleted = 0
		rows, err := tx.QueryContext(ctx, `SELECT id, entry_agent_id, member_agent_id FROM egress_dispatch_pairs`)
		if err != nil {
			return err
		}
		var staleIDs []int64
		for rows.Next() {
			var id, entryAgentID, memberAgentID int64
			if err := rows.Scan(&id, &entryAgentID, &memberAgentID); err != nil {
				rows.Close()
				return err
			}
			if _, ok := keep[repository.EgressDispatchPairKey{EntryAgentID: entryAgentID, MemberAgentID: memberAgentID}]; !ok {
				staleIDs = append(staleIDs, id)
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()

		const chunk = 200
		for start := 0; start < len(staleIDs); start += chunk {
			end := start + chunk
			if end > len(staleIDs) {
				end = len(staleIDs)
			}
			batch := staleIDs[start:end]
			placeholders := make([]string, len(batch))
			args := make([]any, len(batch))
			for i, id := range batch {
				placeholders[i] = "?"
				args[i] = id
			}
			res, err := tx.ExecContext(ctx, "DELETE FROM egress_dispatch_pairs WHERE id IN ("+strings.Join(placeholders, ",")+")", args...)
			if err != nil {
				return err
			}
			affected, err := res.RowsAffected()
			if err != nil {
				return err
			}
			deleted += affected
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return deleted, nil
}

func scanEgressDispatchPairRow(row *sql.Row) (*repository.EgressDispatchPair, error) {
	var pair repository.EgressDispatchPair
	var enabled int
	err := row.Scan(&pair.ID, &pair.EntryAgentID, &pair.MemberAgentID, &pair.Seq, &pair.ListenPort,
		&pair.LocalNet, &enabled, &pair.CreatedAt, &pair.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	pair.Enabled = enabled != 0
	return &pair, nil
}

func scanEgressDispatchPairs(rows *sql.Rows) ([]*repository.EgressDispatchPair, error) {
	pairs := make([]*repository.EgressDispatchPair, 0)
	for rows.Next() {
		pair := &repository.EgressDispatchPair{}
		var enabled int
		if err := rows.Scan(&pair.ID, &pair.EntryAgentID, &pair.MemberAgentID, &pair.Seq, &pair.ListenPort,
			&pair.LocalNet, &enabled, &pair.CreatedAt, &pair.UpdatedAt); err != nil {
			return nil, err
		}
		pair.Enabled = enabled != 0
		pairs = append(pairs, pair)
	}
	return pairs, rows.Err()
}

// isUniqueConstraintErr 判断是否 UNIQUE/主键冲突（seq 或 listen_port 被并发抢占）。
func isUniqueConstraintErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique constraint failed") || strings.Contains(msg, "constraint failed: unique")
}

var _ repository.EgressDispatchPairRepository = (*egressDispatchPairRepo)(nil)
