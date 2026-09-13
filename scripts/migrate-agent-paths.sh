#!/bin/sh
# 迁移 agent 运行时路径、systemd 单元与 nft 表名：xboard → mgpanel
#
# 背景：生产 agent 长期运行 xboard 品牌二进制，其编译默认值为
#   /opt/xboard/{agent,bin}、/var/run/xboard/cores、/sys/fs/cgroup/xboard、
#   nft 表 xboard_proxy/xboard_relay*、/etc/xboard/agent/secrets.json
# 新版 agent（全量改名后构建）默认值为对应的 mgpanel 路径与表名。
# 本脚本把旧数据搬到新路径、改写 systemd 单元与 nft 表，旧目录保留以便回滚（幂等，可重复执行）。
#
# 用法（在 agent 主机上以 root 执行）：
#   sh migrate-agent-paths.sh [/path/to/new-agent-binary]
#
# 说明：会重启 agent 服务，并在改写 sing-box 等核心单元后重启对应核心（秒级中断）。
# 回滚：把 agent.bak.* 恢复为 agent，还原 *.bak.* 的 systemd 单元，把 /opt/mgpanel 数据拷回 /opt/xboard。

set -eu

OLD_ROOT=/opt/xboard
NEW_ROOT=/opt/mgpanel
OLD_ETC=/etc/xboard
NEW_ETC=/etc/mgpanel
NEW_BIN="${1:-}"
OLD_TABLES="xboard_proxy xboard_relay xboard_relay_fwd xboard_relay_nat xboard_forwarding"
UNIT_DIR=/etc/systemd/system

NFT="$(command -v nft || true)"
[ -n "$NFT" ] || NFT=/usr/sbin/nft

say() { echo "[migrate] $*"; }

# ---------- 1. 识别 agent 服务 ----------
SERVICE=""
for s in mgpanel-agent xboard-agent; do
    if systemctl cat "$s.service" >/dev/null 2>&1; then
        SERVICE="$s"
        break
    fi
done
[ -n "$SERVICE" ] || { echo "未找到 agent 服务（mgpanel-agent/xboard-agent）"; exit 1; }
say "服务: $SERVICE"

# ---------- 2. 数据迁移（旧目录为权威来源，旧目录保留）----------
copy_tree() {
    [ -d "$1" ] || return 0
    mkdir -p "$2"
    if command -v rsync >/dev/null 2>&1; then
        rsync -a "$1"/ "$2"/
    else
        cp -a "$1"/. "$2"/ 2>/dev/null || true
    fi
    say "复制 $1 → $2"
}

mkdir -p "$NEW_ROOT/agent" "$NEW_ROOT/bin" "$NEW_ROOT/deploy" "$NEW_ETC/agent"
if [ ! -f "$NEW_ROOT/agent/config.yml" ] && [ -f "$OLD_ROOT/agent/config.yml" ]; then
    cp -a "$OLD_ROOT/agent/config.yml" "$NEW_ROOT/agent/config.yml"
    say "补 config.yml → $NEW_ROOT/agent/config.yml"
fi
copy_tree "$OLD_ROOT/agent/cores" "$NEW_ROOT/agent/cores"
# mesh 目录含 WG 私钥/peer 状态，必须覆盖为当前运行版本
if [ -d "$OLD_ROOT/agent/mesh" ]; then
    [ -d "$NEW_ROOT/agent/mesh" ] && mv "$NEW_ROOT/agent/mesh" "$NEW_ROOT/agent/mesh.stale.$(date +%s)"
    copy_tree "$OLD_ROOT/agent/mesh" "$NEW_ROOT/agent/mesh"
fi
copy_tree "$OLD_ROOT/agent/logs" "$NEW_ROOT/agent/logs"
copy_tree "$OLD_ROOT/bin" "$NEW_ROOT/bin"
copy_tree "$OLD_ROOT/deploy" "$NEW_ROOT/deploy"
copy_tree "$OLD_ETC/agent" "$NEW_ETC/agent"
[ -f "$OLD_ROOT/agent/agent-update-state.json" ] && cp -an "$OLD_ROOT/agent/agent-update-state.json" "$NEW_ROOT/agent/" 2>/dev/null || true

