# Pipeline 人工卡点（ManualValidate）

## 概念
Flow 运行中的人工确认 job（常见 `jobSign=ManualValidate`）。run detail 中 `stages[].stageInfo.jobs[]`（或 `stages[].jobs[]`）带 `actions: ["pass","refuse"]` 时表示可审批。

## CLI 闭环
1. **发现**：`yunxiao pipeline +pending --pipeline-id <id>`（或 `--all-pipelines`）
2. **审批**：`yunxiao pipeline job pass|refuse ... --yes` 或 `+approve` / `+refuse`
3. **等待**：`yunxiao pipeline run watch --pipeline-id <id> --run-id <rid>`  
   - exit **3** = 停在卡点（`gate_paused`）  
   - exit **4** = 超时

## 与阿里云 UID
`validatorType: users` 时 validators 需要数字阿里云 UID，见 organization `--include-aliyun-uid`（#19）。

## 边界
- 领域规则在 `internal/pipelinegate`；cmd 只做编排与 I/O
- 无 WAITING 运行时依赖 fixture 单测

## --all-pipelines 部分成功（0.16.11+）

中途 403/5xx 不整单失败：`meta.scanned` / `skipped_no_permission` / `errors`（与 `+queue` 对齐）。单 `--pipeline-id` 仍硬失败。
