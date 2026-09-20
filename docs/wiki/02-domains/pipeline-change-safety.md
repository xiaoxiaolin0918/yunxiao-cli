# 流水线变更安全三件套

## 背景

`pipeline update` 在自动化改配时高频使用。服务端无独立 validate 端点；`--dry-run` 只拼本地请求体。

## CLI（0.16.9+）

1. `yunxiao pipeline get --id <id> --yaml out.yaml`  
   抽出 `pipelineConfig.flow` 落盘。

2. `yunxiao pipeline diff --id <id> --file new.yaml`  
   归一化 stage/job/step 摘要（+added ~modified -removed）；删除 stage/job 或部署类变更标 `high_risk`。

3. `yunxiao pipeline update --validate --id ... --name ... --file ...`  
   更新前 GET+diff；高危变更必须 `--yes`。`--validate --dry-run` 只返回 diff、不 PUT。

4. API `errorCode=1209300`（YAML 校验失败）时，`error.details.issues` 给出 `path` + `errorMessage`。

## 说明

- Diff 按 **name 路径** 对齐：stage/job/step **重命名**会表现为「删旧 + 增新」，常被标 `high_risk`（删 unit）。改名后请人工确认再 `--yes`。
- 修改叶子 step 时，父 stage/job 也可能出现在 `modified`（整段指纹变化）；以 path 清单为准。
- 真实 `update` PUT 本身始终是 high-risk-write；`--validate` 的高危拦截是额外一层。

## --validate 空 diff 不写（0.16.11+）

- 结构 diff 为 `+0 ~0 -0` 时跳过 PUT，返回 `mode=validate_noop`（版本号不前进）。
- `--validate` 仍表示 diff-then-write；只读请用 `--validate --dry-run` 或 `--check`。
