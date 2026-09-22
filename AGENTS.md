# AGENTS.md — Yunxiao CLI

Practical rules for humans and AI agents working on **yunxiao-cli** (阿里云云效 CLI, Feishu-style progressive discovery).

## Purpose

- Migrating from GitHub CLI or public npm `yx`? See [docs/wiki/00-process/gh-yx-migration.md](docs/wiki/00-process/gh-yx-migration.md).

- CLI binary: `yunxiao` — DevOps API client for Yunxiao (organization, Projex, Codeup, Flow, Packages, Testhub, AppStack).
- Designed for **humans + AI agents**: progressive discovery, risk gates, stable JSON envelopes.
- Prefer loading skill `yunxiao-shared` first, then domain skills as needed.

## Build & run

After changing the command surface or install docs, run `python scripts/check_command_docs.py` (also in CI).


```bash
# Preferred for end users / agents (Feishu-style):
#   npx yunxiao-cli@latest install
# From this checkout before npmjs publish:
#   npm install -g ./npm   OR   npm pack -C npm && npm install -g ./yunxiao-cli-*.tgz

# Go version: see go.mod (currently go 1.24.x; treat as minimum)
make build          # -> ./yunxiao (ldflags Version)
go build -o yunxiao .  # fallback default Version without ldflags
make test           # go test ./...
./yunxiao --version # 0.15.3+
```

Do **not** assume a published `go install` / npmjs path works in every environment yet; local `make build` or `npm install -g ./npm` is the source of truth for this checkout.

## Recent (0.15.3)

- Feishu-style npm one-click installer under `npm/` (`npx yunxiao-cli@latest install`): `npm/scripts/run.js` + `postinstall` → `npm/scripts/install.js` (bundled `releases/` or `YUNXIAO_CLI_DOWNLOAD_BASE`), wizard installs skills and prints auth next steps.
- Default `version.Version` bumped to `0.15.3`; platform archives rebuilt with matching ldflags.

## Prior (0.15.2)


- Companion skills refresh (shared 1.1.0, pipeline 1.1.0, project 1.2.0, codeup 1.1.0, yunxiao-zhiyi-ops 1.3.0; light-touch appstack/organization/packages/testhub 1.0.1): document list `meta.has_more`/`total`/`page`/`pagination`, common `meta.url`, transition `refresh_ok`, Flow console URLs, pipeline run examples + `--yes` on trigger, MR per-item `url`, relations enrich, `--cancel-reason`.
- `client.ListAll` (cap 50 pages) + unit tests; wired `--all` on `pipeline list` and `codeup mrs list`. Available for other callers.
- `scripts/flow-ci.sh`: install Go from `mirrors.aliyun.com/golang`, default `GOPROXY=goproxy.cn` — for Flow public runners / parent `pipeline update`.

## Prior (0.15.1)


- B5 wave2 (0.15.1): broader migration onto `runRead` / `runJSONMutating` — workitem update (+cancel-reason soft-warn dry-run preserved) & relations list enrich via after-hook; codeup files/branches/tags/protected-branches/mrs writes; pipeline trigger/cancel/job/flow-vg; org/project/sprint/versions/packages/testhub/appstack(+rw/deploy)/effort/programs/vm-deploy reads + simple writes. Still hand-rolled: attachments multipart, pipeline create/update (YAML dry-run redaction), MR create multi-step, workitem +shortcuts, testhub results fallback, sprint aggregate, `api` raw.
- B5 (0.15.0): helpers landed; initial workitem typed subset, codeup list/get, pipeline list/get migrated.
- C1: split `cmd/workitem.go` into `workitem_{search,get,comments,create,update,relations,attachments,meta,register}.go` (same package).
- C2: `expectedCompletionDateRe` precompiled in `workitem_bug_create.go`.
- C3: `client.Do` returns `(http.Header, error)`; typed Get/Post/… stay error-only; pagination call sites use `Do` + `MetaWithPagination` (DoRaw kept for status).
- C4: `version.Version` is a `var` default `"0.15.3"`; `make build`/`ci`/`scripts/ci.sh` inject via `-ldflags -X …Version=`.

