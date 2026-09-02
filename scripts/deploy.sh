#!/usr/bin/env bash
set -euo pipefail
STAGE="${STAGE_DIR:-/tmp/mgpanel-deploy}"
DATE=$(date +%Y%m%d%H%M%S)

# ---- env defaults ----
PANEL_HOST="${PANEL_HOST:-root@154.88.64.107}"
AGENT_HOSTS_DEFAULT="root@103.97.175.78 root@45.207.152.46 root@82.108.198.4"
AGENT_HOSTS="${AGENT_HOSTS:-$AGENT_HOSTS_DEFAULT}"
SSH_KEY="${SSH_KEY:-~/.ssh/id_ecdsa}"
SSH_OPTS=(-i "$SSH_KEY" -o StrictHostKeyChecking=accept-new -o ConnectTimeout=8)

PANEL_BIN_PATH="/opt/mgpanel/panel/mgpanel"
PANEL_DIST="/opt/mgpanel/panel/web/user-vite"
PANEL_BIN_SRC="$STAGE/mgpanel-panel"

AGENT_BIN_SRC="$STAGE/mgpanel-agent"
AGENT_BIN_PATH="/opt/mgpanel/agent/agent"

build_frontend() {
  step "build frontend dist"
  (cd web/user-vite && npx tsc --noEmit && npx vite build)
  tar czf "$STAGE/dist.tar.gz" -C "$PWD/web/user-vite/dist" .
}

build_panel() {
  step "build panel binary (linux/amd64)"
  GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o "$PANEL_BIN_SRC" ./cmd/mgpanel
  chmod +x "$PANEL_BIN_SRC"
  if ! grep -q "relay-routes" "$PANEL_BIN_SRC" 2>/dev/null; then
    echo "  ⚠️ 二进制缺少 relay-routes 符号，检查源码分支！"
  fi
}

build_agent() {
  step "build agent binary (linux/amd64)"
  GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o "$AGENT_BIN_SRC" ./cmd/agent
  chmod +x "$AGENT_BIN_SRC"
  if ! grep -q "relay-routes" "$AGENT_BIN_SRC" 2>/dev/null; then
    echo "  ⚠️ 二进制缺少 relay-routes 符号，检查源码分支！"
  fi
}

step() { echo -e "\n===> $*"; }

deploy_panel() {
  step "deploy panel → $PANEL_HOST"
  ssh "${SSH_OPTS[@]}" "$PANEL_HOST" "cd $(dirname $PANEL_BIN_PATH) && cp $(basename $PANEL_BIN_PATH) $(basename $PANEL_BIN_PATH).bak.$DATE && systemctl stop mgpanel" || true
  scp "${SSH_OPTS[@]}" -q "$PANEL_BIN_SRC" "$PANEL_HOST:$PANEL_BIN_PATH.new"
  scp "${SSH_OPTS[@]}" -q "$STAGE/dist.tar.gz" "$PANEL_HOST:/tmp/mgpanel-dist.tar.gz"
  ssh "${SSH_OPTS[@]}" "$PANEL_HOST" "mv $PANEL_BIN_PATH.new $PANEL_BIN_PATH && chmod +x $PANEL_BIN_PATH && rm -rf $PANEL_DIST/dist/* && tar xzf /tmp/mgpanel-dist.tar.gz -C $PANEL_DIST/dist && systemctl start mgpanel && sleep 5 && systemctl is-active mgpanel && curl -s -o /dev/null -w 'front:%{http_code}\n' http://127.0.0.1:18080/"
  echo "  ✅ panel deploy completed"
}

deploy_agent_one() {
  local host="$1"
  step "deploy agent → $host"
  ssh "${SSH_OPTS[@]}" "$host" "systemctl stop mgpanel-agent" 2>/dev/null || true
  scp "${SSH_OPTS[@]}" -q "$AGENT_BIN_SRC" "$host:/tmp/mgpanel-agent.new"
  ssh "${SSH_OPTS[@]}" "$host" "mv /tmp/mgpanel-agent.new $AGENT_BIN_PATH && chmod +x $AGENT_BIN_PATH && systemctl start mgpanel-agent && sleep 3 && systemctl is-active mgpanel-agent" || echo "  ⚠️ $host 部署失败(网络波动可重试)"
}

deploy_agents() {
  for h in $AGENT_HOSTS; do deploy_agent_one "$h"; done
}

# ---------- main ----------
ACTION="${1:-all}"
mkdir -p "$STAGE"
cd "$(dirname "$0")/.."

case "$ACTION" in
  frontend) build_frontend ;;
  panel) build_panel; build_frontend; deploy_panel ;;
  agent) build_agent; deploy_agents ;;
  build) build_panel; build_frontend; build_agent ;;
  all|"panel+agent") build_panel; build_frontend; build_agent; deploy_panel; deploy_agents ;;
  *)
    echo "Usage: $0 panel|agent|build|all|panel+agent|frontend"
    echo "  env: SKIP_STAGING=1 AGENT_HOSTS= PANEL_HOST= SSH_KEY= STAGE_DIR="
    exit 1
    ;;
esac