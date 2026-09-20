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
