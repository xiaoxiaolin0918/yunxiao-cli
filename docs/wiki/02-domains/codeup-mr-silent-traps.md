# Codeup MR 静默失败陷阱（客户端缓解）

## workItemIds 传错不挂接

Codeup 创建变更请求时，若 `workItemIds` 无效：**不报错、不挂接**，且事后 API 往往无法补挂。

CLI（0.16.8+）：

- `codeup mrs create --work-item` / `mrs +create --work-item`：创建前对每个 id/serial 做 `workitem get`，不存在则中止。
- `+create` 在 profile 有 `space_id` 时校验工作项空间一致。
- 创建成功后回读响应中的关联字段；若缺失则 stderr + `meta.warnings` 提示（MR 已建成，需网页补挂）。

## list 端点忽略过滤参数

`GET .../changeRequests` 可能静默忽略 `repositoryId` / `status`。请用：

- `projectIds`（CLI：`--repo`）
- 小写 `state`（CLI：`--state`，如 `opened`）

`yunxiao codeup mrs list --help` 含 WARNING 说明。