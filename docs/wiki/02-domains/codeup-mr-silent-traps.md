# Codeup MR 静默失败陷阱（客户端缓解）

## workItemIds 类型与挂单校验（0.16.12+）

OpenAPI `CreateChangeRequest` 的 `workItemIds` 是 **逗号分隔的 string**，不是 JSON 数组。
CLI 0.16.11 及更早若发送 `[]string`，服务端会静默忽略，MR 建成但无关联。

CLI 0.16.12+：

- 创建前对每个 id/serial 做 `workitem get`（不存在则中止）。
- body 发送 `workItemIds: "id1,id2"`。
- 创建后通过 `workitems/{id}/extRelationRecords?category=codeupMergeRequest` 核对；缺失则尝试 `CreateWorkitemExtRelationRecord` 补挂。
- 仍缺失则 **ok=false / 非 0 退出**（不假装成功）。MR 可能已存在：关单重建，或在网页补挂。

`mrs get` 的详情响应通常不含工作项字段；核对请用工作项侧 `extRelationRecords`，或创建路径返回的 `meta.work_item_linked`。

## list 端点忽略过滤参数

`GET .../changeRequests` 可能静默忽略 `repositoryId` / `status`。请用：

- `projectIds`（CLI：`--repo`）
- 小写 `state`（CLI：`--state`，如 `opened`）

`yunxiao codeup mrs list --help` 有 WARNING 说明。

## 已有 MR 补挂 / 解绑（0.16.17+ / #54）

推送评审仓自动创建的 MR、或建 MR 时漏传 `--work-item`，可用：

```bash
yunxiao codeup mrs link --repo <id> --local-id <n> --work-item ZYPT-5573 --dry-run
yunxiao codeup mrs unlink --repo <id> --local-id <n> --work-item ZYPT-5573 --dry-run
yunxiao codeup mrs update --repo <id> --local-id <n> --work-item ZYPT-5573 --dry-run
```

实现走 Projex `extRelationRecords`（`category=codeupMergeRequest`）：

| 操作 | HTTP |
|------|------|
| 挂单 | `POST .../workitems/{id}/extRelationRecords`（body: mergeRequestId / projectId / 可选 title、url、branches） |
| 列表 | `GET .../extRelationRecords?category=codeupMergeRequest` |
| 解绑 | `DELETE .../extRelationRecords/{relationRecordId}` |

**不要**用 `UpdateChangeRequest` / `mrs update` 的 title 字段去挂工作项；`mrs update --work-item` 内部同样走 extRelationRecords（加法、幂等）。

## 数字 --repo 归属校验（mrs update / #63）

`mrs update --repo <数字id>` 若传错其它项目的 repo id，旧版会直接对非预期仓库写入。

CLI 现对 **数字 id** 校验是否在当前 organization/profile 可达仓清单中：

- `profile.repositories` 已注册的 id；或
- 当前组织下 `GET .../repositories/{id}` 可达

不匹配 → **直接报错**（不提供 `--yes` 绕过）。别名路径仍走 #49（未注册即失败）。

## comments create 缺省 patchset（0.16.31+ / #93）

`GLOBAL_COMMENT` 省略 `--patchset-biz-id` 时，CLI 发一次只读 `GET .../changeRequests/{localId}/diffs/patches`（`--dry-run` 也会发，需凭证与网络；失败直接报错 `resolve latest patchset for MR <n>: …`、不回退；HTTP 错误仍是 `type:"api"` 并带状态码，hint 提示显式传 `--patchset-biz-id`），按以下规则选版本：

