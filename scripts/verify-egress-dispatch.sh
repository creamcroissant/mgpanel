#!/usr/bin/env bash
# =============================================================================
# verify-egress-dispatch.sh —— 「出口集内核分发」（B 方案）生产只读核验（在入口节点上运行）
#
# 只读保证：全部命令都是 show/list/查询，不做任何 add/del/replace/写入。
# 所有命令都可用 --dry-run 只打印不执行，便于灰度前的变更审阅。
#
# 核验内容（对应 docs/plans/20260915-egress-dispatch-l3.md §3.2 契约）：
#   入口侧（I1/I2/I3/I4/I7）：`ip rule show` 里 pref 5500 + fwmark 60000+member + lookup 10000+member；
#                            `ip route show table <table>` 的 default dev xe<member>；
#                            接口存在性与 MTU=1400。
#   出口侧（I8，需 --exit-host）：nft 表 mgpanel_egress 的 forward 放行（iifname "xi*"）
#     注意 family：agent 侧 `nft add table mgpanel_egress` 不带族 → 默认族 ip（与 mgpanel_relay 一致），
#     故默认 --family ip；如实现改为 inet 可用 --family inet 覆盖。
#                            与针对本 pair 网段的 masquerade。
#   真实出口 IP：用 python3（SO_MARK）向 IP 回显服务发起 HTTP 请求，分别打印
#                「不打 mark」与「打 mark」时服务端看到的源地址；
#                --expect-exit-ip 给定时断言「打 mark」看到的地址。
#
# 用法：
#   # 本机即入口节点
#   sudo scripts/verify-egress-dispatch.sh --member 4 --exit-host root@exit.example.com \
#        --expect-exit-ip 203.0.113.1
#   # 从跳板机核验远端入口
#   ./scripts/verify-egress-dispatch.sh --host root@entry.example.com --member 4 --json
#   # 沙箱（本机 netns，复用 e2e-egress-dispatch-netns.sh --keep 的现场）
#   sudo scripts/verify-egress-dispatch.sh --host ns:mg-e --exit-host ns:mg-x \
#        --member 4 --echo-url http://203.0.113.9:8080 --expect-exit-ip 203.0.113.1
#
# 参数：
#   --host <目标>       入口节点执行位置：ssh 目标 / local（默认）/ ns:<netns>（沙箱）
#   --exit-host <目标>  出口节点（可选，给了才做 I8 检查）；取值同上
#   --member <agentID>  成员 id，可多次；省略时由入口 `ip rule show` 的 pref 5500 规则自动发现
#   --mark <十进制>     覆盖用于「真实出口 IP」对比的 mark（默认 60000+首个 member）
#   --echo-url <url>    IP 回显服务（默认 https://api.ipify.org，返回纯文本 IP）
#   --expect-exit-ip <ip>  断言「打 mark」看到的出口 IP
#   --timeout <秒>      HTTP 超时（默认 5）
#   --exit-member <id>  --exit-host 承载的成员 agentID（可多次；用于精确断言该出口的 pair）
#   --family <ip|inet>  nft 表族（默认 ip：agent 建表不带族）
#   --json              以 JSON 输出（stdout 只出 JSON，人读摘要走 stderr）
#   --dry-run           只打印将要执行的命令，不执行、不判定
#   -h, --help
#
# 退出码：0 = 全部通过；1 = 有核验项失败/缺失；2 = 用法或前置条件错误
# =============================================================================
set -uo pipefail

HOST=""
EXIT_HOST=""
ECHO_URL="https://api.ipify.org"
NFT_FAMILY="ip"
EXPECT_EXIT_IP=""
TIMEOUT=5
MTU_EXPECT=1400
JSON=0
DRY_RUN=0
MARK_OVERRIDE=""
MEMBERS=()
EXIT_MEMBERS=()