## Prior (0.14.11)

- Pipeline/run responses attach Flow console `meta.url` / item `url` via `flow.aliyun.com` (`PipelineURL` / `PipelineRunURL` in `internal/zhiyi`).

## Prior (0.14.10)

- A1: accept git-history residual — commits ≤ v0.14.5 may contain old tenant org/space/status-machine IDs in example profiles; current tree is placeholderized; residual assessed acceptable (no `git filter-repo` rewrite).
- D1: `Retry-After` sleep capped at **30s** in `backoffDuration` (e.g. `86400` → sleep 30s); unit test locks the cap.
- C5: README duplicate EN examples removed; document `go install` vs skills-tree discovery limitation.

## Prior (0.14.9)

- B1: HTTP client methods take `context.Context` (`Get`/`Put`/`Post`/`Delete`/`Do`/`DoRaw`/`PostMultipart` + path builders); cmd call sites pass `cmd.Context()`.
- B1: idempotent GET/HEAD retry (up to 3 retries) on 429/5xx/network errors with exponential backoff; respects `Retry-After`; POST/PUT/DELETE not retried.
- P2: `has_more` also inferred from `page*per_page < total` (and total_pages) when `x-next-page` is omitted.
- P2: more list commands wired through `MetaWithPagination` (repos/branches/tags, sprints, versions, hostGroups, variableGroups, org departments, appstack change-orders/logs/executions).

## Prior (0.14.8)

- B3 E2E: cobra `Execute` high-risk path without `--yes` → exit 10 + `confirmation_required` (test intercepts `processExit`).
- B4: `refreshAfterTransition` helper; unit tests lock stderr warning + `refresh_ok` true/false on success path.
- B2: list envelope `meta` exposes `has_more` / `total` / `page` via `MetaWithPagination` (MR list now uses it); ListAll shipped in 0.15.2 (`--all` on pipeline list / codeup mrs list).

## Prior (0.14.7)

- Help/skills: copy-paste examples use `<type-id>` / `<repo-id>` placeholders (no real tenant IDs).
- B3: Write vs HighRiskWrite gate contracts locked in tests; PostMultipart error redaction httptest.
- Transition success JSON includes `refresh_ok` (stderr warning unchanged on refresh miss).

## Three-layer command model

Prefer in this order:

1. **`+shortcuts`** — high-level tasks (`yunxiao project +my-open-items`, `yunxiao codeup +open-mrs`, …)
2. **Typed domain commands** — one OpenAPI-shaped method (`yunxiao codeup mrs list --state opened`)
3. **`yunxiao api`** — raw escape hatch (`yunxiao api GET /oapi/v1/platform/user`)

Never invent endpoints. If a typed command does not exist, use `yunxiao api` only with a confirmed OpenAPI path, or skip.

## Progressive discovery

| Do | Don't |
|----|--------|
| `yunxiao --help` / `yunxiao <domain> --help` | Dump every flag/tool into context |
| `yunxiao schema <id>` (e.g. `codeup.mrs.create`) | Guess required params |
| `yunxiao skills list` / `skills read <name>` | Paste entire skill trees unless needed |
| Start from `yunxiao-shared` | Load all skills up front |

## Risk gates

Each mutating command documents risk in `--help` Long:

| Level | Behavior |
|-------|----------|
| `read` | Safe to run |
| `write` | Confirm intent; prefer `--dry-run` first |
| `high-risk-write` | Needs **user confirmation**, then retry with `--yes`. Without `--yes`: exit **10**, stderr `confirmation_required`. **Never** silently add `--yes`. |

Global `--dry-run` previews without executing (API or local write previews).

## JSON contract

