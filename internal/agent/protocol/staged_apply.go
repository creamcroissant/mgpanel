package protocol

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// StagedApplyMode identifies the transaction mode.
type StagedApplyMode string

const (
	StagedApplyModePatch    StagedApplyMode = "patch"
	StagedApplyModeSnapshot StagedApplyMode = "snapshot"
)

// StagedApplyPatchOperationType identifies patch operations.
type StagedApplyPatchOperationType string

const (
	StagedApplyPatchOperationUpsert StagedApplyPatchOperationType = "upsert"
	StagedApplyPatchOperationDelete StagedApplyPatchOperationType = "delete"
)

// StagedApplyFile is a file written by a staged transaction.
type StagedApplyFile struct {
	Filename string
	Content  []byte
}

// StagedApplyPatchOperation describes a single patch mutation.
type StagedApplyPatchOperation struct {
	Type     StagedApplyPatchOperationType
	Filename string
	Content  []byte
}

// StagedApplyRequest describes a shared patch/snapshot transaction.
type StagedApplyRequest struct {
	Mode            StagedApplyMode
	RunID           string
	CoreType        string
	SnapshotFiles   []StagedApplyFile
	PatchOperations []StagedApplyPatchOperation
}

// StagedApplyResult describes the transaction execution outcome.
type StagedApplyResult struct {
	CoreType   string
	ActiveDir  string
	StageDir   string
	BackupDir  string
	Switched   bool
	RolledBack bool
}

type normalizedStagedApplyFile struct {
	filename string
	content  []byte
}

type normalizedPatchOperation struct {
	typ      StagedApplyPatchOperationType
	filename string
	content  []byte
}

// ExecuteStagedApply executes a shared staged apply transaction for patch or snapshot mode.
func (m *Manager) ExecuteStagedApply(ctx context.Context, req StagedApplyRequest) (StagedApplyResult, error) {
	m.applyMu.Lock()
	defer m.applyMu.Unlock()

	paths, err := ResolveStagedApplyPaths(m.cfg)
	if err != nil {
		return StagedApplyResult{}, err
	}
	coreType, err := normalizeStagedApplyCoreType(req.CoreType)
	if err != nil {
		return StagedApplyResult{}, err
	}
	activeDir := paths.CurrentDir
	if coreType != "" {
		activeDir = paths.ManagedDir
	}

	result := StagedApplyResult{
		CoreType:  coreType,
		ActiveDir: activeDir,
	}

	stageDir, err := os.MkdirTemp(filepath.Dir(activeDir), ".mgpanel_apply_stage_*")
	if err != nil {
		return result, fmt.Errorf("create stage dir: %w", err)
	}
	stageDir = filepath.Clean(stageDir)
	result.StageDir = stageDir
	stageMoved := false
	defer func() {
		if !stageMoved {
			_ = os.RemoveAll(stageDir)
		}
	}()

	switch req.Mode {
	case StagedApplyModeSnapshot:
		files, err := normalizeStagedApplyFiles(req.SnapshotFiles)
		if err != nil {
			return result, err
		}
		if err := writeStagedFiles(stageDir, files); err != nil {
			return result, err
		}
	case StagedApplyModePatch:
		ops, err := normalizePatchOperations(req.PatchOperations)
		if err != nil {
			return result, err
		}
		if err := preparePatchStage(stageDir, activeDir, ops); err != nil {
			return result, err
		}
	default:
		return result, fmt.Errorf("invalid staged apply mode %q", req.Mode)
	}

	if coreType == "xray" {
		if err := buildMergedOutput(paths.LegacyDir, stageDir, paths.MergeOutputFile); err != nil {
			return result, fmt.Errorf("build merged output: %w", err)
		}
	}

	if err := m.ValidateConfigInDir(ctx, stageDir); err != nil {
		return result, fmt.Errorf("validate staged apply: %w", err)
	}

	backupDir, switched, err := switchManagedDir(activeDir, req.RunID, stageDir)
	if err != nil {
		return result, err
	}
	stageMoved = true
	result.BackupDir = backupDir
	result.Switched = switched

	reloadErr := m.ReloadServiceWithValidationDir(ctx, activeDir)
	if reloadErr == nil {
		if switched {
			_ = os.RemoveAll(backupDir)
		}
		return result, nil
	}
	if !switched {
		return result, fmt.Errorf("reload service after apply: %w", reloadErr)
	}

	if err := rollbackManagedDir(activeDir, backupDir); err != nil {
		return result, fmt.Errorf("reload service after apply: %w (rollback switch failed: %v)", reloadErr, err)
	}
	result.RolledBack = true
	if err := m.ReloadServiceWithValidationDir(ctx, activeDir); err != nil {
		return result, fmt.Errorf("reload service after apply: %w (rollback reload failed: %v)", reloadErr, err)
	}
	return result, fmt.Errorf("reload service after apply: %w (rolled back)", reloadErr)
}

