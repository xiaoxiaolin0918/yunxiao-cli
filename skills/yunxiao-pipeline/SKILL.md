---
name: yunxiao-pipeline
version: 1.1.1
description: "云效 Flow 流水线：列表/详情、YAML 创建更新、运行 list/get/trigger/cancel/latest、任务日志、人工卡点 pass/refuse。控制台 URL 见 data[].url / meta.url。"
metadata:
  requires:
    bins: ["yunxiao"]
  cliHelp: "yunxiao pipeline --help"
---

# pipeline (Flow)

开始前先读 [`../yunxiao-shared/SKILL.md`](../yunxiao-shared/SKILL.md)。

## Shortcuts（优先）

| Shortcut | 说明 | Risk |
|----------|------|------|
| `+failed` | 某流水线最近失败运行摘要（需 `--pipeline-id`） | read |
| `+status` | 最近一次运行状态摘要 | read |

```bash
yunxiao pipeline +failed --pipeline-id <id>
yunxiao pipeline +status --pipeline-id <id>
yunxiao pipeline list
yunxiao pipeline list --all                    # 跟页（ListAll，上限 50）
yunxiao pipeline get --id <id>
```

## 控制台 URL（CLI 0.15.x）

| 对象 | 模式 | 出现位置 |
|------|------|----------|
| 流水线 | `https://flow.aliyun.com/pipelines/{id}` | `data[].url`（list）/ `meta.url`（get） |
| 运行 | `https://flow.aliyun.com/pipelines/{id}/builds/{runId}` | `data[].url` / `meta.url` |

Agents 展示结果时优先附上这些可点击链接。

## 运行（run）

```bash
yunxiao pipeline run list --pipeline-id <id>
yunxiao pipeline run list --pipeline-id <id> --status FAIL
yunxiao pipeline run latest --pipeline-id <id>
yunxiao pipeline run get --pipeline-id <id> --run-id <rid>
yunxiao pipeline run trigger --pipeline-id <id> --dry-run
# 用户确认后：
yunxiao pipeline run trigger --pipeline-id <id> --yes
yunxiao pipeline run cancel --pipeline-id <id> --run-id <rid> --dry-run
yunxiao pipeline run cancel --pipeline-id <id> --run-id <rid> --yes
```

`trigger` / `cancel` 为 **high-risk-write**，真发必须 `--yes`。

List 分页：`meta.has_more` / `total` / `page`；完整集用 `pipeline list --all`（见 shared）。缺省单页不得当成全集。

## 任务 / 卡点

```bash
yunxiao pipeline job log --pipeline-id <id> --run-id <rid> --job-id <jid>
yunxiao pipeline job pass --pipeline-id <id> --run-id <rid> --job-id <jid> --dry-run
yunxiao pipeline job refuse --pipeline-id <id> --run-id <rid> --job-id <jid> --dry-run
```

`pass` / `refuse` 为 **high-risk-write**。

## YAML 创建 / 更新（high-risk-write）

```bash
yunxiao pipeline create --name ci --file ./pipeline.yaml --dry-run
yunxiao pipeline update --id <id> --name ci --file ./pipeline.yaml --dry-run
# 用户确认后加 --yes
```

`--file` 必须是 cwd 下相对路径。

## Flow extras

```bash
yunxiao pipeline service-connections list --type codeup
yunxiao pipeline host-groups list
yunxiao pipeline flow-variable-groups list
yunxiao pipeline resource-members list --resource-type PIPELINE --resource-id <id>
```

`flow-variable-groups` 是 **Flow 组织级**变量组，与 AppStack `variable-groups` 不同。

## Flow 公共 runner 与 Go 镜像（可选）

组织 CI / Flow 公共 runner 上访问 `proxy.golang.org` / `go.dev` 可能 SSL 不稳定。推荐：

- `GOPROXY=https://goproxy.cn,direct`（或企业代理）
- 安装 Go 时优先阿里云镜像，例如：
  `curl -fsSL -o /tmp/go.tgz https://mirrors.aliyun.com/golang/go1.24.4.linux-amd64.tar.gz`

仓库内参考脚本：[`../../scripts/flow-ci.sh`](../../scripts/flow-ci.sh)（父流程可把同内容 `pipeline update` 进 Flow YAML）。

## VM deploy / resource-members write

```bash
yunxiao pipeline vm-deploy get --pipeline-id <id> --deploy-id <did>
yunxiao pipeline vm-deploy stop --pipeline-id <id> --deploy-id <did> --dry-run
yunxiao pipeline resource-members create --resource-type pipeline --resource-id <id> --role-name viewer --user-id <uid> --dry-run
```

`vm-deploy` mutations 与 `resource-members` create|update|delete|transfer-owner 为 **high-risk-write**。
## ManualValidate / 阿里云 UID

人工卡点 `ManualValidate` 在 `validatorType: users` 时，`validators` **只认数字型阿里云 UID**，不认组织成员 hex `userId`、也不认邮箱。

取 UID（需 `ALIBABA_CLOUD_ACCESS_KEY_ID` / `SECRET`）：

```bash
yunxiao organization members list --include-aliyun-uid
yunxiao organization members search --query <name> --include-aliyun-uid
```

用返回的 `aliyunUid`（或 `accountId`）填 YAML。流水线 **resource-members** 权限仍用 hex `userId`，与卡点审批不是同一套 ID。概念见 `docs/wiki/02-domains/pipeline.md`。
