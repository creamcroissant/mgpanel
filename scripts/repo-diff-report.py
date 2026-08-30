#!/usr/bin/env python3
from __future__ import annotations

import datetime as dt
import difflib
import fnmatch
import hashlib
import json
import subprocess
import sys
from collections import Counter
from dataclasses import dataclass
from pathlib import Path
from typing import Iterable, List


def run(cmd: List[str], cwd: Path | None = None) -> str:
    result = subprocess.run(
        cmd,
        cwd=str(cwd) if cwd else None,
        text=True,
        capture_output=True,
        check=True,
    )
    return result.stdout


def git_ls_files(repo: Path) -> set[str]:
    out = run(["git", "-C", str(repo), "ls-files"])
    return {line.strip() for line in out.splitlines() if line.strip()}


def file_sha256(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


@dataclass
class IgnoreRule:
    pattern: str
    negated: bool
    directory_only: bool
    anchored: bool
    has_slash: bool

    @staticmethod
    def parse(line: str) -> "IgnoreRule | None":
        raw = line.strip()
        if not raw or raw.startswith("#"):
            return None

        negated = raw.startswith("!")
        if negated:
            raw = raw[1:]

        directory_only = raw.endswith("/")
        if directory_only:
            raw = raw[:-1]

        anchored = raw.startswith("/")
        if anchored:
            raw = raw[1:]

        if raw == "":
            return None

        return IgnoreRule(
            pattern=raw,
            negated=negated,
            directory_only=directory_only,
            anchored=anchored,
            has_slash=("/" in raw),
        )

    def matches(self, path: str) -> bool:
        if self.has_slash:
            candidates = [self.pattern]
            if not self.anchored:
                candidates.append(f"**/{self.pattern}")

            if self.directory_only:
                expanded = []
                for candidate in candidates:
                    expanded.append(candidate)
                    expanded.append(f"{candidate}/**")
                candidates = expanded

            return any(fnmatch.fnmatchcase(path, candidate) for candidate in candidates)

        if self.directory_only:
            parents = path.split("/")[:-1]
            return any(fnmatch.fnmatchcase(part, self.pattern) for part in parents)

        basename = path.rsplit("/", 1)[-1]
        return fnmatch.fnmatchcase(basename, self.pattern)


def load_rules(path: Path) -> list[IgnoreRule]:
    if not path.exists():
        return []

    rules: list[IgnoreRule] = []
    for line in path.read_text(encoding="utf-8").splitlines():
        parsed = IgnoreRule.parse(line)
        if parsed:
            rules.append(parsed)
    return rules


def is_ignored(path: str, rules: list[IgnoreRule]) -> bool:
    ignored = False
    for rule in rules:
        if rule.matches(path):
            ignored = not rule.negated
    return ignored


def filter_paths(paths: Iterable[str], rules: list[IgnoreRule]) -> set[str]:
    if not rules:
        return set(paths)
    return {p for p in paths if not is_ignored(p, rules)}


def top_level_counter(paths: Iterable[str]) -> list[tuple[str, int]]:
    counter: Counter[str] = Counter()
    for p in paths:
        head = p.split("/", 1)[0]
        counter[head] += 1
    return counter.most_common(20)


def compute_state(src_files: set[str], dst_files: set[str], src_repo: Path, dst_repo: Path) -> dict:
    only_src = sorted(src_files - dst_files)
    only_dst = sorted(dst_files - src_files)

    shared = sorted(src_files & dst_files)
    changed: list[str] = []
    for rel in shared:
        src = src_repo / rel
        dst = dst_repo / rel
        if not src.exists() or not dst.exists():
            continue
        if file_sha256(src) != file_sha256(dst):
            changed.append(rel)

    return {
        "only_source": only_src,
        "only_target": only_dst,
        "shared_changed": changed,
    }


def markdown_list(items: list[str]) -> str:
    if not items:
        return "(none)"
    return "\n".join(f"- `{item}`" for item in items)


def write_report(path: Path, title: str, state: dict, source_repo: Path, target_repo: Path, rule_meta: dict | None = None) -> None:
    only_source = state["only_source"]
    only_target = state["only_target"]
    shared_changed = state["shared_changed"]

    now = dt.datetime.now(dt.timezone.utc).strftime("%Y-%m-%d %H:%M:%SZ")
    source_head = run(["git", "-C", str(source_repo), "rev-parse", "--short", "HEAD"]).strip()
    target_head = run(["git", "-C", str(target_repo), "rev-parse", "--short", "HEAD"]).strip()

    lines = [
        f"# {title}",
        "",
        f"- generated_at_utc: `{now}`",
        f"- source_repo: `{source_repo}`",
        f"- target_repo: `{target_repo}`",
        f"- source_head: `{source_head}`",
        f"- target_head: `{target_head}`",
    ]

    if rule_meta is not None:
        lines.extend(
            [
                f"- syncignore_file: `{rule_meta['file']}`",
                f"- syncignore_rule_count: `{rule_meta['count']}`",
            ]
        )

    lines.extend(
        [
            "",
            "## 统计",
            "",
            "| 指标 | 数量 |",
            "| --- | ---: |",
            f"| only_in_source | {len(only_source)} |",
            f"| only_in_target | {len(only_target)} |",
            f"| shared_but_changed | {len(shared_changed)} |",
            "",
            "## 顶层目录分布（only_in_source Top20）",
            "",
        ]
    )

    top20 = top_level_counter(only_source)
    if top20:
        lines.append("| 顶层目录 | 数量 |")
        lines.append("| --- | ---: |")
        for key, value in top20:
            lines.append(f"| `{key}` | {value} |")
    else:
        lines.append("(none)")

    lines.extend(
        [
            "",
            "## only_in_source",
            "",
            markdown_list(only_source),
            "",
            "## only_in_target",
            "",
            markdown_list(only_target),
            "",
            "## shared_but_changed",
            "",
            markdown_list(shared_changed),
            "",
        ]
    )

    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text("\n".join(lines), encoding="utf-8")


def file_text_lines(path: Path) -> list[str]:
    data = path.read_bytes()
    text = data.decode("utf-8", errors="replace")
    return text.splitlines(keepends=True)


def write_unified_diff(diff_path: Path, changed_paths: list[str], source_repo: Path, target_repo: Path) -> None:
    diff_path.parent.mkdir(parents=True, exist_ok=True)
    chunks: list[str] = []

    for rel in changed_paths:
        src = source_repo / rel
        dst = target_repo / rel
        if not src.exists() or not dst.exists():
            continue

        src_lines = file_text_lines(src)
        dst_lines = file_text_lines(dst)
        patch = difflib.unified_diff(
            src_lines,
            dst_lines,
            fromfile=f"a/{rel}",
            tofile=f"b/{rel}",
            lineterm="",
        )
        rendered = "\n".join(patch)
        if rendered:
            chunks.append(rendered)

    diff_path.write_text("\n\n".join(chunks), encoding="utf-8")


def main() -> None:
    source_repo = Path(sys.argv[1]).resolve()
    target_repo = Path(sys.argv[2]).resolve()
    syncignore_file = Path(sys.argv[3]).resolve()
    full_report = Path(sys.argv[4]).resolve()
    managed_report = Path(sys.argv[5]).resolve()
    full_diff_file = Path(sys.argv[6]).resolve()
    managed_diff_file = Path(sys.argv[7]).resolve()
    check_managed = sys.argv[8] == "1"

    all_source = git_ls_files(source_repo)
    all_target = git_ls_files(target_repo)

    full_state = compute_state(all_source, all_target, source_repo, target_repo)
    write_report(
        full_report,
        "xboard vs xboard2p（全量已追踪文件差异）",
        full_state,
        source_repo,
        target_repo,
    )
    write_unified_diff(full_diff_file, full_state["shared_changed"], source_repo, target_repo)

    rules = load_rules(syncignore_file)
    managed_source = filter_paths(all_source, rules)
    managed_target = filter_paths(all_target, rules)
    managed_state = compute_state(managed_source, managed_target, source_repo, target_repo)
    write_report(
        managed_report,
        "xboard vs xboard2p（.syncignore 过滤后差异）",
        managed_state,
        source_repo,
        target_repo,
        rule_meta={"file": str(syncignore_file), "count": len(rules)},
    )
    write_unified_diff(managed_diff_file, managed_state["shared_changed"], source_repo, target_repo)

    summary = {
        "full": {
            "only_source": len(full_state["only_source"]),
            "only_target": len(full_state["only_target"]),
            "shared_changed": len(full_state["shared_changed"]),
        },
        "managed": {
            "only_source": len(managed_state["only_source"]),
            "only_target": len(managed_state["only_target"]),
            "shared_changed": len(managed_state["shared_changed"]),
        },
        "outputs": {
            "full_report": str(full_report),
            "managed_report": str(managed_report),
            "full_diff": str(full_diff_file),
            "managed_diff": str(managed_diff_file),
        },
    }
    print(json.dumps(summary, ensure_ascii=False, indent=2))

    if check_managed and any(summary["managed"].values()):
        print(
            "managed sync diff check failed: "
            f"only_source={summary['managed']['only_source']} "
            f"only_target={summary['managed']['only_target']} "
            f"shared_changed={summary['managed']['shared_changed']}",
            file=sys.stderr,
        )
        sys.exit(1)


if __name__ == "__main__":
    main()
