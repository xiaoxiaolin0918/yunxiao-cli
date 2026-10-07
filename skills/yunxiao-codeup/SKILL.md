---
name: yunxiao-codeup
version: 1.2.0
description: "云效 Codeup：列仓库/分支/MR、评论/标签/评审人、创建/合并/关闭合并请求、推送评审状态感知。用户问代码库、分支、MR 时使用。创建/合并等为 high-risk-write。"
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
| `+open-mrs` | 列出 opened 合并请求（每条注入 `status` / `wip`） | read |
| `mrs +push-review-status` | 某仓 open MR 的推送评审状态（status/wip/ahead/behind/mergeable/评审） | read |

```bash
yunxiao codeup +open-mrs
yunxiao codeup +open-mrs --repo <numericRepoId>
yunxiao codeup mrs +push-review-status --repo <repoId>
yunxiao codeup mrs +push-review-status --repo <repoId> --local-id 139   # 单查一条
```

## 推送评审 WIP / 状态感知（#124 / #132）

推送评审模式 push 自动建的 MR 初始状态为「开发中」（服务端状态 `UNDER_DEV`，非标题前缀）。
此时 `mrs merge` 会 405 `SYSTEM_FORBIDDEN_ERROR`（评审 PASS 也无效）。**没有 OpenAPI 能取消
WIP**（`UpdateChangeRequest` 仅 title/description）——需网页操作：MR 页「…」→ **取消 WIP**
（转 `TO_BE_MERGED` 待合并），再 merge。

- `mrs list` / `+open-mrs` 每条注入 `status`（`newVersionState` 优先：UNDER_DEV/
  UNDER_REVIEW/TO_BE_MERGED/CLOSED/MERGED）与 `wip`；`mrs get`（含 `--brief`）也带 `wip`。
- `mrs list --status UNDER_DEV`：**客户端过滤**（服务端可能忽略 status 参数），配 `--all` 才是全量。
- `mrs merge` 失败时错误自动带 `error.details.mr`（status/wip/todo…）+ hint（UNDER_DEV → 网页
  取消 WIP + 重试命令；`WIP: ` 标题前缀 → 提示改名，注意改名**不能**解除 UNDER_DEV）。
- 典型闭环：push → `+push-review-status` 看到 `wip:true` → 网页取消 WIP → `mrs merge`。


## MR `url`（CLI 0.15.x）

`codeup mrs list` / `+open-mrs` 会为每条 MR 注入可点击 `url`（优先 API `detailUrl`，否则拼控制台链接）与 CLI 计算的 `status` / `wip`（#132）。`mrs get` / `create` / `+create` / `update` 写入 `meta.url`。`get` 将 OpenAPI `status` 同步为脚本友好的 `state`，并加 `wip`；`--brief` 只出 localId/title/status/state/wip/url。`create` / `+create` / `update` **默认 brief 摘要**（破坏性：依赖完整 MR JSON 的脚本请加 `--full`）。
`codeup mrs list` / `+open-mrs` 会为每条 MR 注入可点击 `url`（优先 API `detailUrl`，否则拼控制台链接）。`mrs get` / `create` / `+create` / `update` 写入 `meta.url`。`get` 将 OpenAPI `status` 同步为脚本友好的 `state`。

`mrs get` 输出三档（#130）：默认 **summary**（brief 字段 + `mergeable` / `conflictCheckStatus` / `checkList`(含 `requirementRuleItems`) / `supportMergeFastForwardOnly` / `allRequirementsPass` / `ahead` / `behind` / `reviewers` 摘要 `[{name, opinion}]`；API 未返回的键省略，`meta.projection="summary"`）；`--brief` 只出 localId/title/status/state/detailUrl/url；`--full` 为完整原始对象（#130 之前的默认输出）。兼容开关 `YUNXIAO_MRS_GET_VIEW=full|summary|brief`（flag > env > 默认 summary，非法值在发请求前报错；`--dry-run` 在 `request.projection` 显示所选档位）——新旧 CLI 通吃的脚本设 `YUNXIAO_MRS_GET_VIEW=full` 即可。`create` / `+create` / `update` **默认 brief 摘要**（破坏性：依赖完整 MR JSON 的脚本请加 `--full`）。

```bash
yunxiao codeup mrs list --state opened
yunxiao codeup mrs list --state opened --all
yunxiao codeup mrs list --state opened --source feat/x --repo <repoId> --all   # 客户端按源分支过滤（#96）
yunxiao codeup mrs list --state opened --status UNDER_DEV   # 客户端过滤（#132）
yunxiao codeup +open-mrs
```