usage() {
  awk '/^# 用法：/{f=1} /^# 退出码：/{f=0} f' "$0" | sed 's/^# \{0,1\}//'
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --host) HOST="${2:-}"; shift 2 ;;
    --exit-host) EXIT_HOST="${2:-}"; shift 2 ;;
    --member)
      [[ "${2:-}" =~ ^[0-9]+$ ]] || { echo "用法错误: --member 需要十进制 agent id: ${2:-<空>}" >&2; exit 2; }
      MEMBERS+=("$2"); shift 2 ;;
    --mark)
      [[ "${2:-}" =~ ^[0-9]+$ ]] || { echo "用法错误: --mark 需要十进制数: ${2:-<空>}" >&2; exit 2; }
      MARK_OVERRIDE="$2"; shift 2 ;;
    --echo-url) ECHO_URL="${2:-}"; shift 2 ;;
    --expect-exit-ip) EXPECT_EXIT_IP="${2:-}"; shift 2 ;;
    --timeout)
      [[ "${2:-}" =~ ^[0-9]+$ ]] || { echo "用法错误: --timeout 需要整数秒: ${2:-<空>}" >&2; exit 2; }
      TIMEOUT="$2"; shift 2 ;;
    --exit-member) EXIT_MEMBERS+=("${2:-}"); shift 2 ;;
    --family) NFT_FAMILY="${2:-ip}"; shift 2 ;;
    --json) JSON=1; shift ;;
    --dry-run) DRY_RUN=1; shift ;;
    -h | --help) usage; exit 0 ;;
    *) echo "未知参数: $1" >&2; usage >&2; exit 2 ;;
  esac
done

# URL 会经 ssh 远端 shell 传递（单引号包裹），含单引号则无法安全传递
[[ "$ECHO_URL" != *"'"* ]] || { echo "用法错误: --echo-url 不得包含单引号" >&2; exit 2; }
command -v python3 >/dev/null 2>&1 || { echo "前置条件错误: 需要 python3" >&2; exit 2; }

# ---- 结果记录 ----------------------------------------------------------------
WORKDIR="$(mktemp -d "${TMPDIR:-/tmp}/verify-egress-XXXXXX")"
RECORDS="$WORKDIR/records.tsv"
: >"$RECORDS"
FAIL_COUNT=0
cleanup() {
  local rc=$?
  rm -rf "$WORKDIR"
  exit "$rc"
}
trap cleanup EXIT INT TERM

# 人读行：JSON 模式下全部走 stderr，保证 stdout 只有 JSON
emit_line() { # $1 文本 $2 1=强制 stderr
  if ((JSON)) || [[ "${2:-0}" == 1 ]]; then
    printf '%s\n' "$1" >&2
  else
    printf '%s\n' "$1"
  fi
}

add_check() { # $1 member(-=全局) $2 检查项 $3 ok|fail|skip $4 说明(可省)
  local member="$1" check="$2" status="$3" detail="${4:-}"
  # TSV 记录：制表符/换行会破坏列结构，统一替换为空格（仅影响展示字段）
  detail="${detail//$'\t'/ }"
  detail="${detail//$'\n'/ }"
  printf '%s\t%s\t%s\t%s\n' "$member" "$check" "$status" "$detail" >>"$RECORDS"
  local tag prefix=""
  case "$status" in
    ok) tag="[OK]  " ;;
    fail) tag="[FAIL]" ;;
    *) tag="[SKIP]" ;;
  esac
  [[ "$member" != "-" ]] && prefix="[member $member] "
  local line="  $tag $prefix$check"
  [[ -n "$detail" ]] && line="$line — $detail"
  if [[ "$status" == fail ]]; then
    FAIL_COUNT=$((FAIL_COUNT + 1))
    emit_line "$line" 1
  else
    emit_line "$line" 0
  fi
}

# ---- 目标执行：ssh 目标 / local / ns:<netns>（沙箱），全部只读 ----
run_on() { # $1 目标 $2 命令
  local spec="$1" cmd="$2"
  if ((DRY_RUN)); then
    printf '[dry-run] %s: %s\n' "${spec:-local}" "$cmd" >&2
    return 0
  fi
  case "$spec" in
    "" | local) bash -c "$cmd" ;;
    ns:*) ip netns exec "${spec#ns:}" bash -c "$cmd" ;;
    *) ssh -o BatchMode=yes -o ConnectTimeout=10 -o StrictHostKeyChecking=accept-new "$spec" "$cmd" ;;
  esac
}

