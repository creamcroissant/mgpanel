#!/usr/bin/env bash
# scripts/repo-sync.sh — 将 xboard 的 managed 变更同步到 xboard2p
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TARGET_REPO="${TARGET_REPO:-/opt/work/xboard2p}"
SYNCIGNORE_FILE="${SYNCIGNORE_FILE:-$ROOT_DIR/.syncignore}"

if [[ ! -d "$TARGET_REPO" ]]; then
  echo "❌ 目标仓库不存在：$TARGET_REPO"
  echo "   设置 TARGET_REPO 或创建 xboard2p 仓库"
  exit 1
fi

echo "==> 源仓库: $ROOT_DIR ($(git -C "$ROOT_DIR" rev-parse --short HEAD))"
echo "==> 目标仓库: $TARGET_REPO ($(git -C "$TARGET_REPO" rev-parse --short HEAD))"
echo ""

# 第一步：用 repo-diff.sh 生成 managed 差异报告
echo "==> 生成差异报告..."
OUT_DIR="${ROOT_DIR}/test_data/repo-diff"
MANAGED_DIFF="${OUT_DIR}/managed-tracked.diff"

bash "$ROOT_DIR/scripts/repo-diff.sh" \
  --target "$TARGET_REPO" \
  --syncignore "$SYNCIGNORE_FILE" \
  --out-dir "$OUT_DIR" 2>/dev/null || true

# 第二步：解析报告中的文件列表
# 从 managed report 中提取 shared_changed、only_source、only_target
REPORT="${OUT_DIR}/managed-tracked-diff.md"

extract_section() {
  local section="$1"
  awk -v sec="$section" '
    $0 ~ "^## " sec "$" { found=1; next }
    found && /^## / { exit }
    found && /^- `[^`]+`$/ { gsub(/^- `|`$/, "", $0); print }
  ' "$REPORT"
}

SHARED_CHANGED=$(extract_section "shared_but_changed" || true)
ONLY_SOURCE=$(extract_section "only_in_source" || true)
ONLY_TARGET=$(extract_section "only_in_target" || true)

SRC_COUNT=$(echo "$SHARED_CHANGED" | grep -c . || true)
DST_COUNT=$(echo "$ONLY_SOURCE" | grep -c . || true)
REMOVE_COUNT=$(echo "$ONLY_TARGET" | grep -c . || true)

echo "==> 待同步: ${SRC_COUNT} 文件变更 + ${DST_COUNT} 新文件"
echo "==> 待删除: ${REMOVE_COUNT} 文件（仅存在于目标仓库）"
echo ""

# 第三步：同步 shared_changed 和 only_source 文件
echo "==> 复制变更和新文件..."
TOTAL=$((SRC_COUNT + DST_COUNT))
PROGRESS=0

sync_file() {
  local rel="$1"
  local src="$ROOT_DIR/$rel"
  local dst="$TARGET_REPO/$rel"
  if [[ -f "$src" ]]; then
    mkdir -p "$(dirname "$dst")"
    cp "$src" "$dst"
  fi
}

while IFS= read -r file; do
  [[ -z "$file" ]] && continue
  sync_file "$file"
  PROGRESS=$((PROGRESS + 1))
  if [[ $((PROGRESS % 50)) -eq 0 ]]; then
    echo "  进度: ${PROGRESS}/${TOTAL}"
  fi
done <<< "$SHARED_CHANGED"
echo "  进度: ${PROGRESS}/${TOTAL} — shared_changed 完成"

while IFS= read -r file; do
  [[ -z "$file" ]] && continue
  sync_file "$file"
  PROGRESS=$((PROGRESS + 1))
  if [[ $((PROGRESS % 50)) -eq 0 ]]; then
    echo "  进度: ${PROGRESS}/${TOTAL}"
  fi
done <<< "$ONLY_SOURCE"
echo "  进度: ${PROGRESS}/${TOTAL} — all done"

# 第四步：清理 only_target 文件
if [[ "$REMOVE_COUNT" -gt 0 ]]; then
  echo ""
  echo "==> 删除仅存在于目标仓库的文件..."
  while IFS= read -r file; do
    [[ -z "$file" ]] && continue
    path="$TARGET_REPO/$file"
    if [[ -f "$path" ]]; then
      rm "$path"
      echo "  删除: $file"
      # 尝试清理空目录（不报错）
      rmdir "$(dirname "$path")" 2>/dev/null || true
    fi
  done <<< "$ONLY_TARGET"
fi

echo ""
echo "✅ 同步完成"
echo ""
echo "==> 下一步："
echo "   cd $TARGET_REPO"
echo "   git add -A"
echo "   git status"
