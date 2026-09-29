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

`GLOBAL_COMMENT` 省略 `--patchset-biz-id` 时，CLI 发一次只读 `GET .../changeRequests/{localId}/diffs/patches`（`--dry-run` 也会发，需凭证与网络；失败直接报错、不回退），按以下规则选版本：

1. 候选：`relatedMergeItemType=MERGE_SOURCE`；若没有任何 MERGE_SOURCE 条目，退而使用未带 `relatedMergeItemType` 的条目；`MERGE_TARGET`（目标分支快照）永不选中——它的 `createTime` 可能最新，单纯按时间排序会选错。
2. 排序：`versionNo` 最大 → `createTime` 最新 → 返回顺序靠后。

dry-run 结果在 `request.resolved` 展示所选 `patchset_biz_id` / `version_no`；显式 `--patchset-biz-id` 优先且跳过该 GET。回复（`--parent-comment-biz-id`）未显式传 patchset 时也挂到最新版本，而不是父评论所在版本；需要同版本请从 `comments list` 取父评论的 `related_patchset.patchSetBizId` 显式传入。`INLINE_COMMENT` 不做缺省：`--patchset-biz-id` / `--from-patchset-biz-id` / `--to-patchset-biz-id` / `--file-path` / `--line-number` 全部必填。