`mrs list --source/--target`（#96）按 `sourceBranch`/`targetBranch` **精确**过滤；OpenAPI 无对应 query 参数，过滤在客户端完成（`meta.filtered_by=client`，`meta.total`/`has_more` 仍是服务端过滤前的口径）。不带 `--all` 时只过滤当前页——要全量请配 `--all`。无匹配返回空列表（`ok:true`）。

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
yunxiao codeup mrs comments create --repo <id> --local-id 1 --content "LGTM" --dry-run   # GLOBAL：缺省最新 patchset（0.16.31+；更早版本仍必填 --patchset-biz-id）
yunxiao codeup mrs comments create --repo <id> --local-id 1 --content "LGTM" --patchset-biz-id <biz> --dry-run   # 显式指定优先，且不发解析 GET
# INLINE（行内）：以下参数全部必填，不做自动解析；patchset_biz_id 取 from/to 之一；官方文档：from=比较的起始版本，to=比较的目标版本（与 MERGE_TARGET / MERGE_SOURCE 的对应关系未经验证）
yunxiao codeup mrs comments create --repo <id> --local-id 1 --comment-type INLINE_COMMENT \
  --content "这里需要判空" --patchset-biz-id <to-biz> \
  --from-patchset-biz-id <from-biz> --to-patchset-biz-id <to-biz> \
  --file-path src/main/App.java --line-number 42 --dry-run
