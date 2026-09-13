#!/usr/bin/env bash
# verify-routing-policy.sh —— 生产只读核验分流产物（灰度证据，不做任何写操作）。
#
# 流程：
#   1. 用 ssh 把远端 conf 目录（默认 /etc/sing-box/conf）tar 流式拉到临时目录；
#   2. 用 SINGBOX_BIN 执行 `merge -C <临时目录>` 生成单文件配置；
#   3. 用 python3 校验并打印：规则求值顺序、每条规则 outbound 是否存在、
#      loadbalance 池的 strategy/fallback/成员、rule_set 的 tag 与 url；
#      发现不一致时 exit 1。
#
# 注意：sing-box merge（badjson）是按 schema 反序列化后再序列化，Listable 字段
# （inbound/tag 列表等）会被展开为标量（["a"] → "a"），因此校验用“匹配字段指纹”
# 比较（单元素数组与标量视为等价），而非逐字节比较。
#
# 用法：
#   scripts/verify-routing-policy.sh root@node.example.com
#   SSHK="-o ProxyJump=root@1.2.3.4 root@5.6.7.8" scripts/verify-routing-policy.sh
#
# 环境变量：
#   SSHK             ssh 连接串（连接参数与主机；已含主机时不要再传位置参数）
#   SINGBOX_BIN      执行 merge 的 sing-box 二进制（默认 /opt/mgpanel/bin/sing-box）
#   REMOTE_CONF_DIR  远端 conf 目录（默认 /etc/sing-box/conf）
#
# 退出码：0 = 全部一致；1 = 发现不一致/merge 失败；2 = 用法或前置条件错误（无法核验）
set -euo pipefail

TARGET="${1:-}"
SSHK="${SSHK:-}"
SINGBOX_BIN="${SINGBOX_BIN:-/opt/mgpanel/bin/sing-box}"
REMOTE_CONF_DIR="${REMOTE_CONF_DIR:-/etc/sing-box/conf}"

ssh_args=()
if [[ -n "$SSHK" ]]; then
  read -r -a ssh_args <<<"$SSHK"
fi
if [[ -n "$TARGET" ]]; then
  ssh_args+=("$TARGET")