- Success: stdout JSON with **`ok == true`** (and `data` / optional `meta`).
- Errors: stderr JSON with `ok == false` and `error` object.
- Dry-run success: `ok == true` and `dry_run == true` (request/preview payload).
- **Do not** treat Yunxiao OpenAPI body field `code == 0` as the CLI success signal — use the CLI envelope `ok`.
- Workitem get/create/update (and transition shortcuts) include **`meta.url`** (Projex web link) plus `serial_number` / `resolved_id` when known. Shortcut composed results (e.g. `+bug-create`) also expose top-level `url`.
- Codeup MR get/create/`+create` set **`meta.url`**; list / `+open-mrs` inject per-item `url` (prefers API `detailUrl`, else constructs `…/change/{localId}`).

Filter with `--jq '<expr>'`. Prefer `--format pretty` only for humans.

## Auth

- Env (preferred for CI/agents): `YUNXIAO_ACCESS_TOKEN`, optional `YUNXIAO_ORGANIZATION_ID`
- Or: `yunxiao auth login --token '…'` then `yunxiao auth status` / `yunxiao doctor`
- Optional per-profile PAT: `"access_token"` in `~/.config/yunxiao/profiles/<name>.json` (prefer mode 0600; never commit real tokens)
- Token precedence: `YUNXIAO_ACCESS_TOKEN` > active profile `access_token` > `config.json`; `token_source` is `env` | `profile` | `config` | `none`
- Header used by client: `x-yunxiao-token`
- **Never echo full tokens** in logs, commits, or chat. Redact to a short prefix if debugging.

## Agent skills

Source layout: `skills/yunxiao-*` (each has `SKILL.md`, optional `references/`).

| Skill | Focus |
|-------|--------|
| `yunxiao-shared` | Auth, config, doctor, JSON/`--dry-run`/`--yes` contract |
| `yunxiao-organization` | Orgs, members, departments, roles |
| `yunxiao-project` | Projex projects / workitems |
| `yunxiao-codeup` | Repos, branches, files, MRs |
| `yunxiao-pipeline` | Flow pipelines, runs, jobs, YAML |
| `yunxiao-packages` | Artifact repos / artifacts |
| `yunxiao-testhub` | Test plans / results / comments |
| `yunxiao-appstack` | Apps, change-orders, orchestrations, tags, VGs |
| `yunxiao-yunxiao-zhiyi-ops` | Zhiyi/ZYPT profile + sprint/bug-create/transition/MR shortcuts |

### Install into agent skills dir

Recommended local install (default target `~/.agents/skills`):

```bash
yunxiao skills install
yunxiao skills install --dir /path/to/skills
yunxiao skills install --skill yunxiao-shared --skill yunxiao-codeup
yunxiao skills install --symlink          # link instead of copy
yunxiao skills install --force            # replace existing
yunxiao skills install --dry-run          # preview
```

Alternatives:

```bash
# From a local checkout path
npx skills add /path/to/yunxiao-cli -y -g

# From GitHub (URL MUST end in .git)
npx skills add https://github.com/sliverTwo/yunxiao-cli.git -y -g
```

Then **restart / reload** the AI tool so it picks up skills.

Inspect without installing: `yunxiao skills list|path|read <name>`.



## Security / tenancy notes (A1)

Git history **≤ v0.14.5** may contain old tenant org/space/status-machine IDs in example profiles; the **current tree is placeholderized**. Residual history is assessed acceptable (no `filter-repo` rewrite).

## Pagination meta

List/read paths that go through `runRead` / `Do` + `MetaWithPagination` automatically attach `has_more` / `total` / `page` when the API returns pagination headers; endpoints without those headers are a no-op (no false `has_more`). Use `--all` on `pipeline list` / `codeup mrs list` (via `ListAll`) when Agents need the full set.

## Zhiyi / ZYPT / play profiles (optional)

**Topic / Risk workitem types:** org may define them, but each project must enable them in project settings UI. There is **no OpenAPI** to enable types; create fails with `工作项类型未启用！` until enabled in UI. CLI cannot enable Topic/Risk. Some types also reject `--sprint` (`未启用此字段【迭代】`) — omit sprint.

