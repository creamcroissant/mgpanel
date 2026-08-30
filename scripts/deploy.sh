#!/usr/bin/env bash
# scripts/deploy.sh — 一键构建 + 部署到生产面板与受控 agent
# 用法示例：
#   bash scripts/deploy.sh panel             # 仅构建+部署面板(二进制+前端)
#   bash scripts/deploy.sh agent             # 仅构建+部署 agent 二进制到全部节点
#   bash scripts/deploy.sh build             # 仅本地构建三产物，不部署
#   bash scripts/deploy.sh all|panel+agent   # 全量
# 环境变量：
#   SKIP_STAGING=1   跳过前端 dist 打包(scp 已有产物)
#   AGENT_HOSTS="ip[:label] [ip[:label]...]"  自定义 agent 列表(默认内置三台)
#   PANEL_HOST="root@1.2.3.4"                覆盖面板地址
#   SSH_KEY=~/.ssh/id_ecdsa                  SSH 密钥
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SSH_KEY="${SSH_KEY:-$HOME/.ssh/id_ecdsa}"
SSH_OPTS=(-i "$SSH_KEY" -o ConnectTimeout=15 -o StrictHostKeyChecking=accept-new)
DATE="$(date +%Y%m%d%H%M%S)"
STAGE="${STAGE_DIR:-/tmp/mgpanel-deploy}"
AGENT_HOSTS_DEFAULT="root@103.97.175.78 root@45.207.152.46 root@82.108.198.4"
PANEL_HOST="${PANEL_HOST:-root@154.88.64.107}"
PANEL_BIN_PATH="/opt/mgpanel/panel/mgpanel"
PANEL_DIST="/opt/mgpanel/panel/web/user-vite"

step() { echo "  [$(date +%H:%M:%S)] $*"; }

build_panel() { step "构建 panel(linux/amd64)..."; GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o "$STAGE/mgpanel-panel" ./cmd/mgpanel; grep -q "relay-routes" "$STAGE/mgpanel-panel" || echo "  ⚠️ 二进制缺少 relay-routes 符号，检查源码分支！"; }

build_agent() { step "构建 agent(linux/amd64)..."; GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o "$STAGE/mgpanel-agent" ./cmd/agent; grep -q "relay-routes" "$STAGE/mgpanel-agent" || echo "  ⚠️ agent 二进制缺少 relay-routes 符号！"; }

build_frontend() { step "构建前端 dist..."; (cd "$ROOT_DIR/web/user-vite" && npx tsc --noEmit >/dev/null 2>&1 && npx vite build >/dev/null && rm -f "$STAGE/dist.tar.gz" && tar czf "$STAGE/dist.tar.gz" -C dist .); }

deploy_panel() {
  ssh "${SSH_OPTS[@]}" "$PANEL_HOST" "cd $(dirname $PANEL_BIN_PATH) && cp $(basename $PANEL_BIN_PATH) $(basename $PANEL_BIN_PATH).bak.$DATE && systemctl stop mgpanel"
  scp "${SSH_OPTS[@]}" -q "$STAGE/mgpanel-panel" "$PANEL_HOST:$PANEL_BIN_PATH.new"
  scp "${SSH_OPTS[@]}" -q "$STAGE/dist.tar.gz" "$PANEL_HOST:/tmp/mgpanel-dist.tar.gz"
  ssh "${SSH_OPTS[@]}" "$PANEL_HOST" "mv $PANEL_BIN_PATH.new $PANEL_BIN_PATH && chmod +x $PANEL_BIN_PATH && rm -rf $PANEL_DIST/dist/* && tar xzf /tmp/mgpanel-dist.tar.gz -C $PANEL_DIST/dist && systemctl start mgpanel && sleep 5 && systemctl is-active mgpanel && curl -s -o /dev/null -w 'front:%{http_code}\n' http://127.0.0.1:18080/"
}

deploy_agent_one() {
  local host="$1"
  step "部署 agent → $host"
  ssh "${SSH_OPTS[@]}" "$host" "systemctl stop mgpanel-agent" 2>/dev/null || true
  scp "${SSH_OPTS[@]}" -q "$STAGE/mgpanel-agent" "$host:/tmp/mgpanel-agent.new"
  ssh "${SSH_OPTS[@]}" "$host" "mv /tmp/mgpanel-agent.new /opt/mgpanel/agent/agent && chmod +x /opt/mgpanel/agent/agent && systemctl start mgpanel-agent && sleep 3 && systemctl is-active mgpanel-agent" || echo "  ⚠️ $host 部署失败(网络波动可重试)"
}

deploy_agents() {
  local hosts="${AGENT_HOSTS:-$AGENT_HOSTS_DEFAULT}"
  for h in $hosts; do deploy_agent_one "$h"; done
}

# ---------- main ----------
ACTION="${1:-all}"
mkdir -p "$STAGE"
cd "$ROOT_DIR"
step "部署动作: $ACTION"
step "产物目录: $STAGE"

case "$ACTION" in
  build) build_panel; build_agent; build_frontend; step "构建完成 ✓" ;;
  panel) build_panel; build_frontend; deploy_panel ;;
  agent) build_agent; deploy_agents ;;
  all|"panel+agent") build_panel; build_frontend; build_agent; deploy_panel; deploy_agents ;;
  *) echo "用法: $0 {build|panel|agent|all}"; exit 1 ;;
esac
step "完成 ✓"