func normalizeStagedApplyCoreType(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}
	normalized := normalizeCoreType(trimmed)
	if normalized == "" {
		return "", fmt.Errorf("invalid core_type")
	}
	return normalized, nil
}

func normalizeStagedApplyFiles(files []StagedApplyFile) ([]normalizedStagedApplyFile, error) {
	if len(files) == 0 {
		return nil, fmt.Errorf("snapshot apply has no files")
	}
	result := make([]normalizedStagedApplyFile, 0, len(files))
	seen := make(map[string]struct{}, len(files))
	for _, file := range files {
		filename, err := sanitizeFilename(file.Filename)
		if err != nil {
			return nil, fmt.Errorf("invalid staged file filename %q: %w", file.Filename, err)
		}
		if _, ok := seen[filename]; ok {
			return nil, fmt.Errorf("duplicate staged file filename: %s", filename)
		}
		seen[filename] = struct{}{}
		content := append([]byte(nil), file.Content...)
		if len(content) == 0 {
			return nil, fmt.Errorf("staged file %s content is empty", filename)
		}
		if !json.Valid(content) {
			return nil, fmt.Errorf("staged file %s content is not valid JSON", filename)
		}
		result = append(result, normalizedStagedApplyFile{filename: filename, content: content})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].filename < result[j].filename })
	return result, nil
}

func normalizePatchOperations(ops []StagedApplyPatchOperation) ([]normalizedPatchOperation, error) {
	if len(ops) == 0 {
		return nil, fmt.Errorf("patch apply has no operations")
	}
	result := make([]normalizedPatchOperation, 0, len(ops))
	seen := make(map[string]struct{}, len(ops))
	for _, op := range ops {
		filename, err := sanitizeFilename(op.Filename)
		if err != nil {
			return nil, fmt.Errorf("invalid patch filename %q: %w", op.Filename, err)
		}
		if _, ok := seen[filename]; ok {
			return nil, fmt.Errorf("duplicate patch filename: %s", filename)
		}
		seen[filename] = struct{}{}
		normalized := normalizedPatchOperation{typ: op.Type, filename: filename}
		switch op.Type {
		case StagedApplyPatchOperationUpsert:
			normalized.content = append([]byte(nil), op.Content...)
			if len(normalized.content) == 0 {
				return nil, fmt.Errorf("patch file %s content is empty", filename)
			}
			if !json.Valid(normalized.content) {
				return nil, fmt.Errorf("patch file %s content is not valid JSON", filename)
			}
		case StagedApplyPatchOperationDelete:
		default:
			return nil, fmt.Errorf("invalid patch operation %q", op.Type)
		}
		result = append(result, normalized)
	}
	return result, nil
}

func writeStagedFiles(stageDir string, files []normalizedStagedApplyFile) error {
	if err := os.MkdirAll(stageDir, 0o755); err != nil {
		return fmt.Errorf("create stage dir: %w", err)
	}
	for _, file := range files {
		path := filepath.Join(stageDir, file.filename)
		if err := os.WriteFile(path, file.content, 0o644); err != nil {
			return fmt.Errorf("write staged file %s: %w", file.filename, err)
		}
	}
	return nil
}