# 取真实出口 IP：在本侧用 SO_MARK 建 TCP（mark=0 即不打），HTTP GET 回显服务并打印响应体。
PY_SCRIPT="$(
  cat <<'PY'
import re, socket, ssl, sys
from urllib.parse import urlparse

mark = int(sys.argv[1])
url = sys.argv[2]
timeout = float(sys.argv[3]) if len(sys.argv) > 3 else 5.0

try:
    u = urlparse(url)
    host = u.hostname
    port = u.port or (443 if u.scheme == "https" else 80)
    path = (u.path or "/") + (("?" + u.query) if u.query else "")
    s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    if mark:
        # 等价于核心侧成员出站的 routing_mark / sockopt.mark
        s.setsockopt(socket.SOL_SOCKET, socket.SO_MARK, mark)
    s.settimeout(timeout)
    s.connect((host, port))
    if u.scheme == "https":
        s = ssl.create_default_context().wrap_socket(s, server_hostname=host)
    req = "GET %s HTTP/1.0\r\nHost: %s\r\nUser-Agent: mgpanel-verify-egress\r\nConnection: close\r\n\r\n" % (path, host)
    s.sendall(req.encode())
    data = b""
    while True:
        try:
            chunk = s.recv(4096)
        except (socket.timeout, TimeoutError):
            break
        if not chunk:
            break
        data += chunk
    body = data.split(b"\r\n\r\n", 1)[-1].decode("utf-8", "replace").strip()
    # 回显服务可能是纯文本（api.ipify.org）或 HTML（ifconfig.me）→ 统一提取首个 IPv4
    match = re.search(r"\b(?:\d{1,3}\.){3}\d{1,3}\b", body)
    if match:
        print(match.group(0))
    else:
        print(body.split("\n")[0].strip() if body else "ERR: 空响应")
except Exception as e:
    print("ERR: %s: %s" % (type(e).__name__, e))
    sys.exit(3)
PY
)"

run_py() { # $1 目标 $2 mark $3 url
  local spec="$1" mark="$2" url="$3"
  if ((DRY_RUN)); then
    printf '[dry-run] %s: python3 - (SO_MARK=%s) GET %s\n' "${spec:-local}" "$mark" "$url" >&2
    return 0
  fi
  case "$spec" in
    "" | local) printf '%s' "$PY_SCRIPT" | python3 - "$mark" "$url" "$TIMEOUT" ;;
    ns:*) printf '%s' "$PY_SCRIPT" | ip netns exec "${spec#ns:}" python3 - "$mark" "$url" "$TIMEOUT" ;;
    *) printf '%s' "$PY_SCRIPT" | ssh -o BatchMode=yes -o ConnectTimeout=10 -o StrictHostKeyChecking=accept-new "$spec" \
      "python3 - $mark '$url' $TIMEOUT" ;;
  esac
}

# /30 网段地址 → 网络地址（I4 固定 /30，掩码 255.255.255.252）
net_of_30() { # $1=10.220.0.1/30
  local addr="${1%%/*}" a b c d
  IFS=. read -r a b c d <<<"$addr"
  printf '%s.%s.%s.%d/30\n' "$a" "$b" "$c" "$((d & 252))"
}

TARGET_DESC="${HOST:-local}"
EXIT_DESC="${EXIT_HOST:-<未指定>}"
emit_line "== 出口集内核分发只读核验 =="
emit_line "   入口: $TARGET_DESC   出口: $EXIT_DESC   回显服务: $ECHO_URL   超时: ${TIMEOUT}s"
((DRY_RUN)) && emit_line "   --dry-run：以下命令只打印不执行"
# 本机/netns 模式下 SO_MARK 需要 CAP_NET_ADMIN，提前提醒（否则出口 IP 对比会报 EPERM）
case "$HOST" in
  "" | local | ns:*) [[ $EUID -eq 0 ]] || emit_line "   注意：本机模式需要 root/CAP_NET_ADMIN，否则 SO_MARK 出口 IP 对比将报权限错误" 1 ;;
esac

