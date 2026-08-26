#!/usr/bin/env bash
# test-sandbox.sh —— 在 bubblewrap 命名空间沙箱内执行命令，防止测试触碰宿主环境。
#
# 背景(2026-08-26)：曾在宿主机直接跑 go test ./internal/...，internal/agent/access
# 测试走到 clash_api 注入路径且 configDir 为空默认落到 /etc/sing-box/conf，
# 导致宿主 sing-box 的 experimental.json 被改写并触发 reload（"劫持"）。
#
# 本脚本把宿主文件系统只读挂载（/etc、/run、/var 全不可写），仅 /tmp 为可写
# tmpfs，因此任何测试都无法写 sing-box/xray 配置或操作 systemd 服务；网络保持
# 共享（不隔离回环/对外探测能力），避免破坏需要联网的测试。
#
# 用法：
#   scripts/test-sandbox.sh <命令...>
#   scripts/test-sandbox.sh go test ./internal/... -count=1 -short -cover
set -euo pipefail

SANDBOX_TMP=$(mktemp -d /tmp/mgpanel-sandbox-XXXXXX)
trap 'rm -rf "$SANDBOX_TMP" 2>/dev/null || true' EXIT

GOCACHE="${GOCACHE:-/root/.cache/go-build}"
GOMODCACHE="${GOMODCACHE:-/root/go/pkg/mod}"

exec bwrap \
  --unshare-uts \
  --unshare-ipc \
  --unshare-pid \
  --ro-bind / / \
  --bind "$SANDBOX_TMP" /tmp \
  --bind "$GOCACHE" /root/.cache/go-build \
  --bind "$GOMODCACHE" /root/go/pkg/mod \
  --dev /dev \
  --setenv GOCACHE /root/.cache/go-build \
  --setenv GOPATH /root/go \
  --setenv GOMODCACHE /root/go/pkg/mod \
  --setenv HOME /root \
  --setenv GOFLAGS "" \
  --setenv GOPROXY "https://proxy.golang.org,direct" \
  --chdir /opt/work/mgpanel/mgpanel \
  -- "$@"