func preparePatchStage(stageDir, activeDir string, ops []normalizedPatchOperation) error {
	if err := os.MkdirAll(stageDir, 0o755); err != nil {
		return fmt.Errorf("create stage dir: %w", err)
	}
	if err := copyDirContents(activeDir, stageDir); err != nil {
		return err
	}
	for _, op := range ops {
		path := filepath.Join(stageDir, op.filename)
		switch op.typ {
		case StagedApplyPatchOperationUpsert:
			if err := os.WriteFile(path, op.content, 0o644); err != nil {
				return fmt.Errorf("write patch file %s: %w", op.filename, err)
			}
		case StagedApplyPatchOperationDelete:
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("delete patch file %s: %w", op.filename, err)
			}
		}
	}
	return nil
}

func copyDirContents(srcDir, dstDir string) error {
	info, err := os.Stat(srcDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat active dir: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("active dir is not directory")
	}
	return filepath.Walk(srcDir, func(path string, info fs.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(dstDir, rel)
		entryInfo, err := os.Stat(path)
		if err != nil {
			return err
		}
		if entryInfo.IsDir() {
			return os.MkdirAll(target, entryInfo.Mode())
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err = func() error {
			srcFile, openErr := os.Open(path)
			if openErr != nil {
				return openErr
			}
			defer srcFile.Close()
			dstFile, openErr := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, entryInfo.Mode())
			if openErr != nil {
				return openErr
			}
			_, copyErr := io.Copy(dstFile, srcFile)
			closeErr := dstFile.Close()
			if copyErr != nil {
				if closeErr != nil {
					return fmt.Errorf("write: %w, close: %v", copyErr, closeErr)
				}
				return copyErr
			}
			return closeErr
		}(); err != nil {
			return err
		}
		return nil
	})
}

func switchManagedDir(activeDir, runID, stageDir string) (backupDir string, switched bool, err error) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		runID = "unknown"
	}
	backupDir = filepath.Join(filepath.Dir(activeDir), fmt.Sprintf(".mgpanel_apply_backup_%s_%d", runID, time.Now().UnixNano()))
	if _, statErr := os.Stat(activeDir); statErr == nil {
		if err := os.Rename(activeDir, backupDir); err != nil {
			return "", false, fmt.Errorf("move current dir to backup: %w", err)
		}
		switched = true
	} else if !os.IsNotExist(statErr) {
		return "", false, fmt.Errorf("stat active dir: %w", statErr)
	}
	if err := os.Rename(stageDir, activeDir); err != nil {
		if switched {
			_ = os.Rename(backupDir, activeDir)
		}
		return "", false, fmt.Errorf("activate staged apply: %w", err)
	}
	return backupDir, switched, nil
}

func rollbackManagedDir(activeDir, backupDir string) error {
	backupDir = strings.TrimSpace(backupDir)
	if backupDir == "" {
		return fmt.Errorf("rollback backup dir is empty")
	}
	if err := os.RemoveAll(activeDir); err != nil {
		return fmt.Errorf("remove failed active dir: %w", err)
	}
	if err := os.Rename(backupDir, activeDir); err != nil {
		return fmt.Errorf("restore backup dir: %w", err)
	}
	return nil
}

