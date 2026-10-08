# -*- coding: utf-8 -*-
"""Fail CI if docs drift from core CLI surface or README pins stale patch tags outside changelog.

Wave3: parse rootCmd.AddCommand (and selected parent.AddCommand children) from cmd/*.go;
require each Use token under docs/wiki/ or AGENTS.md; keep command-tree.md in sync.
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

# Parents whose direct children are also inventory-checked.
CHILD_PARENTS = {
    "workitemCmd": "workitem",
    "authCmd": "auth",
    "codeupCmd": "codeup",
    "pipelineCmd": "pipeline",
    "aliasCmd": "alias",
    "skillsCmd": "skills",
}

PINNED_TAG = re.compile(r"releases/tag/v\d+\.\d+\.\d+")
ADD_CALL = re.compile(r"(?P<parent>\w+)\.AddCommand\(([^)]+)\)")
CMD_USE = re.compile(
    r"(?:var\s+)?(?P<var>\w+Cmd)\s*=\s*&cobra\.Command\s*\{[\s\S]*?Use:\s*\"(?P<use>[^\"]+)\"",
)


def first_use_token(use: str) -> str:
    token = use.strip().split()[0]
    return token.lstrip("+")


def parse_uses() -> dict[str, str]:
    blob = "\n".join(p.read_text(encoding="utf-8") for p in sorted(CMD_DIR.glob("*.go")))
    use_by_var: dict[str, str] = {}
    for m in CMD_USE.finditer(blob):
        use_by_var[m.group("var")] = first_use_token(m.group("use"))
    return use_by_var


def parse_add_commands(use_by_var: dict[str, str]) -> tuple[list[tuple[str, str]], dict[str, list[tuple[str, str]]]]:
    """Return root [(var, token)...] and children {parentPath: [(var, token)...]}."""
    root: list[tuple[str, str]] = []
    children: dict[str, list[tuple[str, str]]] = {p: [] for p in CHILD_PARENTS.values()}
    seen_root: set[str] = set()
    seen_child: set[tuple[str, str]] = set()

    for p in sorted(CMD_DIR.glob("*.go")):
        text = p.read_text(encoding="utf-8")
        for m in ADD_CALL.finditer(text):
            parent = m.group("parent")
            for part in m.group(2).split(","):
                var = part.strip()
                if not var:
                    continue
                token = use_by_var.get(var)
                if not token:
                    # skip non-*Cmd helpers if any
                    if var.endswith("Cmd"):
                        raise SystemExit(f"{parent}.AddCommand({var}) but Use not found in cmd/")
                    continue
                if parent == "rootCmd":
                    if var in seen_root:
                        continue
                    root.append((var, token))
                    seen_root.add(var)
                elif parent in CHILD_PARENTS:
                    path = CHILD_PARENTS[parent]
                    key = (path, var)
                    if key in seen_child:
                        continue
                    children[path].append((var, token))
                    seen_child.add(key)
    return root, children


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


def render_tree(root: list[tuple[str, str]], children: dict[str, list[tuple[str, str]]]) -> str:
    lines = [
        "# CLI 命令树（自动对照）",
        "",
        "本文件由 `scripts/check_command_docs.py` 对照 `cmd/` 里 `AddCommand` 生成/校验。",
        "新增顶层或下列父命令的子命令时：更新本表，并确保 `docs/wiki/` 或 `AGENTS.md` 某处提到该命令名。",
        "",
        "## 顶层",
        "",
        "| 命令 | Go 变量 |",
        "|------|---------|",
    ]
    for var, token in root:
        lines.append(f"| `{token}` | `{var}` |")

    for parent in sorted(children):
        lines.extend(["", f"## `{parent}` 子命令", "", "| 命令 | Go 变量 |", "|------|---------|"])
        for var, token in children[parent]:
            lines.append(f"| `{parent} {token}` | `{var}` |")

    lines.append("")
    lines.append("生成命令：`python scripts/check_command_docs.py --write-tree`")
    lines.append("")
    return "\n".join(lines)


def check_tree_doc(
    root: list[tuple[str, str]],
    children: dict[str, list[tuple[str, str]]],
    errors: list[str],
) -> None:
    if not TREE_DOC.exists():
        errors.append(f"missing {TREE_DOC.relative_to(ROOT).as_posix()}")
        return
    text = TREE_DOC.read_text(encoding="utf-8")
    for _, token in root:
        if f"`{token}`" not in text:
            errors.append(f"command-tree.md missing top-level command `{token}`")
    for parent, kids in children.items():
        for _, token in kids:
            needle = f"`{parent} {token}`"
            if needle not in text:
                errors.append(f"command-tree.md missing child {needle}")


def main(argv: list[str] | None = None) -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument(
        "--write-tree",
        action="store_true",
        help="rewrite docs/wiki/01-usage/command-tree.md from cmd/ and exit 0",
    )
    args = ap.parse_args(argv)

    try:
        use_by_var = parse_uses()
        root, children = parse_add_commands(use_by_var)
    except SystemExit as e:
        print(f"check-command-docs FAILED: {e}")
        return 1

    if args.write_tree:
        TREE_DOC.parent.mkdir(parents=True, exist_ok=True)
        TREE_DOC.write_text(render_tree(root, children), encoding="utf-8", newline="\n")
        n_child = sum(len(v) for v in children.values())
        print(
            f"wrote {TREE_DOC.relative_to(ROOT).as_posix()} "
            f"({len(root)} top-level, {n_child} children)"
        )
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
    for _, token in root:
        if token not in wiki:
            errors.append(
                f"top-level command {token!r} not mentioned under docs/wiki/ or AGENTS.md"
            )
    for parent, kids in children.items():
        for _, token in kids:
            # child token alone is enough (e.g. "comments"); prefer full path if unique noise
            if token not in wiki and f"{parent} {token}" not in wiki:
                errors.append(
                    f"child command {parent!r}/{token!r} not mentioned under docs/wiki/ or AGENTS.md"
                )

    check_tree_doc(root, children, errors)

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
    n_child = sum(len(v) for v in children.values())
    print(f"check-command-docs OK ({len(root)} top-level, {n_child} tracked children)")
    return 0


if __name__ == "__main__":
    sys.exit(main())