# =============================================================================
# 0. 成员发现（未显式 --member 时由入口 pref 5500 规则反推 member = fwmark - 60000）
# =============================================================================
# ip rule show 行形如 "5500:\tfrom all fwmark 0xea64 lookup 10004"，
# fwmark 落在 I1 段（60000-64999）即可反推 member = fwmark - 60000。
discover_members() {
  local line fw="" dec=0
  while IFS= read -r line; do
    [[ "$line" == 5500:* ]] || continue
    read -ra fields <<<"$line"
    fw=""
    for ((i = 0; i < ${#fields[@]}; i++)); do
      [[ "${fields[i]}" == "fwmark" ]] && fw="${fields[i + 1]}"
    done
    [[ -n "$fw" ]] || continue
    dec=$((fw))
    ((dec >= 60000 && dec <= 64999)) && printf '%s\n' "$((dec - 60000))"
  done
}

if ((!${#MEMBERS[@]})); then
  if ((DRY_RUN)); then
    # dry-run 不读规则；未显式 --member 时只打印发现命令，成员相关命令需显式指定才会打印
    run_on "$HOST" "ip rule show"
    emit_line "   (dry-run: 未指定 --member，实际运行时将按上述规则的 fwmark 自动发现成员)"
  else
    rules_all="$(run_on "$HOST" "ip rule show" 2>/dev/null || true)"
    mapfile -t MEMBERS < <(printf '%s\n' "$rules_all" | discover_members)
    if ((${#MEMBERS[@]})); then
      emit_line "   自动发现成员: ${MEMBERS[*]}（来自入口 pref 5500 fwmark 规则）"
    else
      add_check "-" "入口 pref 5500 分发规则" fail "未发现 fwmark 60000-64999 的规则（入口未启用 l3 分发？）"
    fi
  fi
fi

# =============================================================================
# 1. 入口侧逐成员核验（I1/I2/I3/I4/I7）
# =============================================================================
PAIR_NETS=()
for m in "${MEMBERS[@]}"; do
  mark="${MARK_OVERRIDE:-$((60000 + m))}"
  table=$((10000 + m))
  iface="xe${m}"
  mark_hex="$(printf '0x%x' "$mark")"
  emit_line "== 成员 $m: mark=$mark (I1) / table=$table (I2) / 接口 $iface (I3) =="

  # --- I2: ip rule pref 5500 ---
  if ((DRY_RUN)); then
    run_on "$HOST" "ip rule show"
    add_check "$m" "ip rule pref 5500 fwmark $mark_hex lookup $table" ok "(dry-run)"
  else
    rules_out="$(run_on "$HOST" "ip rule show" 2>&1)"
    if (($? != 0)); then
      add_check "$m" "ip rule show" fail "命令失败: $rules_out"
    else
      rule_line="$(printf '%s\n' "$rules_out" | grep -E "^5500:" | grep -- "fwmark $mark_hex" | grep -- "lookup $table" || true)"
      if [[ -n "$rule_line" ]]; then
        add_check "$m" "ip rule pref 5500 fwmark $mark_hex lookup $table" ok "$rule_line"
      else
        found="$(printf '%s\n' "$rules_out" | grep -E "^5500:" | tr '\n' '|' || true)"
        add_check "$m" "ip rule pref 5500 fwmark $mark_hex lookup $table" fail \
          "缺失（入口 5500 段现有规则: ${found:-无}）"
      fi
    fi
  fi

  # --- I2: 路由表 default dev xe<member> ---
  if ((DRY_RUN)); then
    run_on "$HOST" "ip route show table $table"
    add_check "$m" "table $table default dev $iface" ok "(dry-run)"
  else
    route_out="$(run_on "$HOST" "ip route show table $table" 2>&1)"
    if [[ "$route_out" == *"default dev $iface"* ]]; then
      add_check "$m" "table $table default dev $iface" ok "$(printf '%s' "$route_out" | tr '\n' ';')"
    else
      add_check "$m" "table $table default dev $iface" fail "缺失（实际: ${route_out:-<空>}）"
    fi
  fi

  # --- I3 + I7: 接口存在性与 MTU ---
  if ((DRY_RUN)); then
    run_on "$HOST" "ip -o link show dev $iface"
    add_check "$m" "接口 $iface MTU $MTU_EXPECT" ok "(dry-run)"
    run_on "$HOST" "ip -o -4 addr show dev $iface"
  else
    link_out="$(run_on "$HOST" "ip -o link show dev $iface" 2>&1)"
    if (($? != 0)) || [[ "$link_out" != *"$iface"* ]]; then
      add_check "$m" "接口 $iface 存在 (I3)" fail "不存在（${link_out:-<空>}）"
    else
      add_check "$m" "接口 $iface 存在 (I3)" ok
      mtu="$(printf '%s' "$link_out" | sed -n 's/.* mtu \([0-9]*\).*/\1/p')"
      if [[ "$mtu" == "$MTU_EXPECT" ]]; then
        add_check "$m" "接口 $iface MTU = $MTU_EXPECT (I7)" ok "mtu $mtu"
      else
        add_check "$m" "接口 $iface MTU = $MTU_EXPECT (I7)" fail "实际 mtu ${mtu:-未知}"
      fi
    fi
    addr_out="$(run_on "$HOST" "ip -o -4 addr show dev $iface" 2>&1)"
    addr="$(printf '%s' "$addr_out" | awk '{for(i=1;i<=NF;i++) if($i=="inet"){print $(i+1); exit}}')"
    if [[ "$addr" == */30 ]]; then
      pair_net="$(net_of_30 "$addr")"
      PAIR_NETS+=("$m:$pair_net") # member:pairNet，避免与 MEMBERS 下标错位
      add_check "$m" "隧道地址 $addr (I4) → pair 网段 $pair_net" ok
    else
      add_check "$m" "隧道地址 (I4, 期望 /30)" fail "实际: ${addr:-<空>}"
    fi
  fi
done

# =============================================================================
# 2. 出口侧核验（I8，仅 --exit-host 给定时）
# =============================================================================
if [[ -n "$EXIT_HOST" ]]; then
  emit_line "== 出口侧 nft 核验: $EXIT_DESC =="
  if ((DRY_RUN)); then
    run_on "$EXIT_HOST" "nft list table $NFT_FAMILY mgpanel_egress"
    add_check "-" "出口 nft 表 mgpanel_egress (I8)" ok "(dry-run)"
  else
    nft_out="$(run_on "$EXIT_HOST" "nft list table $NFT_FAMILY mgpanel_egress" 2>&1)"
    if (($? != 0)); then
      add_check "-" "出口 nft 表 mgpanel_egress (I8)" fail "读取失败或表不存在: $nft_out"
    else
      add_check "-" "出口 nft 表 mgpanel_egress (I8)" ok
      if printf '%s\n' "$nft_out" | grep -Eq 'iifname "xi[0-9]+" accept'; then
        add_check "-" "出口 forward-ok: iifname \"xi*\" accept (I8)" ok \
          "$(printf '%s\n' "$nft_out" | grep -E 'iifname "xi[0-9]+" accept' | head -1 | sed 's/^[[:space:]]*//')"
      else
        add_check "-" "出口 forward-ok: iifname \"xi*\" accept (I8)" fail "未找到出口隧道接口的转发放行规则"
      fi
      if [[ ${#PAIR_NETS[@]} -eq 0 ]]; then
        add_check "-" "出口 masquerade 网段比对 (I8)" skip "入口未取到 pair 网段，跳过网段精确比对"
      else
        # 注意：--exit-host 只对应**一个**成员出口；其它成员的 pair 网段不在该机上属正常，
        # 标 info 而非 fail（除非用 --exit-member 指定了该出口承载的成员）。
        for entry in "${PAIR_NETS[@]}"; do
          m="${entry%%:*}"
          pn="${entry#*:}"
          if printf '%s\n' "$nft_out" | grep -q -- "saddr $pn" && printf '%s\n' "$nft_out" | grep -q 'masquerade'; then
            add_check "$m" "出口 postrouting masquerade saddr $pn (I8)" ok
          elif [[ ${#EXIT_MEMBERS[@]} -gt 0 && " ${EXIT_MEMBERS[*]} " == *" ${m#member } "* ]]; then
            add_check "$m" "出口 postrouting masquerade saddr $pn (I8)" fail "该出口应承载成员 $m 但缺少 masquerade 规则"
          else
            add_check "$m" "出口 postrouting masquerade saddr $pn (I8)" skip "该 pair 不在本出口（多出口场景正常）"
          fi
        done
      fi
    fi
  fi
fi

# =============================================================================
# 3. 真实出口 IP 对比（入口侧 python3 + SO_MARK）
# =============================================================================
MARK_USED="${MARK_OVERRIDE:-$((60000 + ${MEMBERS[0]:-4}))}"
emit_line "== 真实出口 IP 对比（mark=$MARK_USED，回显 $ECHO_URL）=="
if ((DRY_RUN)); then
  run_py "$HOST" 0 "$ECHO_URL"
  run_py "$HOST" "$MARK_USED" "$ECHO_URL"
  add_check "-" "真实出口 IP 对比" ok "(dry-run)"
else
  no_mark_ip="$(run_py "$HOST" 0 "$ECHO_URL" 2>/dev/null)"
  mark_ip="$(run_py "$HOST" "$MARK_USED" "$ECHO_URL" 2>/dev/null)"
  emit_line "   不打 mark: $no_mark_ip"
  emit_line "   打 mark  : $mark_ip"
  if [[ "$no_mark_ip" == ERR:* ]]; then
    add_check "-" "不打 mark 的出口 IP 获取" fail "$no_mark_ip"
  else
    add_check "-" "不打 mark 的出口 IP 获取" ok "$no_mark_ip"
  fi
  if [[ "$mark_ip" == ERR:* ]]; then
    add_check "-" "打 mark=$MARK_USED 的出口 IP 获取" fail "$mark_ip"
  else
    add_check "-" "打 mark=$MARK_USED 的出口 IP 获取" ok "$mark_ip"
  fi
  if [[ -n "$EXPECT_EXIT_IP" ]]; then
    token_re="(^|[^0-9.])${EXPECT_EXIT_IP//./\\.}([^0-9.]|$)"
    if printf '%s' "$mark_ip" | grep -Eq "$token_re"; then
      add_check "-" "断言 mark=$MARK_USED 出口 IP == $EXPECT_EXIT_IP" ok
    else
      add_check "-" "断言 mark=$MARK_USED 出口 IP == $EXPECT_EXIT_IP" fail "实际: $mark_ip"
    fi
  fi
fi

# =============================================================================
# 4. 结果
# =============================================================================
if ((JSON)); then
  python3 - "$RECORDS" "$TARGET_DESC" "$EXIT_DESC" "$ECHO_URL" "$MARK_USED" <<'PY'
import json, sys

records = []
for line in open(sys.argv[1], encoding="utf-8"):
    line = line.rstrip("\n")
    if not line:
        continue
    parts = (line.split("\t") + ["", "", "", ""])[:4]
    records.append({"member": parts[0], "check": parts[1], "status": parts[2], "detail": parts[3]})
doc = {
    "host": sys.argv[2],
    "exit_host": sys.argv[3],
    "echo_url": sys.argv[4],
    "mark": int(sys.argv[5]) if sys.argv[5].isdigit() else sys.argv[5],
    "ok": not any(r["status"] == "fail" for r in records),
    "summary": {
        "ok": sum(1 for r in records if r["status"] == "ok"),
        "fail": sum(1 for r in records if r["status"] == "fail"),
        "skip": sum(1 for r in records if r["status"] == "skip"),
    },
    "checks": records,
}
print(json.dumps(doc, ensure_ascii=False, indent=2))
PY
fi

if ((DRY_RUN)); then
  emit_line "dry-run 完成：以上命令均未执行（本脚本无任何写操作）"
  exit 0
fi
if ((FAIL_COUNT)); then
  emit_line "FAIL: $FAIL_COUNT 项核验未通过（详见上方 [FAIL] 行）" 1
  exit 1
fi
emit_line "PASS: 出口集内核分发核验全部通过（入口 $TARGET_DESC${EXIT_HOST:+ / 出口 $EXIT_DESC}）"
exit 0
