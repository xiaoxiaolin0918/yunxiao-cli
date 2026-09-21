#!/usr/bin/env python3
"""Fail CI if docs drift from core CLI surface or README pins stale patch tags outside changelog."""
from __future__ import annotations

import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]

REQUIRED_DOC_PATHS = [
    "docs/wiki/01-usage/README.md",
    "docs/wiki/00-process/gh-yx-migration.md",
    "docs/wiki/changelog",
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


def main() -> int:
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
    print("check-command-docs OK")
    return 0


if __name__ == "__main__":
    sys.exit(main())