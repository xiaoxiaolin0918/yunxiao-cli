#!/usr/bin/env python3
"""Bump yunxiao-cli version + bilingual Changelog stubs, optionally tag.

Usage:
  python scripts/bump_release.py 0.16.40
  python scripts/bump_release.py 0.16.40 --notes-file notes.md
  python scripts/bump_release.py 0.16.40 --dry-run
  python scripts/bump_release.py 0.16.40 --commit --tag   # local commit + annotated tag only

Does NOT push or create a GitHub Release (release.yml runs on tag push).
"""
from __future__ import annotations

import argparse
import datetime as dt
import pathlib
import re
import subprocess
import sys

ROOT = pathlib.Path(__file__).resolve().parents[1]
VERSION_GO = ROOT / "internal" / "version" / "version.go"
README_EN = ROOT / "README.md"
README_ZH = ROOT / "README.zh-CN.md"
WIKI_DIR = ROOT / "docs" / "wiki" / "changelog"

VERSION_RE = re.compile(r"^var Version = \"([^\"]+)\"", re.M)
SEMVER_RE = re.compile(r"^\d+\.\d+\.\d+$")


def read_version() -> str:
    text = VERSION_GO.read_text(encoding="utf-8")
    m = VERSION_RE.search(text)
    if not m:
        raise SystemExit(f"cannot parse Version in {VERSION_GO}")
    return m.group(1)


def write_version(new: str) -> None:
    text = VERSION_GO.read_text(encoding="utf-8")
    text2, n = VERSION_RE.subn(f'var Version = "{new}"', text, count=1)
    if n != 1:
        raise SystemExit("failed to rewrite Version")
    VERSION_GO.write_text(text2, encoding="utf-8", newline="\n")


def insert_changelog(path: pathlib.Path, version: str, bullet: str, heading_prefix: str = "- **") -> None:
    text = path.read_text(encoding="utf-8")
    if f"**{version}**" in text:
        raise SystemExit(f"{version} already present in {path}")
    lines = text.splitlines(keepends=True)
    out: list[str] = []
    inserted = False
    in_changelog = False
    entry = f"{heading_prefix}{version}** — {bullet}\n"
    for line in lines:
        if not in_changelog and line.startswith("## ") and (
            "Changelog" in line or "变更" in line or "更新日志" in line
        ):
            in_changelog = True
            out.append(line)
            continue
        if in_changelog and not inserted and line.startswith("- **"):
            out.append(entry)
            out.append(line)
            inserted = True
            in_changelog = False
            continue
        out.append(line)
    if not inserted:
        raise SystemExit(f"Changelog insert point not found in {path}")
    path.write_text("".join(out), encoding="utf-8", newline="\n")


def write_wiki(version: str, bullet: str, dry: bool) -> pathlib.Path:
    WIKI_DIR.mkdir(parents=True, exist_ok=True)
    path = WIKI_DIR / f"{version}.md"
    if path.exists():
        raise SystemExit(f"wiki page exists: {path}")
    today = dt.date.today().isoformat()
    body = f"""# {version}

Date: {today}

## Highlights

- {bullet}

## Links

- GitHub Release: https://github.com/xiaoxiaolin0918/yunxiao-cli/releases/tag/v{version}
"""
    if not dry:
        path.write_text(body, encoding="utf-8", newline="\n")
    return path


def run(cmd: list[str]) -> None:
    print("+", " ".join(cmd))
    subprocess.check_call(cmd, cwd=ROOT)


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("version", help="new semver without leading v, e.g. 0.16.40")
    ap.add_argument("--notes", default="", help="one-line changelog summary")
    ap.add_argument("--notes-file", type=pathlib.Path, help="file with changelog summary")
    ap.add_argument("--dry-run", action="store_true")
    ap.add_argument("--commit", action="store_true", help="git commit the bump")
    ap.add_argument("--tag", action="store_true", help="create annotated tag vVERSION (local only)")
    args = ap.parse_args()

    new = args.version.lstrip("v")
    if not SEMVER_RE.match(new):
        raise SystemExit(f"invalid semver: {args.version}")

    old = read_version()
    parts_old = [int(x) for x in old.split(".")]
    parts_new = [int(x) for x in new.split(".")]
    if parts_new <= parts_old:
        raise SystemExit(f"new version {new} must be greater than current {old}")

    bullet = args.notes.strip()
    if args.notes_file:
        bullet = args.notes_file.read_text(encoding="utf-8").strip().splitlines()[0].strip()
    if not bullet:
        bullet = "TBD — fill before tagging"

    print(f"bump {old} -> {new}")
    print(f"notes: {bullet}")

    if args.dry_run:
        print("dry-run: would update version.go, README.md, README.zh-CN.md, docs/wiki/changelog/")
        return 0

    write_version(new)
    insert_changelog(README_EN, new, bullet)
    insert_changelog(README_ZH, new, bullet)
    wiki = write_wiki(new, bullet, dry=False)
    print(f"wrote {VERSION_GO.relative_to(ROOT)}")
    print(f"wrote changelogs + {wiki.relative_to(ROOT)}")

    if args.commit:
        run(["git", "add", str(VERSION_GO.relative_to(ROOT)), "README.md", "README.zh-CN.md", str(wiki.relative_to(ROOT))])
        run(["git", "commit", "-m", f"chore: bump version to {new}"])
    if args.tag:
        run(["git", "tag", "-a", f"v{new}", "-m", f"v{new}"])
        print(f"tagged v{new} locally — push with: git push origin main --tags")
    else:
        print("next: review diffs, commit, tag v%s, push (release.yml builds assets)" % new)
    return 0


if __name__ == "__main__":
    sys.exit(main())