func buildMergedOutput(legacyDir, stageDir, mergeOutputFile string) error {
	legacyBasePath := filepath.Join(legacyDir, mergeOutputFile)
	baseDocument := map[string]any{}
	if content, err := os.ReadFile(legacyBasePath); err == nil {
		if err := json.Unmarshal(content, &baseDocument); err != nil {
			return fmt.Errorf("parse legacy base config %s: %w", legacyBasePath, err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read legacy base config %s: %w", legacyBasePath, err)
	}

	// 管理配置的 section 列表：inbounds/outbounds 按 tag 覆盖，routing/dns 追加
	sectionOverlayTags := []string{"inbounds", "outbounds"}
	sectionOverlayAppend := []string{"routing", "dns"}

	// 收集 stageDir 中所有非 mergeOutputFile 的 JSON 文件
	entries, err := os.ReadDir(stageDir)
	if err != nil {
		return fmt.Errorf("read stage dir: %w", err)
	}
	var managedDocs []map[string]any
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".json") {
			continue
		}
		if name == mergeOutputFile {
			continue
		}
		path := filepath.Join(stageDir, name)
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read staged file %s: %w", name, err)
		}
		var doc map[string]any
		if err := json.Unmarshal(content, &doc); err != nil {
			return fmt.Errorf("parse staged file %s: %w", name, err)
		}
		managedDocs = append(managedDocs, doc)
	}

	// 按 tag 覆盖的 section（inbounds, outbounds）
	for _, section := range sectionOverlayTags {
		baseItems := extractItems(baseDocument, section)
		managedItems := collectItemsFromDocs(managedDocs, section)
		merged, err := overlayItemsByTag(baseItems, managedItems, section)
		if err != nil {
			return fmt.Errorf("merge %s: %w", section, err)
		}
		if len(merged) > 0 {
			itemsAny := make([]any, 0, len(merged))
			for _, item := range merged {
				itemsAny = append(itemsAny, item)
			}
			baseDocument[section] = itemsAny
		}
	}

	// 追加的 section（routing, dns）
	for _, section := range sectionOverlayAppend {
		baseItems := extractItems(baseDocument, section)
		managedItems := collectItemsFromDocs(managedDocs, section)
		if len(managedItems) > 0 {
			itemsAny := make([]any, 0, len(baseItems)+len(managedItems))
			for _, item := range baseItems {
				itemsAny = append(itemsAny, item)
			}
			for _, item := range managedItems {
				itemsAny = append(itemsAny, item)
			}
			baseDocument[section] = itemsAny
		}
	}

	mergedContent, err := json.MarshalIndent(baseDocument, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal merged output: %w", err)
	}
	mergedPath := filepath.Join(stageDir, mergeOutputFile)
	if err := os.WriteFile(mergedPath, mergedContent, 0o644); err != nil {
		return fmt.Errorf("write merged output %s: %w", mergedPath, err)
	}
	return nil
}

func extractItems(doc map[string]any, section string) []map[string]any {
	if doc == nil {
		return nil
	}
	raw, ok := doc[section]
	if !ok || raw == nil {
		return nil
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil
	}
	result := make([]map[string]any, 0, len(arr))
	for _, item := range arr {
		if m, ok := toMap(item); ok {
			result = append(result, m)
		}
	}
	return result
}

func collectItemsFromDocs(docs []map[string]any, section string) []map[string]any {
	var result []map[string]any
	for _, doc := range docs {
		items := extractItems(doc, section)
		result = append(result, items...)
	}
	return result
}

func overlayItemsByTag(baseItems, managedItems []map[string]any, section string) ([]map[string]any, error) {
	managedByTag := make(map[string]map[string]any, len(managedItems))
	managedNoTag := make([]map[string]any, 0)
	for _, item := range managedItems {
		tag := itemTag(item)
		if tag == "" {
			managedNoTag = append(managedNoTag, item)
			continue
		}
		if _, exists := managedByTag[tag]; exists {
			return nil, fmt.Errorf("duplicate managed %s tag: %s", section, tag)
		}
		managedByTag[tag] = item
	}
	result := make([]map[string]any, 0, len(baseItems)+len(managedItems))
	usedManaged := make(map[string]struct{}, len(managedByTag))
	for _, item := range baseItems {
		tag := itemTag(item)
		if tag != "" {
			if managed, ok := managedByTag[tag]; ok {
				result = append(result, managed)
				usedManaged[tag] = struct{}{}
				continue
			}
		}
		result = append(result, item)
	}
	remainingTags := make([]string, 0)
	for tag := range managedByTag {
		if _, used := usedManaged[tag]; used {
			continue
		}
		remainingTags = append(remainingTags, tag)
	}
	sort.Strings(remainingTags)
	for _, tag := range remainingTags {
		result = append(result, managedByTag[tag])
	}
	result = append(result, managedNoTag...)
	return result, nil
}

func itemTag(item map[string]any) string {
	if item == nil {
		return ""
	}
	tag, _ := item["tag"].(string)
	return strings.TrimSpace(tag)
}

func toMap(value any) (map[string]any, bool) {
	mapped, ok := value.(map[string]any)
	if !ok || mapped == nil {
		return nil, false
	}
	return mapped, true
}
