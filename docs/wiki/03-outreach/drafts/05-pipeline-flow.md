> **状态：草稿 / 未发布**（仅存仓库 wiki，勿直接当已发 CSDN 文）  
> 系列：yunxiao-cli CSDN 引流 · 基于 **v0.16.40**（main 已含 #170 `auth refresh`，随 0.16.41 发版）  
> 命令以本机 `yunxiao <cmd> --help` / `schema` 为准；写操作示例默认带 `--dry-run`。

# 流水线不用盯控制台：`pipeline` 列表、待审批、`browse` 一键开页

值班时最耗时的往往不是「改 YAML」，而是：**来回切 Flow 页签，找哪条跑挂了、卡在谁的人工卡点**。yunxiao-cli 的 `pipeline`（别名 `flow`）把列表、最新状态、待审批和浏览器跳转收成终端命令。

## 列表与最新状态（只读起步）

```bash
yunxiao pipeline list
yunxiao pipeline +status --pipeline-id <id>
yunxiao pipeline +failed --pipeline-id <id>
```

typed 运行观察（以 `--help` 为准）：

```bash
yunxiao pipeline run list --pipeline-id <id>
yunxiao pipeline run latest --pipeline-id <id>
yunxiao pipeline run get --pipeline-id <id> --run-id <rid>
yunxiao pipeline run watch --pipeline-id <id> --run-id <rid>
```

## 待审批 / 人工闸门

```bash
# 推荐：指定流水线扫 WAITING 运行里的 pass/refuse 类任务
yunxiao pipeline +pending --pipeline-id <id>

# 也可扫多条（有页预算与硬顶 50 条流水线，注意成本）
yunxiao pipeline +pending --all-pipelines
yunxiao pipeline +pending --pipeline-id <id> --include-running
```

跨流水线扫描时，中途 403/5xx **不会整次中断**，失败会收进 `meta.errors` / `meta.skipped_no_permission`。

**通过 / 拒绝闸门是 high-risk-write**，必须用户确认后再带 `--yes`：

```bash
yunxiao pipeline +approve \
  --pipeline-id <id> --run-id <rid> --job-id <jid> --dry-run

yunxiao pipeline +approve \
  --pipeline-id <id> --run-id <rid> --job-id <jid> --yes

yunxiao pipeline +refuse \
  --pipeline-id <id> --run-id <rid> --job-id <jid> --yes
```

触发运行、取消、改 pipeline YAML（`create` / `update`）同样是高风险——先 `--dry-run`，再确认后 `--yes`。可用 `yunxiao pipeline diff` 对比本地 YAML 与线上定义。

## `browse`：终端出链接，再决定是否开浏览器

```bash
yunxiao browse pipeline --pipeline-id <id> --print-only
yunxiao browse pipeline --pipeline-id <id> --run-id <rid>
yunxiao browse workitem --space-id <sid> --serial ZYPT-1 --print-only
yunxiao browse mr --repo-url https://codeup.aliyun.com/org/repo --local-id 3 --print-only
```

`--print-only`（或全局 `--dry-run`）只打印 URL，适合 SSH / 远程会话。

## 和 `yunxiao status` 联动

早晨一条看板，少开三个页签：

```bash
yunxiao status
# 需要顺带看闸门时：
yunxiao status --pipeline-id <id>
# 或（更贵）yunxiao status --all-pipelines
```

`status` 聚合：

- `workitems`：指派给我的打开项（默认可按 category / status-stage 调）
- `mrs`：打开的 MR（与 `+open-mrs` 同类富化）
- `pending_gates`：仅当提供 `--pipeline-id` 或 `--all-pipelines` 时启用（全量扫贵，默认跳过并给 reason）

各段 soft-fail：一段失败不拖死整次输出。

## 安全清单

| 操作 | 风险 | 建议 |
|------|------|------|
| `list` / `+status` / `+pending` / `run watch` | read | 随便跑 |
| `+approve` / `+refuse` / `run trigger` / `cancel` | high-risk-write | `--dry-run` → 确认 → `--yes` |
| `pipeline create|update`（YAML） | high-risk-write | 先 `diff`，再 dry-run |

下一篇讲 **给 AI Agent 用的 skills / profile / `--dry-run` 护栏**。收藏本篇 + Star，值班会轻松一点。

---

### 安装 yunxiao-cli（GitHub Releases，勿用 npm）

**仓库**：https://github.com/xiaoxiaolin0918/yunxiao-cli  
**最新 Release**：https://github.com/xiaoxiaolin0918/yunxiao-cli/releases/latest  

**Windows（PowerShell）一行安装：**

```powershell
irm https://raw.githubusercontent.com/xiaoxiaolin0918/yunxiao-cli/main/install.ps1 | iex
```

默认装到 `%USERPROFILE%\.local\bin`（归档内 `skills/`、`profiles/` 会一并落下）。之后升级：

```powershell
yunxiao update --yes
```

**macOS / Linux**：在 Releases 下载对应平台归档，解压后把 `yunxiao` 放到 `PATH`，然后：

```bash
yunxiao auth login --browser
yunxiao whoami && yunxiao doctor && yunxiao status
```

> npm 包装 `sanzhi-yunxiao-cli` 已停用，请勿再 `npm install`。

**本文相关**：Flow 流水线：`pipeline +pending`、`browse --print-only`、与 `yunxiao status` 联动  
觉得有用请给仓库点个 **Star**，Issues 欢迎提场景。