**play vs zhiyi:** `zhiyi` keeps the full Zhiyi/ZYPT field set; `play` is sandbox-accurate (minimal `bug_create_fields`, `bug_transition_required` only `{"100010":["80"]}`, sandbox bug statuses). Profiles may include `workitem_defaults` (per-`type_id` OpenAPI field defaults + `create_required`). Creates (`workitem create`, `+bug-create`) pull priority/trackers/测试负责人/验收负责人 from `workitem_defaults` unless overridden or `--no-defaults`. Use `yunxiao profile doctor` (read) to diff profile field/status ids vs live `fields` + `workflow` (also lists `workitem_defaults` type_ids).

For 智衣 tenant bug workflow: install profile (`yunxiao profile install-example zhiyi`), set `YUNXIAO_PROFILE=zhiyi`, use `workitem get ZYPT-…`, `sprint +current`, `workitem +bug-create`, `workitem +bug-transition`, `workitem +transition` (any type via `workflows`), `codeup mrs +create`. For sandbox regression: `--profile play` / `YUNXIAO_PROFILE=play` and `+bug-create` without module/env (or `--minimal`). Profiles are project-scoped (`space_id`); `workflows` is keyed by `type_id`. Relation types that work in sims: `ASSOCIATED`, `DEPEND_ON` (`RELATED`/`PARENT_SUB` often fail; Task parent via `--parent-id`). Codeup `--content-file` accepts absolute or cwd-relative paths. To refresh graphs when OpenAPI workflows omit transitions: `workitem +explore-workflow --type-id <id> --category <Req|Bug|Task> --cleanup --write-profile --yes` (sandbox first; never auto-run on ZYPT). That writes `workflows[<type-id>]` with **verified** `edges` plus `hinted_edges` for needs_fields (#61); for the profile `bug_type_id` it also updates legacy `bug_edges` / `bug_statuses` used by `+bug-transition`. Prefer `verified_edges` / `edges` over `hinted_edges` when consuming graphs. See skill `yunxiao-yunxiao-zhiyi-ops`. Do not hardcode Zhiyi status/field IDs into shared defaults.

## Source layout

| Path | Role |
|------|------|
| `cmd/` | Cobra commands (domain packages + root/auth/skills/api) |
| `internal/client` | HTTP client + API errors |
| `internal/config` | Env + config file resolution |
| `internal/output` | JSON success/error/dry-run envelopes |
| `internal/risk` | Risk levels + high-risk confirmation gate |
| `internal/schema` | `yunxiao schema` descriptors |
| `internal/version` | `Version` var (ldflags-injectable) |
| `internal/profile` | Tenant profiles (`~/.config/yunxiao/profiles`) |
| `internal/workflow` | Status-transition probe / graph helpers |
| `internal/zhiyi` | Bug transition BFS, create-bug body, sprint aggregate, MR helpers |
| `profiles/` | Example tenant profiles (`zhiyi.example.json`, `play.example.json`) |
| `skills/` | Companion agent skills (`yunxiao-*`) |
| `testdata/` | Fixtures |

## Tests & quality bar

```bash
go test ./...
# or
make test
```

- Prefer asserting **dry-run** / gate behavior over live API calls.
- Temp dirs are fine for filesystem helpers (e.g. skills install).
- Keep Conventional Commits; English messages OK (`feat:`, `fix:`, `docs:`).

## Do / don't

**Do**

- Match existing command patterns in `cmd/` (flags, `runMutating`, `handleErr`).
- Document `Risk: read|write|high-risk-write` on mutating commands.
- Bump `internal/version` when releasing user-visible CLI changes.
- Update README.md (English) and README.zh-CN.md (Chinese) and skills when adding domains.

**Don't**

- Commit secrets, tokens, or private org/repo IDs.
- Invent OpenAPI paths not confirmed in docs/MCP.
- Bypass high-risk confirmation for agents.
- Overwrite unrelated skills under `~/.agents/skills` (install only touches `yunxiao-*` names it installs).

## Quick agent checklist

1. Auth present? `yunxiao doctor` / `auth status`
2. Right layer? shortcut → typed → `api`
3. Params unknown? `yunxiao schema <id>`
4. Mutating? `--dry-run` first; high-risk → ask user → `--yes`
5. Parse stdout `ok == true`; read errors from stderr
