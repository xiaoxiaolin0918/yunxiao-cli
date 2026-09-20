# yx parity UX and docs progressive — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Keep the Go `yunxiao` CLI as the Agent/CI main path, while borrowing install discoverability, browse sugar, and docs information architecture from public npm `yx` (yunxiao-cli@0.1.5)—without weakening `--dry-run` / `--yes`.

**Architecture:** Three waves. Wave 1 is docs-only (README progressive layout, usage index, gh/yx migration map). Wave 2 adds first-class UX commands (`browse`, completion docs, optional `alias`) under existing HighRiskWrite gates. Wave 3 adds command↔docs CI checks and npm thin-wrapper / OIDC publish evaluation. Do not dual-stack with Node.

**Tech Stack:** Existing Go + cobra CLI; bilingual README; Chinese `docs/wiki`; companion skills; GitHub Issues/PRs.

## Global Constraints

- Go binary `yunxiao` remains the main stack; do not migrate to TypeScript/`yx`.
- Never weaken global `--dry-run`, `-y/--yes`, risk tiers, or "unlinkable work item => ok=false".
- Do not rename root domains to full gh surface (`pr`/`issue`); docs mapping or optional aliases only.
- Version examples must track current release (>= 0.16.12) or say "latest GitHub Release"; no pinned stale tags like 0.16.7 in install snippets.
- Detailed wiki in Chinese; keep AGENTS.md a thin English entry; sync skills how-to when behavior changes.
- Each wave must be independently reviewable/mergeable.

## Background

- A = npm `yunxiao-cli@0.1.5` command `yx` (AndersonBY): gh-flavored human CLI, npm install, shallow gates, thin pipeline.
- B = this repo Go `yunxiao` 0.16.12 (planned npm `sanzhi-yunxiao-cli`): Agent/CI path, OAuth+profiles, skills/schema, deep Flow/Codeup/workitem.
- Multi-agent consensus (云效评审 / 全栈开发 / 文档评审): keep B as standard; steal UX/docs IA from A only.

## File map

| Path | Role |
|------|------|
| `README.md`, `README.zh-CN.md` | Human 30s path + Agent block; fold or link long lists |
| `docs/wiki/01-usage/README.md` | Human module index |
| `docs/wiki/00-process/gh-yx-migration.md` | gh/`yx` → yunxiao map |
| `docs/wiki/00-process/yx-parity-ux-plan.md` | Short Chinese pointer to this plan |
| `cmd/browse.go` (+ tests) | Open console URLs |
| `cmd/alias.go` (+ tests, optional) | Local argv aliases; cannot bypass `--yes` |
| `skills/yunxiao-*/SKILL.md` | Document browse/alias/completion |
| `.github/workflows` + check script (later) | Command↔docs consistency |
| `npm/` | Thin wrapper visibility (later) |

---

## Wave 1 — Docs P0 (no runtime behavior change)

### Task 1: README progressive rewrite (CN+EN)

- [ ] Replace pinned old versions (e.g. 0.16.7) with latest Release link or current 0.16.12+.
- [ ] Add human **30-second quickstart** at top: install (Releases primary; note `sanzhi-yunxiao-cli`) → auth → three read-only tries → `--help`.
- [ ] Keep **For AI agents** block in its own section (not buried, not burying humans).
- [ ] Move long domain examples / full command dump / changelog into `<details>` or wiki links.
- [ ] Sync README.md and README.zh-CN.md.

### Task 2: Modular usage index

- [ ] Add `docs/wiki/01-usage/README.md` indexing auth, organization, project, workitem, codeup, pipeline, appstack, skills, doctor.
- [ ] One short page per group: common commands, risk, links to `--help`/`schema`/skill—no full flag tables.
- [ ] README links to the index only.

### Task 3: gh / yx migration page

- [ ] Add `docs/wiki/00-process/gh-yx-migration.md`.
- [ ] Map at least: `pr`↔`codeup mrs`/`+open-mrs`; `issue`↔`workitem`; `repo`↔`codeup repos`; `workflow`/`run`↔`pipeline`; `browse`/`status`/`search`/`alias` → present or explicit gap.
- [ ] Link from AGENTS.md (one line) and wiki process index.

**Wave 1 done when:** PR merged; docs review OK; binary behavior unchanged.

---

## Wave 2 — UX features (keep gates)

### Task 4: `yunxiao browse`

- [ ] TDD pure URL builders (no browser in unit tests).
- [ ] Commands to open repo / MR / pipeline run / workitem; read-only (no `--yes`).
- [ ] Headless: print URL and succeed (document exit behavior).
- [ ] Wiki + skill + changelog.

### Task 5: Completion docs

- [ ] Document `yunxiao completion bash|zsh|powershell` in README/usage (Windows section).
- [ ] Optional one-liner in Agent paste block.

### Task 6: Optional `alias` (P1)

- [ ] Aliases expand argv prefixes only; still honor root `--dry-run`/`--yes`.
- [ ] TDD expansion + high-risk still needs `--yes`.
- [ ] If schedule slips: document-only recommended shell aliases; defer binary.

### Task 7: MR comment threads (separate plan)

- [ ] Research OpenAPI vs existing `codeup mrs comments`; track in its own issue—do not block Waves 1–2.

**Wave 2 done when:** `browse` ships; completion docs done; alias shipped or explicitly deferred.

---

## Wave 3 — Engineering / distribution (P2)

### Task 8: Command↔docs CI check

- [ ] Export cobra command tree; fail CI if wiki/usage/skills drift badly.
- [ ] Lint: README must not pin exact patch versions in install snippets.

### Task 9: npm thin wrapper + OIDC publish eval

- [ ] Make `sanzhi-yunxiao-cli` visible beside Releases in docs.
- [ ] Evaluate GitHub OIDC npm publish (no long-lived token); publish only after explicit login/approval.

### Task 10: `status` / `search` product eval

- [ ] If API supports cheaply, plan features; else document Known gaps.

---

## Out of scope

- Replacing main stack with Node/`yx`
- Weakening risk gates
- Renaming `codeup mrs` → `pr` globally
- Rewriting pipeline to match A's thinner surface

## PDCA

- **Plan:** this file + GitHub issues
- **Do:** Wave-ordered PRs; TDD for features; CN wiki + EN AGENTS + skills sync
- **Check:** 云效评审 (features) / 文档评审 (Wave 1)
- **Act:** adjust; then decide alias / status / search