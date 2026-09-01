package sqlite

import (
	"context"
	"testing"

	"github.com/creamcroissant/xboard/internal/repository"
)

// TestDesiredArtifact_ReplaceRevisionRollback verifies that a mid-batch
// failure rolls back the whole replace: old rows must survive untouched.
func TestDesiredArtifact_ReplaceRevisionRollback(t *testing.T) {
	db := setupTestDB(t)
	repo := newDesiredArtifactRepo(db)
	ctx := context.Background()

	seed := []*repository.DesiredArtifact{
		{AgentHostID: 1, CoreType: "sing-box", DesiredRevision: 3, Filename: "a.json", SourceTag: "spec-1", Content: []byte("old-a"), ContentHash: "h-a", GeneratedAt: 1},
		{AgentHostID: 1, CoreType: "sing-box", DesiredRevision: 3, Filename: "b.json", SourceTag: "mesh", Content: []byte("old-b"), ContentHash: "h-b", GeneratedAt: 1},
	}
	if err := repo.CreateBatch(ctx, seed); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// New batch contains a nil artifact → mid-transaction failure.
	bad := []*repository.DesiredArtifact{
		{AgentHostID: 1, CoreType: "sing-box", DesiredRevision: 3, Filename: "new.json", SourceTag: "spec-1", Content: []byte("new"), ContentHash: "h-new", GeneratedAt: 2},
		nil,
	}
	if _, err := repo.ReplaceRevision(ctx, 1, "sing-box", 3, bad, "spec-1", "mesh"); err == nil {
		t.Fatalf("expected error on nil artifact")
	}

	rows, err := repo.List(ctx, repository.DesiredArtifactFilter{AgentHostID: 1})
	if err != nil {
		t.Fatalf("list after rollback: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 old rows preserved after rollback, got %d", len(rows))
	}
	for _, r := range rows {
		if r.Filename == "new.json" {
			t.Fatalf("new row leaked past rollback")
		}
	}
}

// TestDesiredArtifact_ReplaceRevisionSuccess verifies the happy path:
// scoped rows replaced atomically, other dimensions untouched.
func TestDesiredArtifact_ReplaceRevisionSuccess(t *testing.T) {
	db := setupTestDB(t)
	repo := newDesiredArtifactRepo(db)
	ctx := context.Background()

	seed := []*repository.DesiredArtifact{
		{AgentHostID: 1, CoreType: "sing-box", DesiredRevision: 4, Filename: "old.json", SourceTag: "spec-1", Content: []byte("x"), ContentHash: "hx", GeneratedAt: 1},
		{AgentHostID: 1, CoreType: "xray", DesiredRevision: 4, Filename: "other-core.json", SourceTag: "spec-9", Content: []byte("y"), ContentHash: "hy", GeneratedAt: 1},
	}
	if err := repo.CreateBatch(ctx, seed); err != nil {
		t.Fatalf("seed: %v", err)
	}

	fresh := []*repository.DesiredArtifact{
		{AgentHostID: 1, CoreType: "sing-box", DesiredRevision: 4, Filename: "fresh.json", SourceTag: "spec-1", Content: []byte("fresh"), ContentHash: "hf", GeneratedAt: 2},
	}
	n, err := repo.ReplaceRevision(ctx, 1, "sing-box", 4, fresh, "spec-1")
	if err != nil || n != 1 {
		t.Fatalf("replace: n=%d err=%v", n, err)
	}

	rows, err := repo.List(ctx, repository.DesiredArtifactFilter{AgentHostID: 1})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	names := map[string]bool{}
	for _, r := range rows {
		names[r.Filename] = true
	}
	if names["old.json"] || !names["fresh.json"] || !names["other-core.json"] {
		t.Fatalf("unexpected rows after scoped replace: %v", names)
	}
}
// TestDesiredArtifact_ReplaceRevision_SameFilenameDifferentTag verifies that
// re-rendering a file whose source_tag changed (e.g. mesh-direct.json moving
// from policy-A to policy-B) does not trigger the UNIQUE(2067) conflict:
// the old row must be deleted by filename even though its source_tag is not
// in the new scoped sourceTags set.
func TestDesiredArtifact_ReplaceRevision_SameFilenameDifferentTag(t *testing.T) {
	db := setupTestDB(t)
	repo := newDesiredArtifactRepo(db)
	ctx := context.Background()

	seed := []*repository.DesiredArtifact{
		{AgentHostID: 7, CoreType: "sing-box", DesiredRevision: 20, Filename: "mesh-direct.json", SourceTag: "policy-A", Content: []byte("old"), ContentHash: "h-old", GeneratedAt: 1},
		{AgentHostID: 7, CoreType: "sing-box", DesiredRevision: 20, Filename: "mesh-routes.json", SourceTag: "policy-A", Content: []byte("routes"), ContentHash: "h-r", GeneratedAt: 1},
	}
	if err := repo.CreateBatch(ctx, seed); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// 重渲染：同上 revision，mesh-direct.json source_tag 变为 policy-B，
	// scoped sourceTags 只含新 tag。
	fresh := []*repository.DesiredArtifact{
		{AgentHostID: 7, CoreType: "sing-box", DesiredRevision: 20, Filename: "mesh-direct.json", SourceTag: "policy-B", Content: []byte("new"), ContentHash: "h-new", GeneratedAt: 2},
		{AgentHostID: 7, CoreType: "sing-box", DesiredRevision: 20, Filename: "mesh-routes.json", SourceTag: "policy-B", Content: []byte("routes"), ContentHash: "h-r", GeneratedAt: 2},
	}
	n, err := repo.ReplaceRevision(ctx, 7, "sing-box", 20, fresh, "policy-B")
	if err != nil {
		t.Fatalf("replace with changed tag: %v", err)
	}
	if n != 2 {
		t.Fatalf("inserted = %d, want 2", n)
	}

	rows, err := repo.List(ctx, repository.DesiredArtifactFilter{AgentHostID: 7, CoreType: strPtr("sing-box"), DesiredRevision: ptrOfInt64(20)})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows after replace = %d, want 2 (no UNIQUE conflict)", len(rows))
	}
	for _, r := range rows {
		if r.SourceTag != "policy-B" {
			t.Fatalf("row %s source_tag = %s, want policy-B", r.Filename, r.SourceTag)
		}
	}
}
