package sqlite

import (
	"context"
	"testing"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

func art(hostID int64, core string, rev int64, name, content string) *repository.DesiredArtifact {
	return &repository.DesiredArtifact{
		AgentHostID:     hostID,
		CoreType:        core,
		DesiredRevision: rev,
		Filename:        name,
		ContentHash:     "hash-" + name,
		Content:         []byte(content),
		SourceTag:       "test",
	}
}

// TestDesiredArtifactRepo_ReplaceRevision 事务化替换：成功路径与空批次。
func TestDesiredArtifactRepo_ReplaceRevision(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	repo := newDesiredArtifactRepo(db)

	// 预置 revision 9 的两个工件
	if _, err := repo.ReplaceRevision(ctx, 1, "sing-box", 9, []*repository.DesiredArtifact{
		art(1, "sing-box", 9, "a.json", "old-a"),
		art(1, "sing-box", 9, "b.json", "old-b"),
	}); err != nil {
		t.Fatalf("seed rev9: %v", err)
	}

	// 同 revision 重放：幂等替换（先删同 revision 再插入）
	n, err := repo.ReplaceRevision(ctx, 1, "sing-box", 9, []*repository.DesiredArtifact{
		art(1, "sing-box", 9, "a.json", "retried-a"),
	})
	if err != nil || n != 1 {
		t.Fatalf("idempotent replay rev9: n=%d err=%v", n, err)
	}
	list9, _ := repo.List(ctx, repository.DesiredArtifactFilter{AgentHostID: 1, CoreType: strPtr("sing-box"), DesiredRevision: ptrOfInt64(9)})
	if len(list9) != 1 || string(list9[0].Content) != "retried-a" {
		t.Fatalf("rev9 should contain exactly the retried file, got %d", len(list9))
	}

	// 新 revision 写入不影响旧 revision（历史由 PruneOldRevisions 管理）
	n, err = repo.ReplaceRevision(ctx, 1, "sing-box", 10, []*repository.DesiredArtifact{
		art(1, "sing-box", 10, "c.json", "new-c"),
	})
	if err != nil || n != 1 {
		t.Fatalf("write rev10: n=%d err=%v", n, err)
	}
	oldList, _ := repo.List(ctx, repository.DesiredArtifactFilter{AgentHostID: 1, CoreType: strPtr("sing-box"), DesiredRevision: ptrOfInt64(9)})
	if len(oldList) != 1 {
		t.Fatalf("rev9 must be preserved by design, got %d", len(oldList))
	}

	// 空 batch 也应成功（仅删除旧 revision）
	n, err = repo.ReplaceRevision(ctx, 2, "xray", 1, nil)
	if err != nil || n != 0 {
		t.Fatalf("empty batch replace: n=%d err=%v", n, err)
	}

	// GetLatestRevision 反映新版本
	latest, err := repo.GetLatestRevision(ctx, 1, "sing-box")
	if err != nil || latest != 10 {
		t.Fatalf("latest = %d err=%v, want 10", latest, err)
	}

	// 无记录 → (0, nil)（MAX 语义，非错误）
	if latest, err := repo.GetLatestRevision(ctx, 42, "sing-box"); err != nil || latest != 0 {
		t.Fatalf("no records should yield (0,nil), got (%d,%v)", latest, err)
	}
}

func strPtr(s string) *string { return &s }

// TestShortLinkRepoCRUD 短链全生命周期 + 计数器。
func TestShortLinkRepoCRUD(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	repo := NewShortLinkRepository(db)

	link := &repository.ShortLink{Code: "abc123", UserID: 7}
	if err := repo.Create(ctx, link); err != nil {
		t.Fatalf("create: %v", err)
	}

	byCode, err := repo.FindByCode(ctx, "abc123")
	if err != nil || byCode.ID != link.ID {
		t.Fatalf("by code: %v", err)
	}
	byUser, _ := repo.FindByUserID(ctx, 7)
	if len(byUser) != 1 {
		t.Fatalf("by user: %d", len(byUser))
	}

	link.CustomParams = `{"k":"v"}`
	if err := repo.Update(ctx, link); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ := repo.FindByCode(ctx, "abc123")
	if got.CustomParams != `{"k":"v"}` {
		t.Fatalf("update not persisted: %q", got.CustomParams)
	}

	for i := 0; i < 3; i++ {
		if err := repo.IncrementAccessCount(ctx, link.ID, 1700000000+int64(i)); err != nil {
			t.Fatalf("increment: %v", err)
		}
	}
	final, _ := repo.FindByCode(ctx, "abc123")
	if final.AccessCount != 3 {
		t.Fatalf("access count = %d, want 3", final.AccessCount)
	}

	if err := repo.Delete(ctx, link.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.FindByCode(ctx, "abc123"); err != repository.ErrNotFound {
		t.Fatalf("deleted link should be ErrNotFound, got %v", err)
	}

	// DeleteByUserID 批量清理
	l2 := &repository.ShortLink{Code: "xyz789", UserID: 8}
	_ = repo.Create(ctx, l2)
	if err := repo.DeleteByUserID(ctx, 8); err != nil {
		t.Fatalf("delete by user: %v", err)
	}
	if _, err := repo.FindByCode(ctx, "xyz789"); err != repository.ErrNotFound {
		t.Fatalf("user links should be purged, got %v", err)
	}
}