yunxiao codeup mrs comments resolve --repo <id> --local-id 1 --comment-biz-id <biz> --dry-run
yunxiao codeup mrs comments reopen --repo <id> --local-id 1 --comment-biz-id <biz> --dry-run
# --comment-biz-id 取自 comments list 每条的 comment_biz_id（JSON 里也可能是 commentBizId）
yunxiao codeup mrs labels list --repo <id> --local-id 1
yunxiao codeup mrs labels attach --repo <id> --local-id 1 --label-ids 1,2 --dry-run
yunxiao codeup mrs reviewers add --repo <id> --local-id 1 --reviewer <userId1,userId2> --dry-run
```

`comments create` / `comments resolve` / `comments reopen` / `labels attach` / `reviewers add` 为 **write**（`--dry-run` 可预览）。

**GLOBAL_COMMENT 缺省 patchset（0.16.31+；更早版本仍必填 `--patchset-biz-id`，#93）：** 省略 `--patchset-biz-id` 时，CLI 直接 `GET …/changeRequests/{localId}/diffs/patches`（与 `mrs diffs` 同一端点，只读）选最新 patchset：

1. 候选：`relatedMergeItemType=MERGE_SOURCE` 的条目；**仅当**返回里**完全没有带类型的条目**（任何 `relatedMergeItemType` 值——MERGE_TARGET 或其他未知值——都会关闭回退）时，才退而使用**未带** `relatedMergeItemType` 的条目；有带类型条目但没有 MERGE_SOURCE → 报错；`MERGE_TARGET` 永不选中。
2. 排序（全序，与返回顺序无关）：`versionNo` 最大（字符串按数值比，`"10"` > `"9"`；`"3.0"` 视为 3）→ 有可解析 `createTime` 的优先 → `createTime` 最新（RFC3339 带时区比较；无时区按 UTC；≥10 位纯数字按 epoch 秒/毫秒，`"20260929"` 这类短数字视为无法解析）→ 仍完全并列时取返回顺序中**靠后**的一条。

- `--comment-type` 大小写不敏感，只接受 `GLOBAL_COMMENT` / `INLINE_COMMENT`；只有 GLOBAL 自动解析（`inline_comment` 缺 `--patchset-biz-id` 照样报必填）。
- 这次 GET 在 `--dry-run` 下**也会发出**：需要有效凭证与网络；GET 失败或没有候选时直接报错 `resolve latest patchset for MR <n>: …`（不回退、不发评论；HTTP 错误仍为 `type:"api"` + 状态码），hint 里带实际的 `mrs diffs --repo <你传的值> --local-id <n>` 或提示显式传 `--patchset-biz-id`。
- dry-run 在 `request.resolved` 展示 `patchset_biz_id` / `patchset_source=latest` / `resolved_via`（`GET …/diffs/patches`）/ `version_no`（缺失时省略）；成功时 `meta.patchset_biz_id` + `meta.patchset_source=latest`。
- 显式传 `--patchset-biz-id` 始终优先，且**跳过**这次 GET。
- 回复（`--parent-comment-biz-id`）未显式传 patchset 时同样挂到**最新** patchset，而不是父评论所在版本（官方文档未说明回复是否须与父评论同一 patchset）；需要同版本时请显式传入父评论所在的 patchset。`related_patchset.patchSetBizId` 只在 CreateChangeRequestComment **响应**的评论对象中有文档记载，`comments list` 是否返回该字段未经验证——拿不到时，对照父评论的时间来确定它所在的 patchset（不要自行按 `createTime` 给 patchset 排序来找「最新」）。

**INLINE_COMMENT（所有版本）：** 必须同时给出 `--comment-type INLINE_COMMENT`、`--patchset-biz-id`、`--from-patchset-biz-id`、`--to-patchset-biz-id`、`--file-path`、`--line-number`（>0），缺任何一项都在请求前报错，不做自动解析。

patchset biz id 可从 `mrs diffs` 取得（0.16.32+：`meta.latest_patchset_biz_id` 或 `data[]` 中 `latest:true` 的一项即最新，规则同上，无需自行按 createTime 排序；无候选时省略该 meta、全部 `latest:false`；`latest` 由 CLI 注入，会覆盖 API 同名字段；`--jq '.data[] | select(.latest)'` 假设 `data` 为数组，#94）；`--comment-biz-id` 取自 `comments list` 的 `comment_biz_id`。

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

merge 被拒（405 `SYSTEM_FORBIDDEN_ERROR`「该状态下的评审不允许合并」）多为推送评审 MR 卡
「开发中」(`UNDER_DEV`)：CLI 会自动在 `error.details.mr` 带出当前 status/wip/todo 并给
hint。取消 WIP 需网页（MR 页「…」→ 取消 WIP）——没有 OpenAPI；改标题去 `WIP: ` 前缀对
UNDER_DEV 无效（标题前缀是另一套 WIP 信号，见 #135）。
**merge 预检（#130）**：真正 POST 前（`--dry-run` 下也会做只读 GET）CLI 先取一次 MR 详情，校验状态一致性（MERGED/CLOSED 直接拒绝）、`conflictCheckStatus`（HAS_CONFLICT / CHECKING）、`mergeable=false`，并在详情携带合并方式配置时校验 `--merge-type`（`mergeTypes` / `supportedMergeTypes` / `mergeSetting`，或 `supportMergeFastForwardOnly=false` 拒 ff-only；网页端「创建合并节点」等写法会归一化）。不满足时 exit 1 + 结构化错误（`merge_type_not_supported` 等）+ 可行动 hint（列出可用方式，如 `available: ff-only, no-fast-forward`），**不发 POST**；通过时成功输出带 `meta.precheck`（dry-run 为 `request.precheck`）。详情 GET 失败则拒绝合并（fail closed）。无 `--yes` 时确认门仍先行（exit 10，不发任何请求）。

### write（`--dry-run` 即可预览；非 high-risk，一般不需 `--yes`）

```bash
yunxiao codeup mrs update --repo <id> --local-id 1 --title "WIP: docs" --dry-run
yunxiao codeup mrs update --repo <id> --local-id 1 --work-item ZYPT-5573 --dry-run
yunxiao codeup mrs update --repo <id> --local-id 1 --wip --dry-run    # 加 WIP: 前缀（#97）
yunxiao codeup mrs update --repo <id> --local-id 1 --unwip            # 去掉 WIP:/WIP 前缀（#97）
yunxiao codeup mrs link --repo <id> --local-id 1 --work-item ZYPT-5573 --dry-run
yunxiao codeup mrs unlink --repo <id> --local-id 1 --work-item ZYPT-5573 --dry-run
```

`update` / `link` / `unlink` 为 **write**（标题/描述或工作项关联变更），不是 high-risk-write。

`mrs update --wip/--unwip`（#97）：WIP 前缀 = 行首 `WIP`（大小写不敏感）后跟冒号（冒号两侧空格可选）或空白（`WIP: x` / `wip x` / `Wip:x`；`WIPfix` 不算）。两者都需要先 GET 当前标题（`--dry-run` 下也会发这次 GET）；前缀已加/已无 → 幂等无操作（不发 PUT，`meta.wip_changed=false`）；成功路径 `meta.wip_action` + `meta.wip_changed`，dry-run `request.resolved.{before,after,changed}`。与 `--title` 互斥（`--wip` 与 `--unwip` 也互斥）；可与 `--description` / `--work-item` 组合。

`mrs update` 的数字 `--repo` 会校验是否属于当前 organization/profile 可达仓（profile.repositories ∪ org GET）；不匹配直接报错（无 `--yes` 放行）。别名未注册仍按 #49 失败。

### read

```bash
yunxiao codeup mrs get --repo <id> --local-id 1 --brief   # 含 wip（UNDER_DEV 时 true）
yunxiao codeup mrs +push-review-status --repo <id>        # open MR 状态巡检（#132）
yunxiao codeup mrs get --repo <id> --local-id 1            # 默认 summary：够判断能不能合
yunxiao codeup mrs get --repo <id> --local-id 1 --brief      # 最小 6 键
yunxiao codeup mrs get --repo <id> --local-id 1 --full       # 完整原始对象
yunxiao codeup mrs diffs --repo <id> --local-id 1   # 每项 latest:true|false + meta.latest_patchset_biz_id（0.16.32+）
yunxiao codeup mrs diffs --repo <id> --local-id 1 --jq '.meta.latest_patchset_biz_id'
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
