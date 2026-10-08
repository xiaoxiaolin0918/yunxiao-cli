# -*- coding: utf-8 -*-
"""Fail CI if docs drift from core CLI surface or README pins stale patch tags outside changelog.

Wave3: also parse rootCmd.AddCommand from cmd/*.go and require each top-level
command Use token to appear somewhere under docs/wiki/ (or AGENTS.md).
Writes/validates docs/wiki/01-usage/command-tree.md inventory when --write-tree.
"""
from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
CMD_DIR = ROOT / "cmd"
TREE_DOC = ROOT / "docs" / "wiki" / "01-usage" / "command-tree.md"

REQUIRED_DOC_PATHS = [
    "docs/wiki/01-usage/README.md",
    "docs/wiki/00-process/gh-yx-migration.md",
    "docs/wiki/changelog",
    "docs/wiki/01-usage/command-tree.md",
]

REQUIRED_MENTIONS = [
    "browse",
    "alias",
    "pipeline",
    "codeup",
    "workitem",
    "auth",
    "doctor",
]

PINNED_TAG = re.compile(r"releases/tag/v\d+\.\d+\.\d+")
ADD_ROOT = re.compile(r"rootCmd\.AddCommand\(([^)]+)\)")
CMD_USE = re.compile(
    r"(?:var\s+)?(?P<var>\w+Cmd)\s*=\s*&cobra\.Command\s*\{[\s\S]*?Use:\s*\"(?P<use>[^\"]+)\"",
)


def first_use_token(use: str) -> str:
    token = use.strip().split()[0]
    return token.lstrip("+")


def parse_root_commands() -> list[tuple[str, str]]:
    """Return [(varName, useToken), ...] in discovery order."""
    blob = "\n".join(p.read_text(encoding="utf-8") for p in sorted(CMD_DIR.glob("*.go")))
    use_by_var: dict[str, str] = {}
    for m in CMD_USE.finditer(blob):
        use_by_var[m.group("var")] = first_use_token(m.group("use"))

    ordered: list[tuple[str, str]] = []
    seen: set[str] = set()
    for p in sorted(CMD_DIR.glob("*.go")):
        text = p.read_text(encoding="utf-8")
        for m in ADD_ROOT.finditer(text):
            for part in m.group(1).split(","):
                var = part.strip()
                if not var or var in seen:
                    continue
                token = use_by_var.get(var)
                if not token:
                    raise SystemExit(f"rootCmd.AddCommand({var}) but Use not found in cmd/")
                ordered.append((var, token))
                seen.add(var)
    return ordered


def wiki_blob() -> str:
    parts: list[str] = []
    wiki = ROOT / "docs" / "wiki"
    if wiki.exists():
        for p in wiki.rglob("*"):
            if p.is_file() and p.suffix.lower() in {".md", ".txt"}:
                parts.append(p.read_text(encoding="utf-8"))
    agents = ROOT / "AGENTS.md"
    if agents.exists():
        parts.append(agents.read_text(encoding="utf-8"))
    return "\n".join(parts)


def render_tree(cmds: list[tuple[str, str]]) -> str:
    lines = [
        "# CLI 顶层命令树（自动对照）",
        "",
        "本文件由 `scripts/check_command_docs.py` 对照 `cmd/` 里 `rootCmd.AddCommand` 生成/校验。",
        "新增顶层命令时：更新本表，并确保 `docs/wiki/` 或 `AGENTS.md` 某处提到该命令名。",
        "",
        "| 命令 | Go 变量 |",
        "|------|---------|",
    ]
    for var, token in cmds:
        lines.append(f"| `{token}` | `{var}` |")
    lines.append("")
    lines.append("生成命令：`python scripts/check_command_docs.py --write-tree`")
    lines.append("")
    return "\n".join(lines)


def check_tree_doc(cmds: list[tuple[str, str]], errors: list[str]) -> None:
    if not TREE_DOC.exists():
        errors.append(f"missing {TREE_DOC.relative_to(ROOT).as_posix()}")
        return
    text = TREE_DOC.read_text(encoding="utf-8")
    for _, token in cmds:
        if f"`{token}`" not in text:
            errors.append(f"command-tree.md missing top-level command `{token}`")


def main(argv: list[str] | None = None) -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument(
        "--write-tree",
        action="store_true",
        help="rewrite docs/wiki/01-usage/command-tree.md from cmd/ and exit 0",
    )
    args = ap.parse_args(argv)

    try:
        cmds = parse_root_commands()
    except SystemExit as e:
        print(f"check-command-docs FAILED: {e}")
        return 1

    if args.write_tree:
        TREE_DOC.parent.mkdir(parents=True, exist_ok=True)
        TREE_DOC.write_text(render_tree(cmds), encoding="utf-8", newline="\n")
        print(f"wrote {TREE_DOC.relative_to(ROOT).as_posix()} ({len(cmds)} commands)")
        return 0

    errors: list[str] = []
    for rel in REQUIRED_DOC_PATHS:
        if not (ROOT / rel).exists():
            errors.append(f"missing required doc path: {rel}")

    parts = []
    for rel in (
        "docs/wiki/01-usage/README.md",
        "docs/wiki/00-process/gh-yx-migration.md",
        "AGENTS.md",
    ):
        p = ROOT / rel
        if p.exists():
            parts.append(p.read_text(encoding="utf-8"))
    blob = "\n".join(parts)
    for name in REQUIRED_MENTIONS:
        if name not in blob:
            errors.append(f"command {name!r} not mentioned in usage/migration/AGENTS")

    wiki = wiki_blob()
    for _, token in cmds:
        if token not in wiki:
            errors.append(
                f"top-level command {token!r} not mentioned under docs/wiki/ or AGENTS.md"
            )

    check_tree_doc(cmds, errors)

    for readme in ("README.md", "README.zh-CN.md"):
        text = (ROOT / readme).read_text(encoding="utf-8")
        in_changelog = False
        for i, line in enumerate(text.splitlines(), 1):
            if line.startswith("## ") and ("Changelog" in line or "变更" in line):
                in_changelog = True
            elif line.startswith("## "):
                in_changelog = False
            if in_changelog:
                continue
            if PINNED_TAG.search(line):
                errors.append(
                    f"{readme}:{i}: pinned release tag in non-changelog section: {line.strip()[:80]}"
                )

    if errors:
        print("check-command-docs FAILED:")
        for e in errors:
            print(" -", e)
        return 1
    print(f"check-command-docs OK ({len(cmds)} top-level commands)")
    return 0


if __name__ == "__main__":
    sys.exit(main())
