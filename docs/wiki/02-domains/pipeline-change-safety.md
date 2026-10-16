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

- 仅当结构 diff 为空、全文 YAML 等价（含 sources/triggers）、且 `--name` 与线上一致时跳过 PUT（`mode=validate_noop`）；只改名或只改非结构字段仍会 PUT。
- `--validate` 仍表示 diff-then-write；只读请用 `--validate --dry-run` 或 `--check`。

## 复制流水线（0.16.44+ / #356）

官方 OpenAPI **没有** `CopyPipeline`。CLI 提供：

```bash
yunxiao pipeline copy --id <src> --name <new> --dry-run
yunxiao pipeline copy --id <src> --name <new> --yes
# 同义：pipeline clone
```

实现：同企业内 GET 源流水线 YAML → POST 创建。--dry-run 仍会 GET 源配置以便预览 content_bytes / source_name，但不会 POST。

**引用原样带走**：YAML 里的变量组、服务连接、代码源、主机组等 ID **不改写**；须在当前企业存在。跨企业复制不支持。

## +status 默认精简（0.16.45+ / #359）

原始最近一次运行响应常达 ~150KB，且含明文 globalParams。CLI 默认投影为精简 SummaryPipelineRun：

```bash
yunxiao pipeline +status --pipeline-id <id>
yunxiao pipeline +status --pipeline-id <id> --full   # 全量原始运行（含 globalParams）
```

- 默认输出：状态、阶段摘要、runId（已归一化）、控制台 meta.url 等；**不含** globalParams
- 需要完整字段（含密钥类参数）时显式传 --full
- Agents 日常巡检用默认；排障/对账再用 --full