1. 候选：`relatedMergeItemType=MERGE_SOURCE`；**仅当**返回里完全没有带类型的条目（整批都未带 `relatedMergeItemType`，如旧接口）时，才退而使用未带类型的条目；任何带类型条目（MERGE_TARGET 或其他未知值）都会关闭该回退，此时没有 MERGE_SOURCE 就直接报错（未带类型的条目不当作源版本）。`MERGE_TARGET`（目标分支快照）永不选中——它的 `createTime` 可能最新，单纯按时间排序会选错。
2. 排序（全序，结果与返回顺序无关，除非完全并列）：`versionNo` 最大（`"10"` > `"9"`，`"3.0"` 视为 3，小数/非数字视为无版本）→ 有可解析 `createTime` 的优先于无法解析的 → `createTime` 最新（RFC3339；无时区按 UTC；≥10 位纯数字按 epoch 秒/毫秒；`"20260929"` 等不足 10 位的数字视为无法解析）→ 仍完全并列时取返回顺序靠后。

`--comment-type` 大小写不敏感，只接受 `GLOBAL_COMMENT` / `INLINE_COMMENT`（其他值直接报错）；只有 GLOBAL 才会自动解析。dry-run 结果在 `request.resolved` 展示所选 `patchset_biz_id` / `patchset_source=latest` / `resolved_via`（`GET <diffs/patches 路径>`）/ `version_no`（缺失时省略，不显示 0）；显式 `--patchset-biz-id` 优先且跳过该 GET。回复（`--parent-comment-biz-id`）未显式传 patchset 时也挂到最新版本，而不是父评论所在版本（官方文档未说明回复是否须与父评论同一 patchset）；需要同版本请显式传入父评论所在的 patchset（`related_patchset.patchSetBizId` 仅在 CreateChangeRequestComment 响应的评论对象中有文档记载，`comments list` 是否返回未经验证）。`INLINE_COMMENT` 不做缺省：`--patchset-biz-id` / `--from-patchset-biz-id` / `--to-patchset-biz-id` / `--file-path` / `--line-number` 全部必填。

## mrs diffs 标记最新 patchset（0.16.32+ / #94）

`GET .../diffs/patches` 返回乱序、无「最新」标记。`yunxiao codeup mrs diffs` 保留 API 原字段与顺序，并追加：

- 每项 `latest: true|false`（至多一条为 true；该字段由 CLI 注入到 API 原始对象中，若 API 返回同名字段会被**覆盖**）；
- `meta.latest_patchset_biz_id`（无候选时省略）与 `meta.latest_version_no`（缺失时省略）。

「最新」与上文 comments create 缺省规则完全相同（共用 `internal/mrpatchset.Latest`）。无候选（空列表 / 仅 MERGE_TARGET / 有 MERGE_TARGET（或其他带类型条目）但无 MERGE_SOURCE——即使同时存在未带类型的条目）时全部 `latest:false`，命令仍 `ok:true`。不要再按 `createTime` 自行排序——`MERGE_TARGET` 的时间可能最新。

## mrs merge 拒绝错误可行动化（0.16.38+ / #127）

服务端拒绝合并（405「该状态下的评审不允许合并，请刷新页面后重试」等）时，平台文案不含任何可行动信息。CLI 在合并失败后 best-effort 再 GET 一次 MR 详情（与 `mrs get` 同一端点；诊断失败则原样透传平台错误，不编造状态），并给错误信封附加：

- `error.subtype="merge_rejected"`（HTTP 层仍 `type:"api"` + 状态码，message 前缀 `merge MR <n>:`）；
- `error.details.current_status`（当前状态，识别不出时省略）、`state_gap`（状态机差距一句话）、`suggested_actions`（下一步动作数组）、`mr`（localId/title/status/url）、`diagnose.source`（`GET <MR 详情路径>`）；
- `error.hint` 汇总差距与下一步。

状态 → 建议动作：

| 当前状态 | state_gap 要点 | suggested_actions |
|----------|----------------|-------------------|
| `UNDER_DEV`（开发中/WIP） | 需先解除 WIP 才能合并 | 网页 MR 页「…」菜单 →「取消 WIP」（暂无 OpenAPI，见 #124）；随后重试 merge 命令（附可复制命令，含 `--yes`） |
| `MERGED` | 已合并，无事可做 | `mrs get` 确认 |
| `CLOSED` | 需先重开 | `mrs reopen --dry-run`（确认后 `--yes`）；重试 merge |
| `UNDER_REVIEW` | 需评审通过 | `mrs review --opinion PASS --dry-run`（确认后 `--yes`）；重试 merge |
| 其他/未知 | 按原始状态提示 | `mrs get`；重试 merge |

