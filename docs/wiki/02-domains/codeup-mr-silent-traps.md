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