# ---------- 3. 稳定二进制软链指向新核心目录 ----------
for b in sing-box xray; do
    src="$OLD_ROOT/bin/$b"
    dst="$NEW_ROOT/bin/$b"
    [ -e "$src" ] || continue
    if [ -L "$dst" ]; then
        real="$(readlink -f "$src" 2>/dev/null || true)"
        case "$real" in
            "$OLD_ROOT"/*) ln -sfn "$NEW_ROOT${real#$OLD_ROOT}" "$dst" ;;
        esac
    elif [ ! -e "$dst" ]; then
        cp -a "$src" "$dst"
    fi
    say "$dst -> $(readlink "$dst" 2>/dev/null || echo '(regular file)')"
done

# ---------- 4. config.yml 路径/表名改写（只匹配明确模式，避免误改随机密钥）----------
for cfg in "$NEW_ROOT/agent/config.yml" "$OLD_ROOT/agent/config.yml"; do
    [ -f "$cfg" ] || continue
    if grep -qE '/opt/xboard|/var/run/xboard|/etc/xboard|/sys/fs/cgroup/xboard|xboard_proxy|xboard_relay|xboard_forwarding' "$cfg"; then
        cp -a "$cfg" "$cfg.bak.$(date +%s)"
        sed -i -E \
            -e 's#/opt/xboard#/opt/mgpanel#g' \
            -e 's#/var/run/xboard#/var/run/mgpanel#g' \
            -e 's#/etc/xboard#/etc/mgpanel#g' \
            -e 's#/sys/fs/cgroup/xboard#/sys/fs/cgroup/mgpanel#g' \
            -e 's#xboard_proxy#mgpanel_proxy#g' \
            -e 's#xboard_relay#mgpanel_relay#g' \
            -e 's#xboard_forwarding#mgpanel_forwarding#g' \
            "$cfg"
        say "改写 $cfg"
    fi
done

# ---------- 5. 日志改名 ----------
for d in "$NEW_ROOT/agent/logs" "$OLD_ROOT/agent/logs"; do
    [ -d "$d" ] || continue
    for f in "$d"/xboard-*.log; do
        [ -e "$f" ] || continue
        b="$(basename "$f")"
        mv -n "$f" "$d/mgpanel-${b#xboard-}"
        say "日志 $b → mgpanel-${b#xboard-}"
    done
done

# ---------- 6. systemd 单元路径改写（agent 自身 + 核心单元）----------
CORE_RESTART=""
for u in "$UNIT_DIR"/*.service; do
    [ -f "$u" ] || continue
    grep -q "$OLD_ROOT" "$u" || continue
    cp -a "$u" "$u.bak.$(date +%s)"
    sed -i "s#$OLD_ROOT#$NEW_ROOT#g" "$u"
    name="$(basename "$u")"
    case "$name" in
        "$SERVICE.service") say "改写单元 $name（agent 自身）" ;;
        *)
            systemctl is-active --quiet "$name" 2>/dev/null && CORE_RESTART="$CORE_RESTART $name"
            say "改写单元 $name（核心）"
            ;;
    esac
done
systemctl daemon-reload

# 归档旧 xboard-agent 单元文件（已由 mgpanel-agent 取代）
if [ "$SERVICE" = "mgpanel-agent" ]; then
    systemctl disable xboard-agent >/dev/null 2>&1 || true
    mkdir -p "$NEW_ROOT/agent/legacy-units"
    for f in "$UNIT_DIR"/xboard-agent.service "$UNIT_DIR"/xboard-agent.service.bak.*; do
        [ -e "$f" ] || continue
        mv "$f" "$NEW_ROOT/agent/legacy-units/" && say "归档旧单元 $(basename "$f")"
    done
    systemctl daemon-reload
fi

# ---------- 7. 切换 agent ----------
say "停止 $SERVICE"
systemctl stop "$SERVICE" || true

if [ -n "$NEW_BIN" ] && [ -f "$NEW_BIN" ]; then
    [ -f "$NEW_ROOT/agent/agent" ] && cp -a "$NEW_ROOT/agent/agent" "$NEW_ROOT/agent/agent.bak.$(date +%s)"
    install -m 0755 "$NEW_BIN" "$NEW_ROOT/agent/agent"
    say "已替换 agent 二进制 → $NEW_ROOT/agent/agent"
fi

systemctl start "$SERVICE"
sleep 4
if ! systemctl is-active --quiet "$SERVICE"; then
    say "❌ $SERVICE 未启动，请检查 journalctl -u $SERVICE -n 50"
    exit 1
fi
say "$SERVICE 已启动"

# 重启被改写单元的核心（秒级中断）
for n in $CORE_RESTART; do
    say "重启核心单元 $n"
    systemctl restart "$n" || true
    i=0
    while [ "$i" -lt 12 ]; do
        systemctl is-active --quiet "$n" && break
        sleep 5
        i=$((i + 1))
    done
    say "$n: $(systemctl is-active "$n" 2>/dev/null || echo unknown)"
done

# ---------- 8. 等新表建立后清理旧 nft 表 ----------
NEW_TABLES=""
i=0
while [ "$i" -lt 12 ]; do
    sleep 5
    NEW_TABLES="$("$NFT" list tables 2>/dev/null | grep -o 'mgpanel_[a-z_]*' | sort -u | tr '\n' ' ' || true)"
    [ -n "$NEW_TABLES" ] && break
    i=$((i + 1))
done
say "mgpanel 表: ${NEW_TABLES:-（暂无，可能本节点无 nft 规则）}"

for t in $OLD_TABLES; do
    if "$NFT" list table ip "$t" >/dev/null 2>&1; then
        "$NFT" delete table ip "$t" && say "已删除旧表 $t"
    fi
done

# ---------- 9. 结果 ----------
say "服务状态: $(systemctl is-active "$SERVICE")"
for c in sing-box xray; do
    [ -d "$NEW_ROOT/agent/cores/$c" ] && say "核心 $c: $(ls "$NEW_ROOT/agent/cores/$c" | tr '\n' ' ')"
done
echo "--- 运行中核心进程 ---"
ps -eo args | grep -E "sing-box|xray" | grep -v grep | head -3 || true
echo "--- nft tables ---"
"$NFT" list tables 2>/dev/null || true
echo "--- agent 日志尾部 ---"
ls -t "$NEW_ROOT/agent/logs"/mgpanel-*.log 2>/dev/null | head -1 | xargs -r tail -3
