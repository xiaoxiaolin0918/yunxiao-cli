---
name: yunxiao-codeup
version: 1.1.5
description: "云效 Codeup：列仓库/分支/MR、评论/标签/评审人、创建/合并/关闭合并请求。用户问代码库、分支、MR 时使用。创建/合并等为 high-risk-write。"
metadata:
  requires:
    bins: ["yunxiao"]
  cliHelp: "yunxiao codeup --help"
---

# codeup

开始前先读 [`../yunxiao-shared/SKILL.md`](../yunxiao-shared/SKILL.md)。

> List `meta` 可能含 `has_more`；`codeup mrs list --all` 可跟页（ListAll，上限 50）。

## Shortcuts（优先）

| Shortcut | 说明 | Risk |
|----------|------|------|
| `+open-mrs` | 列出 opened 合并请求 | read |

```bash
yunxiao codeup +open-mrs
yunxiao codeup +open-mrs --repo <numericRepoId>
```


## MR `url`（CLI 0.15.x）

`codeup mrs list` / `+open-mrs` 会为每条 MR 注入可点击 `url`（优先 API `detailUrl`，否则拼控制台链接）。`mrs get` / `create` / `+create` / `update` 写入 `meta.url`。`get` 将 OpenAPI `status` 同步为脚本友好的 `state`；`--brief` 只出 localId/title/status/state/url。`create` / `+create` / `update` **默认 brief 摘要**（破坏性：依赖完整 MR JSON 的脚本请加 `--full`）。

```bash
yunxiao codeup mrs list --state opened
yunxiao codeup mrs list --state opened --all
yunxiao codeup +open-mrs
```

## Typed commands

```bash
yunxiao codeup repos list --search demo
yunxiao codeup branches list --repo <repoId>
yunxiao codeup files tree --repo <repoId> --ref master
yunxiao codeup files get --repo <repoId> --path README.md --ref master
yunxiao codeup files create --repo <repoId> --path docs/a.md --branch master --message "add" --content "x" --dry-run
yunxiao codeup files update --repo <repoId> --path docs/a.md --branch master --message "upd" --content-file ./a.md --dry-run
yunxiao codeup commits list --repo <repoId> --ref master
yunxiao codeup mrs list --state opened
yunxiao schema codeup.mrs.create
```

## 创建 MR（high-risk-write）

**MUST**：先 `--dry-run` → 向用户确认 → 用户同意后再加 `--yes`。

```bash
yunxiao codeup mrs create \
  --repo <repoId> --source feature/x --target master \
  --title "feat: x" --description "..." --reviewer <userId1,userId2> --dry-run

# 用户明确同意后：
yunxiao codeup mrs create \
  --repo <repoId> --source feature/x --target master \
  --title "feat: x" --reviewer <userId1,userId2> --yes
```

`--repo` 可为数字 id，或 `org/repo`（会编码）；非数字时 CLI 会尝试拉取仓库解析 `sourceProjectId`/`targetProjectId`。

`--reviewer`：逗号分隔 userId，写入 OpenAPI `reviewerUserIds`；与 `mrs +create --reviewer` 语义一致。

缺 `--yes` → exit **10** + `confirmation_required`（见 shared skill）。


## MR 评论与标签

```bash
yunxiao codeup mrs comments list --repo <id> --local-id 1   # newest first; --sort asc
yunxiao codeup mrs comments create --repo <id> --local-id 1 --content "LGTM" --patchset-biz-id <biz> --dry-run
yunxiao codeup mrs comments resolve --repo <id> --local-id 1 --comment-biz-id <biz> --dry-run
yunxiao codeup mrs comments reopen --repo <id> --local-id 1 --comment-biz-id <biz> --dry-run
# --comment-biz-id 取自 comments list 每条的 comment_biz_id（JSON 里也可能是 commentBizId）
yunxiao codeup mrs labels list --repo <id> --local-id 1
yunxiao codeup mrs labels attach --repo <id> --local-id 1 --label-ids 1,2 --dry-run
yunxiao codeup mrs reviewers add --repo <id> --local-id 1 --reviewer <userId1,userId2> --dry-run
```

`comments create` / `comments resolve` / `comments reopen` / `labels attach` / `reviewers add` 为 **write**（`--dry-run` 可预览）。`patchset-biz-id` 可从 `mrs diffs` 取得。

`reviewers add`：逗号分隔 userId → OpenAPI `POST …/person/REVIEWER` body `userIds`（与 create 的 `reviewerUserIds` 字段名不同；CLI `--reviewer` 语义一致）。

> **Label detach**：公开 OpenAPI / MCP 仅有 Get + Attach，无 Detach/Delete labels；CLI 不封装。

## 不负责

工作项 → `yunxiao-project`；流水线 → `yunxiao-pipeline`。