注意：high-risk 门（缺 `--yes` → exit 10 `confirmation_required`）先于一切诊断，不产生任何 API 调用；合并成功也不发诊断 GET。重试命令里的 `--yes` 只是提示文本，仍需用户/agent 自行确认。

## 推送评审 WIP 状态：merge 405 与状态感知（#124 / #132）

推送评审（push review）模式下 `git push` 自动创建/累积更新的 MR，初始状态为「开发中」
（服务端状态，非标题前缀）。状态枚举（`ListMergeRequests` 的 `newVersionState`、
`GetChangeRequest` 的 `status`）：

| 状态 | 含义 | 可合并 |
|------|------|--------|
| `UNDER_DEV` | 开发中（WIP） | 否（405 `SYSTEM_FORBIDDEN_ERROR`「该状态下的评审不允许合并」，即使评审已 PASS） |
| `UNDER_REVIEW` | 评审中 | 否 |
| `TO_BE_MERGED` | 待合并 | 满足合并前置条件后可 |
| `CLOSED` / `MERGED` | 已关闭 / 已合并 | — |

**没有任何已确认的 OpenAPI 可以切换该服务端状态**：`UpdateChangeRequest` 仅接受
title/description（官方文档实证），官方 MCP server 也没有 WIP 工具。「取消 WIP」只存在于
网页（MR 页右上「…」→ **取消 WIP**，状态立即转「待合并」）。CLI 因此**不发明端点**，
而是：

- `mrs merge` 失败（API 错误）时，CLI 走 #127 的诊断信封（见上节）：追加一次只读
  `GET .../changeRequests/{localId}`，`error.subtype="merge_rejected"` +
  `current_status` / `state_gap` / `suggested_actions` / `error.details.mr`
  （localId/title/status/url + state/wip/ahead/behind/mergeable/todo）。
  #124 并入的独有信息：`allRequirementsPass=false` → suggested_actions 列出未通过的
  检查项（MERGE_CONFLICT_CHECK / COMMENTS_CHECK / CI_CHECK / REVIEWER_APPROVED_CHECK，
  也在 `error.details.mr.todo`）；标题带 `WIP: ` 前缀 → 提示改名（标题前缀 WIP 是
  独立信号，PR #135 的 `--wip/--unwip` 即针对它；改名**不能**解除 UNDER_DEV）。
  刷新失败时静默退化为原始 merge 错误。
- 状态感知（#132）：`mrs list` / `+open-mrs` 每条 MR 注入 CLI 计算的 `status`
  （newVersionState 优先，旧版 `state` 兜底）与 `wip`（UNDER_DEV 或 workInProgress；
  无信号时省略；注入会覆盖 API 同名字段，旧版小写 `state` 不动 —— 同 #94 `latest` 惯例）。
  `mrs get`（含 `--brief`）同样输出 `wip`。
- `mrs list --status UNDER_DEV`：**客户端过滤**（服务端可能忽略 status 参数，见上），
  回显 `meta.status_filter`；非 `--all` 时只过滤当前页。
- 新 shortcut `codeup mrs +push-review-status --repo <r>`：列出该仓 open MR（一次 list
  GET）并逐条 GET 详情，输出 status/wip/ahead/behind/mergeable/todo/reviewers（含
  review_opinion_status）+ `meta.count` / `meta.wip_count`；`--local-id` 单查一条（仅一次
  详情 GET）；`--all` 跟页（ListAll 上限 50）。典型闭环：
  push → `+push-review-status` 发现 `wip:true` → 网页取消 WIP → `mrs merge`。
