package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/creamcroissant/mgpanel/internal/cache"
	"github.com/creamcroissant/mgpanel/internal/repository"
)

func statRecord(userID, hostID int64, rt int, recordAt int64, u, d int64) repository.StatUserRecord {
	return repository.StatUserRecord{
		UserID: userID, AgentHostID: hostID, ServerRate: 1.0,
		RecordAt: recordAt, RecordType: rt,
		Upload: u, Download: d,
		CreatedAt: time.Now().Unix(), UpdatedAt: time.Now().Unix(),
	}
}

// TestStatUserRepo_UpsertAccumulate ON CONFLICT 累加语义。
func TestStatUserRepo_UpsertAccumulate(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	repo := &statUserRepo{db: db}

	day0 := time.Now().Truncate(24 * time.Hour).Unix()
	if err := repo.Upsert(ctx, statRecord(1, 1, 1, day0, 100, 200)); err != nil {
		t.Fatalf("upsert1: %v", err)
	}
	if err := repo.Upsert(ctx, statRecord(1, 1, 1, day0, 50, 25)); err != nil {
		t.Fatalf("upsert2: %v", err)
	}

	sum, err := repo.SumByRange(ctx, repository.StatUserSumFilter{UserID: ptrOfInt64(1), AgentHostID: ptrOfInt64(1), StartAt: day0 - 10, EndAt: day0 + 10})
	if err != nil {
		t.Fatalf("sum: %v", err)
	}
	if sum.Upload != 150 || sum.Download != 225 {
		t.Fatalf("accumulated = (%d,%d), want (150,225)", sum.Upload, sum.Download)
	}
}

// TestStatUserSumCacheKeyDistinction 不同 filter 维度必须产生不同缓存键。
func TestStatUserSumCacheKeyDistinction(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	cacheStore := cache.NewStore(cache.Options{DefaultTTL: 15 * time.Second})
	repo := &statUserRepo{db: db, cache: cacheStore}

	now := time.Now().Unix()
	_ = repo.Upsert(ctx, statRecord(1, 1, 1, now-100, 100, 200))
	_ = repo.Upsert(ctx, statRecord(2, 1, 1, now-100, 9999, 9999))

	base := repository.StatUserSumFilter{StartAt: now - 3600, EndAt: now + 60}
	withUser := base
	withUser.UserID = ptrOfInt64(1)

	r1, _ := repo.SumByRange(ctx, base)     // 全部用户
	r2, _ := repo.SumByRange(ctx, withUser) // 仅用户 1
	if r1.Upload == r2.Upload {
		t.Fatalf("different filters must not share cache entries: %d vs %d", r1.Upload, r2.Upload)
	}
	if r2.Upload != 100 {
		t.Fatalf("user-filtered sum = %d, want 100", r2.Upload)
	}

	// 相同 filter 第二次调用命中缓存（结果一致即可，无 DB 可比对）
	r2b, _ := repo.SumByRange(ctx, withUser)
	if r2b.Upload != r2.Upload || r2b.Download != r2.Download {
		t.Fatalf("cached repeat mismatch: %+v vs %+v", r2b, r2)
	}
}

// TestStatServerRepo_SumAndTop server 维度聚合 + Top 排序边界。
func TestStatServerRepo_SumAndTop(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	repo := &statServerRepo{db: db}

	now := time.Now().Unix()
	rec := func(serverID int64, u, d int64) repository.StatServerRecord {
		return repository.StatServerRecord{
			ServerID: serverID,
			RecordAt: now - 100, RecordType: 1,
			Upload: u, Download: d,
			CreatedAt: time.Now().Unix(), UpdatedAt: time.Now().Unix(),
		}
	}
	for _, r := range []repository.StatServerRecord{rec(1, 500, 250), rec(2, 100, 50), rec(1, 300, 150)} {
		if err := repo.Upsert(ctx, r); err != nil {
			t.Fatalf("upsert server record: %v", err)
		}
	}

	sum, err := repo.SumByRange(ctx, repository.StatServerSumFilter{
		ServerID: ptrOfInt64(1), RecordType: 1, StartAt: now - 3600, EndAt: now + 60,
	})
	if err != nil {
		t.Fatalf("server sum: %v", err)
	}
	if sum.Upload != 800 || sum.Download != 400 {
		t.Fatalf("server1 = (%d,%d), want (800,400)", sum.Upload, sum.Download)
	}

	top, err := repo.TopByRange(ctx, repository.StatServerTopFilter{
		RecordType: 1, StartAt: now - 3600, EndAt: now + 60, Limit: 10,
	})
	if err != nil {
		t.Fatalf("top: %v", err)
	}
	if len(top) < 2 || top[0].Upload < top[len(top)-1].Upload {
		t.Fatalf("top ordering broken: len=%d", len(top))
	}

	// 空 range
	empty, _ := repo.SumByRange(ctx, repository.StatServerSumFilter{StartAt: 1, EndAt: 2})
	if empty.Upload != 0 || empty.Download != 0 {
		t.Fatalf("empty range should zero-sum, got %+v", empty)
	}
}

func ptrOfInt64(v int64) *int64 { return &v }
