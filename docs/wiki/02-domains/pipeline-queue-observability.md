# 流水线排队 / Runner 排障可观测

## 背景

私有 Runner（`runsOn.group: private/...`）被长构建占用时，多条流水线会出现 WAITING 堆积。公开 Flow OpenAPI **没有** runner 组在线数 / 队列深度接口。

## CLI（0.16.10+）

1. `yunxiao pipeline +queue [--pipeline-id] [--group private/xxx]`  
   扫描流水线（默认最多 50）上的 RUNNING/WAITING，带等待秒数；能读到 YAML 时附带 `runner_groups`。

2. `yunxiao pipeline runner-groups list`  
   从流水线 YAML 的 `runsOn.group` 发现组（非服务端库存 API）。

3. `yunxiao pipeline runner-groups status --group private/xxx`  
   按组过滤排队视图；`online_executors` 恒为不可用说明。

4. `pipeline run get` 在 WAITING/RUNNING 时 `meta.queue`：等待时长、YAML 中的 runner_groups、仍 WAITING 的 jobs。

5. 流水线相关 **403** 会提示按流水线核对 `resource-members` / 网页提权。

## 局限

- 无法从 OpenAPI 得知「被哪个 run 占用」或执行器在线数；需网页或本 CLI 的跨流水线 RUNNING 列表辅助推理。

## 截断与失败可见性

- 默认最多扫 50 条流水线；`meta.truncated=true` 表示流水线列表被截断。
- 每条流水线每个状态只取第一页（20 条）；`meta.runs_truncated=true` 表示可能还有更多 RUNNING/WAITING。
- `--group` 时若某流水线 YAML 读失败，仍会列出其排队 run，并在 `meta.group_filter_yaml_unknown` 标明；`yaml_errors` / `run_list_errors` 汇总失败原因。
- 大组织扫描会放大 GET 次数，巡检请加 `--pipeline-id` 或 `--group` 收窄。

