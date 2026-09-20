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
