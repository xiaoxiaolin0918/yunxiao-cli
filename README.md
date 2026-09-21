Language: English | [中文](README.zh-CN.md)

# yunxiao-cli

Yunxiao (Alibaba Cloud DevOps) CLI redesigned like Feishu/Lark CLI: progressive discovery, `+shortcuts`, typed API commands, raw `api` escape hatch, risk gates, and agent skills.

CLI binary name: **`yunxiao`**.

## Human 30-second quickstart

1. Install (primary): open [https://github.com/xiaoxiaolin0918/yunxiao-cli/releases/latest](https://github.com/xiaoxiaolin0918/yunxiao-cli/releases/latest), download the archive for your OS, put `yunxiao` on PATH.
2. Optional npm thin wrapper: `npm i -g sanzhi-yunxiao-cli` (fetches GitHub Release binaries; use `./npm` locally if not published yet).
3. Login: `yunxiao auth login --browser` (CI: `--token`).
4. Smoke: `yunxiao whoami` · `yunxiao doctor` · `yunxiao codeup +open-mrs`.
5. Open console: `yunxiao browse pipeline --pipeline-id <id> --print-only`.
6. Completion: `yunxiao completion bash|zsh|powershell` (see [usage index](docs/wiki/01-usage/README.md)).

Writes: `--dry-run` first; high-risk needs `--yes` after confirmation.

More: [docs/wiki/01-usage/README.md](docs/wiki/01-usage/README.md) · migrate from gh/`yx`: [docs/wiki/00-process/gh-yx-migration.md](docs/wiki/00-process/gh-yx-migration.md)

---

## For AI agents

Paste the following into an AI agent (install → auth → skills → list projects read-only → user picks a project → **local-only** profile init):

```text
Install and init yunxiao CLI with a LOCAL profile:

1) Install from GitHub Releases (primary):
   https://github.com/xiaoxiaolin0918/yunxiao-cli/releases/latest
   Download the archive for the user's OS/arch, extract, put `yunxiao` on PATH.
   yunxiao --version   # should match GitHub latest Release

2) Auth — prefer browser OAuth; use PAT only for CI/headless (never print/paste raw tokens into chat):
   Recommended: yunxiao auth login --browser
   WARNING: OAuth consent = full account API capability (no module scopes; broader than fine-grained PAT).
   After login: yunxiao auth probe-oauth
   PAT fallback:
     Console: https://account-devops.aliyun.com/settings/personalAccessToken
     Help: https://help.aliyun.com/zh/yunxiao/user-guide/personal-access-token
     yunxiao auth login --token "<PAT>"
   yunxiao whoami && yunxiao doctor && yunxiao auth status

3) yunxiao skills install

4) Read-only: yunxiao project list — ask the user to pick a project/space_id

5) Init a LOCAL profile under ~/.config/yunxiao/profiles/:
   Prefer: yunxiao +onboard
   Or: yunxiao +onboard --space-id <id> --profile <name>

6) export YUNXIAO_PROFILE=<name> ; yunxiao profile show ; yunxiao profile doctor ; yunxiao doctor

7) Risk: --dry-run before writes; high-risk needs user confirm then --yes; long JSON via --data-file.
```

## Quick start

```bash
# 1) Install from GitHub Releases (primary)
#    https://github.com/xiaoxiaolin0918/yunxiao-cli/releases/latest
#    Download the archive for your OS/arch, extract, put `yunxiao` on PATH.
yunxiao --version   # should match GitHub latest Release

yunxiao auth login --browser    # or: yunxiao auth login --token "<PAT>"
yunxiao auth probe-oauth         # after browser login
yunxiao whoami && yunxiao doctor

yunxiao organization +whoami
yunxiao pipeline list
yunxiao codeup repos list
```

Start writes with `--dry-run`, confirm high-risk writes before adding `--yes`, and use `--data-file ./body.json` for long JSON. Install companion skills with `yunxiao skills install` (wizard supports multiple selections; or `--skill ...`).

## CLI vs MCP

| Aspect | `yunxiao-cli` | Yunxiao/Alibaba Cloud DevOps MCP |
|--------|---------------|----------------------------------|
| Form factor | Local command-line executable | MCP server exposing tools to an MCP client |
| Typical use | Scripts, CI, terminals, and copy-paste commands | Conversational workflows in an IDE or chat agent |
| Install | Download a release binary or build from source; install skills separately when needed | Configure the MCP server in an MCP-capable client |
| Auth | CLI profiles, environment variables, PAT, or browser OAuth | Credentials and authorization are managed through the MCP server/client setup |
| Discovery | `--help`, `schema`, typed commands, `+shortcuts`, and companion skills | Tool catalog and input schemas surfaced by the MCP client |
| Output | stdout/stderr, structured JSON, exit codes, and shell filters such as `--jq` | Structured tool results rendered by the client |
| Write safety | Explicit `--dry-run`; high-risk writes require `--yes` after confirmation | Depends on the tool and MCP client confirmation controls rather than universal CLI flags |
| Reproducibility | Commands can be copied, versioned, scripted, and audited | Calls depend more on client context and settings, so auditability and reproducibility are typically weaker |
| IDE dependency | None | Requires an MCP-capable IDE, agent, or other client |

Use the CLI for scripts, CI, and copy-paste commands; use MCP for chat in an IDE; many teams use both.

**Weekly quality / date-window reports** (date range → Req/Bug → JSON → scripts): prefer the typed CLI (`workitem search` with `--created-after` / `--created-before`, `--updated-*`, `--finish-*`, plus `--status` / `--status-stage`, `--all` to follow pages, and opt-in `--as-items` for `{items, pagination}` in `data`). Use MCP chat only as a fallback when you are already in an IDE agent. Filtering by `finishTime` via conditions may work; oapi SearchWorkitems / get responses omit `finishTime` (schemas list `gmtCreate` / `gmtModified` / `updateStatusAt`) — the CLI does **not** invent or enrich `finishTime` from `updateStatusAt`. OpenAPI `perPage` max is 200: use `meta.total` / `has_more` / `--all`, not `len(data)`. Server-side date conditions may still return out-of-window rows — client-filter on `gmtCreate` / `gmtModified` / `customFieldValues` as needed. Typed `workitem search` and raw `api POST …/workitems:search` are **read** (no `--yes`). Raw `api` bodies that pass MCP-like `createdAfter` / `updatedAfter` / `finishTimeAfter` (etc.) at top level are normalized into official `conditions` (see `meta.request`).

### MCP → CLI mapping (weekly / discovery)

| MCP-style intent | CLI |
|------------------|-----|
| `search_workitems` + `createdAfter` / `createdBefore` | `yunxiao workitem search --created-after … --created-before …` (+ `--all`, optional `--as-items`) |
| same for updated / finish windows | `--updated-after/before`, `--finish-after/before` |
| work item comments | `yunxiao workitem comments list --id <id>` |
| list orgs / projects (spaces) | `yunxiao organization list`, `yunxiao project list` |
| inspect search params | `yunxiao schema workitem.search` |
| `finishTime` on response | **Gap:** filter may work; oapi search/get responses usually omit `finishTime` |

### Agent / scripts on Windows

Prefer **Node or Python subprocess** (capture stdout as a Buffer/bytes, then `JSON.parse`) over PowerShell `>` redirects when consuming CLI JSON — redirects can alter encoding and break parsers. Use `yunxiao doctor` to print the resolved executable path and active profile (`organization_id`, `space_id`).

For AI agents, the CLI workflow is Agent paste followed by `yunxiao …`; the MCP workflow is tool-based. MCP reduces command memorization, but it often provides weaker auditability and reproducibility than the CLI.

## Install

**Recommended — [GitHub Releases](https://github.com/xiaoxiaolin0918/yunxiao-cli/releases/latest):**

Download the archive for your OS/arch, extract it, and add the `yunxiao` binary to `PATH`.

```bash
yunxiao --version   # should match GitHub latest Release
```

This project is maintained on **GitHub only** (`sliverTwo/yunxiao-cli`).

**From source (secondary):**

```bash
make build                 # produces ./yunxiao (injects Version via -ldflags)
# or (without ldflags, Version falls back to package default 0.16.13)
go build -o yunxiao .
# pin version explicitly:
# go build -ldflags "-X github.com/yunxiao-cli/yunxiao/internal/version.Version=<release>" -o yunxiao .
make install               # installs to ~/.local/bin/yunxiao
# or
go install github.com/yunxiao-cli/yunxiao@latest   # when published
```

Requires Go 1.24.4+. `make build` / `make ci` set `-ldflags -X …version.Version=$(VERSION)` (`VERSION` defaults to `git describe` or `0.16.13`).

**Known limitation:** `go install` / a lone binary does **not** ship the repo `skills/` tree, so `yunxiao skills list|read|install` will not find skills unless you run from a source checkout (or an installer that extracts `skills/`), or copy/`npx skills add` the tree. Prefer `make build` from a checkout, then `yunxiao skills install`, for skills-aware workflows.


## Update

Releases iterate quickly — use `yunxiao update` to upgrade. The CLI may also print a short **stderr** hint when a newer GitHub Release exists (at most one network check per 24h, cached under `~/.config/yunxiao/update_check.json`). Hints are skipped for `update` / `self-update` / `completion`, when `--format json` (the default), and when disabled via env. Check failures never block or fail your command; nothing is auto-downloaded.

The hint itself is printed in Chinese, for example: `发现新版本 yunxiao：0.16.6 → 0.16.13。运行：yunxiao update`.

**Binary (GitHub Releases) — recommended:**

```bash
yunxiao update --check     # report only; exit 2 if a newer release exists (CI/Agent-friendly)
yunxiao update             # TTY: confirm before replacing this binary
yunxiao update --yes       # non-interactive apply (scripts)
# alias: yunxiao self-update
```

`yunxiao update` downloads the matching platform archive from [GitHub Releases](https://github.com/xiaoxiaolin0918/yunxiao-cli/releases) (`yunxiao-cli-<ver>-<os>-<arch>.tar.gz|.zip`), verifies SHA-256 when `checksums.txt` is present, and replaces the running binary (write-beside then rename; on Windows you may need to restart the process if `--version` still shows the old build). Gate: `--dry-run` / `--check` never writes; applying requires TTY confirm or `--yes`.

Optional doctor probe (still opt-in; no download):

```bash
yunxiao doctor --check-update
```

Disable opportunistic hints and doctor `--check-update`:

```bash
export YUNXIAO_UPDATE_CHECK=0   # also: false | off | no
```

**npm installer (`sanzhi-yunxiao-cli`):**

```bash
npm install -g sanzhi-yunxiao-cli@latest
# or re-run the postinstall fetcher after bumping the package
```

Same env overrides as the installer: `YUNXIAO_CLI_GITHUB_REPO`, `YUNXIAO_CLI_DOWNLOAD_BASE`.

## Auth

1. Create a Personal Access Token in the Yunxiao console (primary):  
   https://account-devops.aliyun.com/settings/personalAccessToken  
   Help: https://help.aliyun.com/zh/yunxiao/user-guide/personal-access-token

   Recommended module checkboxes for this CLI: Organization/members **read**; Projex/Codeup/Flow **read+write** (or read-only if preferred); Packages/Testhub/AppStack as needed. Token name tip: `yunxiao-cli`.
2. Prefer env (CI / shells):

```bash
export YUNXIAO_ACCESS_TOKEN="<PAT>"
# optional
export YUNXIAO_ORGANIZATION_ID="<orgId>"
export YUNXIAO_API_BASE_URL="https://openapi-rdc.aliyuncs.com"   # default
export YUNXIAO_EDITION="central"   # or region
```

Or store in config (`~/.config/yunxiao/config.json`, mode 0600):

```bash
yunxiao auth login --token "<PAT>"
yunxiao auth status
yunxiao whoami
yunxiao doctor
```

Token precedence (highest first): `YUNXIAO_ACCESS_TOKEN` env → `~/.config/yunxiao/credentials.json` (last successful `auth login`, browser or token) → active profile `access_token` → legacy `config.json`. OAuth tokens live only in `credentials.json` (mode 0600), not in profile JSON. `yunxiao auth status` reports `token_source` / `token_kind` (`pat`|`oauth`) without printing raw tokens.
**Browser OAuth:** `yunxiao auth login --browser` (authorization code + PKCE + DCR via `/.well-known/oauth-authorization-server`). Consent = **full account API capability** (platform has no module scopes). CI/headless: keep `--token` / env. Probe gate: `yunxiao auth probe-oauth`. Does **not** use legacy `CreateOAuthToken`.


## Agent quickstart

```text
Browse:     yunxiao <domain> --help
Inspect:    yunxiao schema <id>          # e.g. codeup.mrs.create
Prefer:     +shortcuts over typed over raw api
Risk:       read | write | high-risk-write
            high-risk-write needs --yes after user confirms
Preview:    --dry-run   Filter: --jq '...'
```

## Agent Skills

Companion skills live under `skills/yunxiao-*` (each has `SKILL.md`):

| Skill | Use for |
|-------|---------|
| `yunxiao-shared` | Auth, config, doctor, JSON contract, `--dry-run` / `--yes` |
| `yunxiao-organization` | Orgs, members, departments, roles |
| `yunxiao-project` | Projex projects & work items |
| `yunxiao-codeup` | Repos, branches, files, MRs |
| `yunxiao-pipeline` | Flow pipelines, runs, jobs, YAML |
| `yunxiao-packages` | Artifact repositories & artifacts |
| `yunxiao-testhub` | Test plans, results, plan comments |
| `yunxiao-appstack` | Apps, change-orders, orchestrations, tags, variable groups |
| `yunxiao-zhiyi-ops` | Zhiyi/ZYPT sprint/bug-create/transition/MR + tenant profile (optional) |

**Install** (so AI tools can discover them; default dir `~/.agents/skills`):

```bash
# 1) Recommended — local CLI install (copy into ~/.agents/skills)
yunxiao skills install
yunxiao skills install --skill yunxiao-shared --skill yunxiao-codeup
yunxiao skills install --dir /custom/skills --dry-run
yunxiao skills install --symlink --force

# 2) Via skills CLI from a local checkout
npx skills add /path/to/yunxiao-cli -y -g

# 3) From GitHub (URL must end in .git)
npx skills add https://github.com/sliverTwo/yunxiao-cli.git -y -g
```

Then restart / reload your AI tool so skills are picked up.

Inspect without installing:

```bash
yunxiao skills list
yunxiao skills path
yunxiao skills read yunxiao-shared
```

Contributors and AI agents editing this repo: see **[AGENTS.md](AGENTS.md)**.

## Examples by domain

Long JSON bodies: prefer `--data-file path.json` or `--data @path.json` (avoids shell quoting limits).


```bash
# organization
yunxiao organization +whoami
yunxiao organization list
yunxiao organization members search --query alice

# project / work items
yunxiao project list --name demo
yunxiao project +my-open-items
yunxiao project +created-by-me --status-stage 1,2
yunxiao workitem search --assigned-to self --category Req --priority <id>
yunxiao workitem search --category Req --created-after "2026-09-01 00:00:00" --created-before "2026-09-07 23:59:59"
yunxiao workitem search --category Bug --finish-after "2026-09-01 00:00:00" --finish-before "2026-09-07 23:59:59"
yunxiao workitem get --id <id>
yunxiao workitem comments list --id <id>
yunxiao workitem comment --id <id> --content "note" --dry-run
yunxiao workitem create --space-id <sid> --type-id <tid> --subject "title" --assigned-to self --dry-run
yunxiao workitem update --id <id> --assigned-to self --dry-run
yunxiao workitem +transition --id <id|serial> --to <alias|statusId> --dry-run

# codeup
yunxiao codeup repos list
yunxiao codeup branches list --repo <repoId>
yunxiao codeup tags list --repo <repoId>
yunxiao codeup tags create --repo <repoId> --tag-name v1.0 --ref master --dry-run
yunxiao codeup protected-branches list --repo <repoId>
yunxiao codeup protected-branches create --repo <repoId> --branch master --allow-push-roles 40,30 --dry-run
yunxiao codeup files tree --repo <repoId> --ref master
yunxiao codeup commits list --repo <repoId> --ref master
yunxiao codeup files create --repo <id> --path a.txt --branch master --message "add" --content "hi" --dry-run
yunxiao codeup files delete --repo <id> --path a.txt --branch master --message "rm" --dry-run
yunxiao codeup mrs merge --repo <id> --local-id 1 --merge-type no-fast-forward --dry-run
yunxiao codeup mrs close --repo <id> --local-id 1 --dry-run
yunxiao codeup mrs review --repo <id> --local-id 1 --opinion PASS --dry-run

yunxiao codeup mrs get --repo <id> --local-id 1
yunxiao codeup mrs diffs --repo <id> --local-id 1
yunxiao codeup mrs comments list --repo <id> --local-id 1
yunxiao codeup mrs comments create --repo <id> --local-id 1 --content "LGTM" --patchset-biz-id <biz> --dry-run
yunxiao codeup mrs labels list --repo <id> --local-id 1
yunxiao codeup mrs labels attach --repo <id> --local-id 1 --label-ids 1,2 --dry-run
yunxiao codeup mrs reopen --repo <id> --local-id 1 --dry-run
yunxiao codeup compare --repo <id> --from master --to feature
yunxiao pipeline job retry --pipeline-id <id> --run-id <r> --job-id <j> --dry-run
yunxiao pipeline job pass --pipeline-id <id> --run-id <r> --job-id <j> --dry-run
yunxiao pipeline job refuse --pipeline-id <id> --run-id <r> --job-id <j> --dry-run
yunxiao packages artifacts delete --repo-id <id> --repo-type GENERIC --id <aid> --dry-run
yunxiao workitem types list --space-id <sid> --category Req
yunxiao workitem create --space-id <sid> --type-id <tid> --subject "t" --assigned-to self --custom-fields '{"fid":"v"}' --dry-run
yunxiao workitem relations list --id <id> --relation-type ASSOCIATED
yunxiao workitem relations create --id <id> --related-id <rid> --relation-type ASSOCIATED --dry-run
yunxiao workitem delete --id <id> --dry-run
yunxiao testhub results update --plan-id <p> --testcase-id <t> --status PASSED --dry-run
yunxiao testhub plan-comments list --plan-id <p> --testcase-id <t>
yunxiao appstack change-orders job-logs --app my-app --sn <sn> --job-sn <jsn>
yunxiao appstack orchestrations list --app my-app
yunxiao appstack change-orders create --app my-app --data '{...}' --dry-run
yunxiao appstack change-orders create --app my-app --data-file order.json --dry-run
yunxiao codeup +open-mrs
yunxiao codeup mrs create --repo <id> --source feat --target master --title "x" --dry-run
yunxiao codeup mrs create --repo <id> --source feat --target master --title "x" --yes   # after user OK

# pipeline
yunxiao pipeline list
yunxiao pipeline +status --pipeline-id <id>
yunxiao pipeline run list --pipeline-id <id>
yunxiao pipeline run latest --pipeline-id <id>
yunxiao pipeline +failed --pipeline-id <id>
yunxiao pipeline job log --pipeline-id <id> --run-id <rid> --job-id <jid>
yunxiao pipeline run trigger --pipeline-id <id> --branch master --dry-run
yunxiao pipeline run cancel --pipeline-id <id> --run-id <rid> --dry-run

# packages (upload skipped — see Known gaps)
yunxiao packages repos list
yunxiao packages artifacts list --repo-id <id> --repo-type GENERIC

# testhub / appstack
yunxiao testhub plans list --project-id <id>
yunxiao testhub plans progress --plan-id <id>
yunxiao appstack apps list
yunxiao appstack change-orders versions --app my-app
yunxiao appstack change-orders job-logs --app my-app --sn <sn> --job-sn <jsn>
yunxiao appstack orchestrations list --app my-app


# v0.7
yunxiao pipeline get --id <id>
yunxiao pipeline create --name ci --file ./pipeline.yaml --dry-run
yunxiao pipeline update --id <id> --name ci --file ./pipeline.yaml --dry-run
yunxiao workitem attachments list --id <id>
yunxiao workitem attachments create --id <id> --file ./shot.png --dry-run
yunxiao appstack tags search --search demo
yunxiao appstack tags create --name t --color "#4676e5" --dry-run
yunxiao appstack tags bind --app my-app --tag-names t --dry-run
yunxiao appstack variable-groups list --app my-app
yunxiao appstack variable-groups revision --app my-app

# v0.8
yunxiao organization departments list
yunxiao organization roles list
yunxiao project get --id <id>
yunxiao sprint list --space-id <id>
yunxiao versions list --space-id <id>
yunxiao workitem fields --space-id <s> --type-id <t>
yunxiao pipeline service-connections list --type codeup
yunxiao pipeline host-groups list
yunxiao pipeline flow-variable-groups list
yunxiao codeup repos get --repo <id>
yunxiao codeup branches create --repo <id> --branch feat --ref master --dry-run
yunxiao appstack apps create --name demo --dry-run
yunxiao appstack change-requests list --app my-app
yunxiao appstack global-vars list
yunxiao testhub cases search --repo-id <id>
yunxiao testhub directories create --repo-id <id> --name folder --dry-run

# v0.9
yunxiao appstack release-workflows list --app my-app
yunxiao appstack release-workflows stage execute --app a --workflow-sn w --stage-sn s --dry-run
yunxiao appstack deploy machine-log --tunnel-id 1 --machine-sn sn
yunxiao appstack deploy add-hosts --instance n --host-sns a,b --dry-run
yunxiao pipeline vm-deploy get --pipeline-id p --deploy-id d
yunxiao pipeline vm-deploy stop --pipeline-id p --deploy-id d --dry-run
yunxiao pipeline resource-members create --resource-type pipeline --resource-id id --role-name viewer --user-id u --dry-run
yunxiao workitem efforts list --id <id>
yunxiao workitem efforts mine --start-date 2026-01-01 --end-date 2026-01-31
yunxiao workitem estimated-efforts create --id <id> --owner self --spent-time 4 --dry-run
yunxiao programs search --name demo
yunxiao codeup repos create --name my-repo --path my-repo --dry-run
# escape hatch
yunxiao api GET /oapi/v1/platform/user
yunxiao schema
```


## Profiles: play vs zhiyi (optional)
`yunxiao +onboard` creates a generic local profile from a chosen `space_id` under `~/.config/yunxiao/profiles/`.


Tenant-specific Projex constants live in a **profile JSON**, not hardcoded CLI defaults.
Profiles are **project-scoped** (`space_id`); discovered workitem graphs live under `workflows` keyed by **`type_id`**.
`workitem_defaults` (keyed by **`type_id`**) stores OpenAPI field defaults + create-required ids for create payloads; `workitem create` and `+bug-create` apply those field defaults (priority/trackers/QA-owner/acceptance-owner, …) unless overridden by flags / `--custom-fields` or `--no-defaults`. `yunxiao profile doctor` reports which types have them and checks those field ids against live fields.

| Profile | Purpose |
|---------|---------|
| **zhiyi** | Full Zhiyi/ZYPT field set (`module` / `environment` / `ExpCompletionTime` + rich `bug_transition_required`) |
| **play** | Sandbox/YXCLI regression — minimal `bug_create_fields` (priority + seriousLevel only); `bug_transition_required` = `{"100010":["80"]}` only; sandbox bug statuses |

```bash
yunxiao profile install-example zhiyi   # or: play
export YUNXIAO_PROFILE=zhiyi            # or play
yunxiao profile show
yunxiao profile doctor                 # diff profile vs live fields/workflow (read)
yunxiao workitem get ZYPT-5768         # zhiyi serials; play uses YXCLI-…
yunxiao sprint +current --dry-run
# Zhiyi full create:
yunxiao workitem +bug-create --title "title" --description "description" \
  --expected-completion 2026-09-20 --sprint <id> --dry-run
# Sandbox / non-Zhiyi (omit module/env/ExpCompletionTime):
yunxiao workitem +bug-create --profile play --title "title" --description "description" \
  --sprint <id> --dry-run
yunxiao workitem +bug-create --minimal --title "…" --description "…" --sprint <id> --dry-run
yunxiao workitem +bug-transition --id ZYPT-5768 --to processing \
  --plan-due-date 2026-09-20 --developer <uid> --dry-run
yunxiao workitem +explore-workflow --type-id <bug_type_id> --cleanup --dry-run
yunxiao workitem relations create --id <id> --related-id <rid> --relation-type ASSOCIATED --dry-run
# Codeup --content-file accepts cwd-relative or absolute paths
yunxiao codeup files update --repo sandbox --path README.md --branch x \
  --message "…" --content-file /tmp/note.md --dry-run
yunxiao codeup mrs +create --repo iipmes_gy --source feat/x \
  --title "fix" --work-item ZYPT-5768 --wip --dry-run
```

See skill `yunxiao-zhiyi-ops`, `profiles/zhiyi.example.json`, and `profiles/play.example.json`.

## Risk / dry-run / --yes

| Level | Rule |
|-------|------|
| read | Safe to run |
| write | Confirm intent; use `--dry-run` when available |
| high-risk-write | Exit **10** + `confirmation_required` without `--yes`. Ask the user; only then append `--yes`. Never auto-confirm. |

## Build & test

```bash
make test
make build
./yunxiao --help
```


## Known gaps

Surfaces intentionally **not** wrapped (use `yunxiao api` when you have a confirmed OpenAPI path):

| Gap | Reason |
|-----|--------|
| Packages **upload** / repo create-delete | Not clear in MCP `operations/packages` / no OpenAPI for upload |
| Codeup **blame**, **cherry-pick** | No solid OpenAPI confirmed — do not invent |
| Projex **Topic / Risk** type enable on a project | Org may define types; project must enable them in **project settings UI**. Create returns `工作项类型未启用！` (work item type not enabled); no OpenAPI to enable — CLI cannot enable Topic/Risk |
| Topic / Risk **sprint** field binding | Some types return `未启用此字段【迭代】` (sprint field not enabled) — omit `--sprint` (CLI surfaces a hint) |
| Relation types | Working: `ASSOCIATED`, `DEPEND_ON`. `RELATED` / `PARENT_SUB` often fail type constraints; Task parent via `--parent-id` on create |
| MR label **detach** | No OpenAPI in MCP |
| AppStack full CR lifecycle beyond list/create surfaces already shipped | Expand only when MCP is unambiguous |
| Flow structured pipeline YAML generator (`createPipelineWithOptions`) | MCP helper only; CLI takes raw YAML `--file` |

v0.9 landed deferred clears: AppStack release-workflows + deploy host mutations, Flow VM deploy orders, Projex efforts/programs, Flow resource-member writes, Codeup `repos create` (high-risk).

## Development

```bash
make test && make build
make ci                 # go build -ldflags … ./... && go test ./... && go vet ./...
./scripts/ci.sh         # same checks, POSIX (local / any CI runner)
```

CI/CD is **GitHub Actions only** (this repo is maintained on GitHub, not mirrored to Codeup Flow):

- `.github/workflows/ci.yml` — CI on pushes to `main` and pull requests
- `.github/workflows/release.yml` — build platform archives and publish a GitHub Release when a `v*` tag is pushed

See [AGENTS.md](AGENTS.md) for contributor / AI-agent conventions.

## Changelog

- **0.16.15** — bug-create alias validation; mrs get `--brief` + status→state; mrs update title/description; create brief output (#45–#46, #48–#51)
- **0.16.14** — `yunxiao alias` (no embedded `--yes`); command-docs CI check; npm/OIDC publish eval (#38–#40)
- **0.16.13** — `yunxiao browse` (pipeline/workitem/mr/repo/url, `--print-only`); human 30s README; usage index + gh/yx migration; completion docs (#34–#37)
- **0.16.12** — Codeup MR workItemIds as OpenAPI string + fail if link missing
- **0.16.11** — +pending --all-pipelines soft-fail meta; update --validate noop only when full YAML+name match; --check (#29/#30)
- **0.16.10** — pipeline queue observability: +queue, runner-groups, run meta.queue, 403 hints (#23)
- **0.16.9** — pipeline change safety: get --yaml, diff, update --validate, 1209300 details (#21)
- **0.16.8** — Codeup MR workItemIds precheck + list ignored-param WARNING (#24)
- **0.16.7** — Manual gate loop: `pipeline +pending`, `run watch`, `job pass|refuse --yes` / `+approve|+refuse` (#22)
- **0.16.6** — organization members list|search --include-aliyun-uid for ManualValidate Aliyun UIDs (#19)
- **0.16.5** — yunxiao codeup mrs reviewers add for existing MRs (#18)
- **0.16.4** — Chinese `update --help` + yunxiao-shared skill self-update (#15); `codeup mrs create --reviewer` (Fixes #16) + `+create` reviewerUserIds fix (#17)
- **0.16.3** — opportunistic update hint on CLI use (Chinese stderr, 24h cache, `YUNXIAO_UPDATE_CHECK=0`)
- **0.16.2** — workitem search date filters + `--all` + read-only `:search`; weekly followups (`--as-items`, doctor path, MCP mapping); `yunxiao update` / `self-update`
- **0.16.1** — Smoke fixes: `appstack apps list` sends required `pagination=keyset`; `workitem search` / `project +my-open-items` use profile `space_id` or clear CLI error; friendlier hint when `programs search` is blocked on non-Advanced orgs
- **0.16.0** — Browser OAuth (`auth login --browser` / `--dry-run`), `credentials.json` (0600), `auth probe-oauth` (O1 header gate), auto refresh for `token_kind=oauth`, For AI agents section prefers browser OAuth; PAT `--token` kept for CI
- **0.15.7** — `yunxiao +onboard` writes a **generic local** profile under `~/.config/yunxiao/profiles/` (TTY project pick or `--space-id`); README For AI agents prompts (EN + zh-CN); missing-token hints include PAT console URL + module permission checklist
- **0.15.6** — default newest-first for comment/activity/history-style lists (`--sort asc|desc`; invalid values rejected); comments sort by **create** time; activity/MR/runs/efforts prefer update/modified; client-side `--sort` is **page-local** when the list is paginated (`--all` sorts across collected pages)
- **0.15.5** — `--data-file` and `--data @file.json` for long JSON payloads (`api`, appstack, testhub, …)
- **0.15.2** — companion skills refresh for CLI 0.15.x (`has_more` / `meta.url` / `refresh_ok`); `client.ListAll` + `--all` on `pipeline list` & `codeup mrs list`; `scripts/flow-ci.sh` (Alibaba golang mirror + `GOPROXY=goproxy.cn`)
- **0.15.1** — B5 wave2: more cmds on `runRead`/`runJSONMutating` (workitem update/relations list; codeup writes; pipeline mutations + remaining reads; org/project/sprint/versions/packages/testhub/appstack/effort/programs reads + simple writes). Still custom: multipart attachments, cancel-reason soft-warn dry-run envelope, pipeline create/update YAML redaction preview, multi-step shortcuts (+transition/+bug*/+explore-workflow, MR create, testhub results fallback, sprint +bugs aggregate)
- **0.15.0** — structural: B5 `runRead`/`runJSONMutating` cmd helpers (partial migration); C1 split `workitem.go`; C2 precompiled date regex; C3 `Do` returns headers (lists use `Do`+`MetaWithPagination`); C4 ldflags Version injection
- **0.14.11** — pipeline/run responses include Flow console `url` (`meta.url`; list items) via `https://flow.aliyun.com/pipelines/{id}` and `.../builds/{runId}`
- **0.14.10** — A1: document accept of git-history residual (≤v0.14.5 example IDs); D1: Retry-After sleep capped at 30s; C5: README duplicate EN examples cleaned; note `go install` vs skills-tree discovery
- **0.14.9** — B1: HTTP client `context.Context` + GET/HEAD retry (429/5xx/network, Retry-After); P2: `has_more` via total/page/per_page; more lists use MetaWithPagination
- **0.14.8** — cobra Execute→exit 10 E2E; `refreshAfterTransition` + warning/`refresh_ok` unit tests; list `meta.has_more`/`total`/`page` via MetaWithPagination (MR list wired)
- **0.14.7** — help/skills sanitize real IDs to placeholders; B3 Write/gate contract tests + PostMultipart httptest; transition `refresh_ok` in success JSON
- **0.14.6** — sanitize example profiles (placeholders only); `go mod tidy`; `make ci` / `scripts/ci.sh`; transition refresh-fail stderr warning
- **0.14.4** — workitem/MR responses include clickable `url` (`meta.url`; list items get `url`); builders in `internal/zhiyi`
- **0.14.3** — optional per-profile `access_token`; token precedence env > profile > config; `auth status` / doctor report `token_source`
- **0.14.2** — `workitem create` / `+bug-create` apply profile `workitem_defaults` (priority/trackers/QA-owner/acceptance-owner) unless overridden or `--no-defaults`
- **0.14.1** — `workitem_defaults` in profiles (per-`type_id` field defaults + create_required); `profile doctor` reports/verifies them
- **0.14.0** — Sandbox-accurate `play` profile; `+bug-create --minimal` / omit disabled fields; `profile doctor`; relation-type docs (`ASSOCIATED`/`DEPEND_ON`); sprint/field-not-enabled hints; `--content-file` absolute paths
- **0.13.1** — Codeup `--repo` alias resolution for branches/files/commits/compare/mrs/repos (reuse `resolveCodeupRepo`)
- **0.13.0** — `workitem +transition` (any type via `workflows`); Codeup `tags` + `protected-branches`; Topic/Risk enable is UI-only
- **0.12.1** — profiles store per-`type_id` `workflows`; `--write-profile` fills that map (legacy `bug_*` kept for Bug)
- **0.12.0** — `workitem +explore-workflow` auto-discovers status transition graphs; profile `--write-profile` merge
- **0.11.0** — Zhiyi `sprint +current`, `workitem +bug-create`, `codeup mrs +create`; profile repos/create-fields
- **0.10.0** — Zhiyi tenant profile; ZYPT workitem get; `workitem +bug-transition`; skill `yunxiao-zhiyi-ops`
- **0.9.1** — `yunxiao skills install`; AGENTS.md; README skills install docs
- **0.9.0** — AppStack RW + deploy; Flow vm-deploy + resource-members write; efforts/programs; repos create
- **0.8.0** — org dept/roles; sprint/versions; Flow SC/HG/VG/RM list; codeup repos/branches; AppStack apps/CR/global-vars; testhub cases
- **0.7.0** — pipeline YAML get/create/update; AppStack tags + variable-groups; workitem attachments
- **0.6.0** — MR comments/labels; pipeline pass/refuse; testhub results; workitem relations; AppStack job-logs

## License

MIT — see [LICENSE](LICENSE).