fi
if [[ ${#ssh_args[@]} -eq 0 ]]; then
  echo "usage: $0 [user@host]   (或 SSHK=\"<ssh 参数> [user@host]\")" >&2
  exit 2
fi
if [[ ! -x "$SINGBOX_BIN" ]]; then
  echo "sing-box binary not executable: $SINGBOX_BIN (用 SINGBOX_BIN=... 指定)" >&2
  exit 2
fi
if ! command -v python3 >/dev/null 2>&1; then
  echo "python3 is required" >&2
  exit 2
fi

WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/verify-routing-policy-XXXXXX")"
trap 'rm -rf "$WORK_DIR"' EXIT

CONF_DIR="$WORK_DIR/conf"
MERGED="$WORK_DIR/merged.json"
mkdir -p "$CONF_DIR"

echo "==> 拉取 ${ssh_args[*]}:$REMOTE_CONF_DIR"
ssh "${ssh_args[@]}" "tar -C '$REMOTE_CONF_DIR' -cf - ." > "$WORK_DIR/conf.tar"
tar -C "$CONF_DIR" -xf "$WORK_DIR/conf.tar"
rm -f "$WORK_DIR/conf.tar"
echo "    本地文件数: $(find "$CONF_DIR" -type f | wc -l)"

echo "==> merge -C $CONF_DIR ($SINGBOX_BIN)"
if ! "$SINGBOX_BIN" merge "$MERGED" -C "$CONF_DIR" >/dev/null; then
  echo "merge 失败（需支持 loadbalance 的 fork 二进制）: $SINGBOX_BIN merge -C $CONF_DIR" >&2
  exit 1
fi

python3 - "$MERGED" "$CONF_DIR" <<'PY'
import json
import os
import sys

merged_path, conf_dir = sys.argv[1], sys.argv[2]
with open(merged_path, encoding="utf-8") as fh:
    merged = json.load(fh)

problems = []
route = merged.get("route") or {}
rules = route.get("rules") or []
outbounds = [o for o in (merged.get("outbounds") or []) if isinstance(o, dict)]
tags = [o.get("tag") for o in outbounds if o.get("tag")]


def as_list(value):
    if value is None:
        return []
    return value if isinstance(value, list) else [value]


def scalar(value):
    items = as_list(value)
    return items[0] if items else None


def unwrap(value):
    """sing-box merge 会把 Listable 单元素数组展开为标量；比较时统一形状。"""
    if isinstance(value, list):
        items = [unwrap(v) for v in value]
        return items[0] if len(items) == 1 else items
    if isinstance(value, dict):
        return {k: unwrap(v) for k, v in value.items()}
    return value


MATCH_KEYS = (
    "inbound", "inboundTag", "rule_set", "domain", "domain_suffix", "domain_keyword",
    "ip", "ip_cidr", "geosite", "geoip", "protocol", "port", "network", "source_ip_cidr",
    "user", "clash_mode",
)
TARGET_KEYS = ("outbound", "outboundTag", "action")


def fingerprint(rule):
    return {k: unwrap(rule[k]) for k in MATCH_KEYS + TARGET_KEYS if k in rule}


# ---- 规则求值顺序：文件名升序（agent 侧 normalizeArtifacts 与 sing-box -C 均为字典序合并数组）----
rule_files = []
for name in sorted(os.listdir(conf_dir)):
    path = os.path.join(conf_dir, name)
    if not os.path.isfile(path) or not name.endswith(".json"):
        continue
    try:
        with open(path, encoding="utf-8") as fh:
            data = json.load(fh)
    except (OSError, ValueError) as exc:
        problems.append("配置文件解析失败 %s: %s" % (name, exc))
        continue
    if isinstance(data, dict):
        file_rules = ((data.get("route") or {}).get("rules")) or []
        if file_rules:
            rule_files.append((name, file_rules))

expected_fp = [fingerprint(r) for _, file_rules in rule_files for r in file_rules]
actual_fp = [fingerprint(r) for r in rules]
if expected_fp != actual_fp:
    problems.append(
        "merge 后的 route.rules 与按文件名字典序拼接的结果不一致（顺序/内容漂移）："
        "merged=%d 条, expected=%d 条" % (len(rules), len(expected_fp))
    )

print("== 规则求值顺序（%d 条，源文件 %d 个）==" % (len(rules), len(rule_files)))
index = 0
for name, file_rules in rule_files:
    for _ in file_rules:
        if index >= len(rules):
            break
        rule = rules[index]
        match = []
        for key in MATCH_KEYS:
            val = rule.get(key)
            if val:
                items = as_list(val)
                match.append("%s=%s" % (key, ",".join(str(v) for v in items)))
        target = scalar(rule.get("outbound", rule.get("outboundTag", rule.get("action", "-"))))
        print("  [%02d] %-46s -> %s   <- %s" % (index, " ".join(match) or "-", target, name))
        index += 1
for extra in rules[index:]:
    print("  [%02d] (无源文件归属) %s" % (index, json.dumps(extra, ensure_ascii=False)))
    index += 1

# ---- 规则 outbound 引用存在性 ----
for i, rule in enumerate(rules):
    target = scalar(rule.get("outbound", rule.get("outboundTag")))
    if target and target not in tags:
        problems.append("规则 #%d 的 outbound %r 不在 outbounds 中" % (i, target))

# ---- loadbalance 池 ----
allowed_strategy = {"round-robin", "least-connections", "source-hash", "consistent-hash"}
pools = [o for o in outbounds if o.get("type") == "loadbalance"]
print("== loadbalance 池（%d 个）==" % len(pools))
for pool in pools:
    tag = pool.get("tag")
    strategy = pool.get("strategy")
    fallback = pool.get("fallback")
    members = as_list(pool.get("outbounds"))
    print("  tag=%s strategy=%s fallback=%s members=%s" % (tag, strategy, fallback, members))
    if strategy not in allowed_strategy:
        problems.append("池 %s 的 strategy=%r 不在 %s 中" % (tag, strategy, sorted(allowed_strategy)))
    if fallback != "direct":
        problems.append("池 %s 的 fallback=%r，应为 \"direct\"" % (tag, fallback))
    if not members:
        problems.append("池 %s 成员为空" % tag)
    if "direct" in members:
        problems.append("池 %s 把 direct 当作分摊成员（会直连泄漏，应用 fallback）" % tag)
    for member in members:
        if member not in tags:
            problems.append("池 %s 成员 %r 不在 outbounds 中" % (tag, member))
    if fallback and fallback not in tags:
        problems.append("池 %s 的 fallback %r 不在 outbounds 中" % (tag, fallback))

# ---- rule_set ----
rule_sets = [rs for rs in as_list(route.get("rule_set")) if isinstance(rs, dict)]
rule_set_tags = [rs.get("tag") for rs in rule_sets]
print("== rule_set（%d 个）==" % len(rule_sets))
for rs in rule_sets:
    rs_tag = rs.get("tag")
    rs_type = rs.get("type")
    rs_url = rs.get("url")
    print("  tag=%s type=%s url=%s download_detour=%s" % (rs_tag, rs_type, rs_url, rs.get("download_detour")))
    if not rs_tag:
        problems.append("rule_set 缺少 tag: %s" % json.dumps(rs, ensure_ascii=False))
    if rs_type == "remote" and not rs_url:
        problems.append("remote rule_set %s 缺少 url" % rs_tag)
    detour = rs.get("download_detour")
    if detour and detour not in tags:
        problems.append("rule_set %s 的 download_detour %r 不在 outbounds 中" % (rs_tag, detour))

# ---- 规则引用 rule_set 的存在性 ----
for i, rule in enumerate(rules):
    for ref in as_list(rule.get("rule_set")):
        if ref not in rule_set_tags:
            problems.append("规则 #%d 引用未定义的 rule_set %r" % (i, ref))

print("== outbounds（%d 个，direct=%s）==" % (len(tags), "direct" in tags))
print("  " + ", ".join(str(t) for t in tags))

if problems:
    print("\n== 不一致（%d 项）==" % len(problems))
    for p in problems:
        print("  - " + p)
    sys.exit(1)

print("\nOK: 顺序、outbound 引用、loadbalance 池、rule_set 均一致")
PY