## 文件写操作

`files create` / `files update` / `files delete` 为 **high-risk-write**：先 `--dry-run`，用户确认后再 `--yes`。

## MR 写操作

### high-risk-write（需 `--dry-run` → 确认 → `--yes`）

```bash
yunxiao codeup mrs review --repo <id> --local-id 1 --opinion PASS --dry-run
yunxiao codeup mrs merge --repo <id> --local-id 1 --merge-type no-fast-forward --dry-run
yunxiao codeup mrs close --repo <id> --local-id 1 --dry-run
yunxiao codeup mrs reopen --repo <id> --local-id 1 --dry-run
```

均为 **high-risk-write**（尤其 merge 会改写目标分支）。

### write（`--dry-run` 即可预览；非 high-risk，一般不需 `--yes`）

```bash
yunxiao codeup mrs update --repo <id> --local-id 1 --title "WIP: docs" --dry-run
yunxiao codeup mrs update --repo <id> --local-id 1 --work-item ZYPT-5573 --dry-run
yunxiao codeup mrs link --repo <id> --local-id 1 --work-item ZYPT-5573 --dry-run
yunxiao codeup mrs unlink --repo <id> --local-id 1 --work-item ZYPT-5573 --dry-run
```

`update` / `link` / `unlink` 为 **write**（标题/描述或工作项关联变更），不是 high-risk-write。

### read

```bash
yunxiao codeup mrs get --repo <id> --local-id 1 --brief
yunxiao codeup mrs diffs --repo <id> --local-id 1
yunxiao codeup compare --repo <id> --from master --to feature
```

## 分支

```bash
yunxiao codeup repos get --repo <id>
yunxiao codeup branches get --repo <id> --branch master
yunxiao codeup branches create --repo <id> --branch feat --ref master --dry-run
yunxiao codeup branches delete --repo <id> --branch feat --dry-run
```

create/delete 为 **high-risk-write**。

## Tags / protected branches (v0.13)

```bash
yunxiao codeup tags list --repo <id|alias>
yunxiao codeup tags create --repo <id> --tag-name v1.0 --ref master --dry-run
yunxiao codeup tags delete --repo <id> --tag-name v1.0 --yes

yunxiao codeup protected-branches list --repo <id|alias>
yunxiao codeup protected-branches get --repo <id> --id <ruleId>
yunxiao codeup protected-branches create --repo <id> --branch master \
  --allow-push-roles 40,30 --allow-merge-roles 40,30 --dry-run
yunxiao codeup protected-branches delete --repo <id> --id <ruleId> --yes
```

`--repo` 支持 profile.repositories 别名。tags/protect create|delete 为 **high-risk-write**。

## Create repository (v0.9, high-risk-write)

```bash
yunxiao codeup repos create --name my-repo --path my-repo --visibility private --dry-run
# 用户确认后加 --yes
```

来源：`createRepositoryFunc`（query `createParentPath=true`）。

## Known gaps

- Blame / cherry-pick：无扎实 OpenAPI，勿臆造；需要时用 `yunxiao api`。
- MR label detach：无 OpenAPI。

## Merge requests

### Silent traps (#24 / #32)
- Wiki: [codeup-mr-silent-traps.md](../../docs/wiki/02-domains/codeup-mr-silent-traps.md)
- Pass --work-item on mrs create / mrs +create: CLI GETs each id before create.
- From 0.16.12: body sends comma-separated workItemIds string; after create verifies/repairs via extRelationRecords; **fails (ok=false)** if links still missing (see "MR work-item link" below). Older releases only warned.
- Prefer --repo / --state on mrs list (server may ignore repositoryId / status).


## MR work-item link (0.16.12+)

- --work-item on mrs create / mrs +create: body sends comma-separated workItemIds string.
- After create: verify via workitem extRelationRecords (category codeupMergeRequest); repair once if missing; **fail** if still missing.
- Remediation when failed (0.16.17+): `yunxiao codeup mrs link --repo <id> --local-id <n> --work-item <id|serial>` (or `mrs update --work-item`); older: close/recreate or Codeup web UI.

## Link / unlink existing MR (0.16.17+ / #54)

- Detail: [codeup-mr-silent-traps.md](../../docs/wiki/02-domains/codeup-mr-silent-traps.md)
- Push-created MRs cannot carry workItemIds; use `mrs link` / `mrs unlink` / `mrs update --work-item`.
- API: Projex extRelationRecords create/list/delete (`category=codeupMergeRequest`); **not** UpdateChangeRequest.
- Idempotent; prefer `--dry-run` first.

## Browse

- `yunxiao browse mr --repo-url <web> --local-id <n> --print-only`
- `yunxiao browse repo --repo-url <web> --print-only`
