> **状态：草稿 / 未发布**（仅存仓库 wiki，勿直接当已发 CSDN 文）  
> 系列：yunxiao-cli CSDN 引流 · 基于 **v0.16.40**（main 已含 #170 `auth refresh`，随 0.16.41 发版）  
> 命令以本机 `yunxiao <cmd> --help` / `schema` 为准；写操作示例默认带 `--dry-run`。

# 别再网页里翻 MR：`yunxiao codeup` 列出/合并/诊断拒绝原因

Codeup 网页找 MR 的痛点很固定：仓库多、状态乱、合并失败只丢一句含糊错误。yunxiao-cli 的 `codeup` 域把「列仓库 / 列打开的 MR / 合并前预检 / 被拒诊断」收成可复制命令。

## 先看打开的 MR

```bash
# 当前组织下打开的 MR（+shortcut，只读）
yunxiao codeup +open-mrs

# 限定仓库
yunxiao codeup +open-mrs --repo org/my-repo
yunxiao codeup +open-mrs --repo my-alias
```

每条结果会带可点击的 `url`，以及 CLI 计算的 `status`（推送评审生命周期：`UNDER_DEV` / `UNDER_REVIEW` / `TO_BE_MERGED` / `CLOSED` / `MERGED`）和可选 `wip` 标记。

等价 typed 写法：

```bash
yunxiao codeup mrs list --state opened --repo org/my-repo
```

## `--repo` 怎么写（#125）

`--repo` 接受多种形式，不必死记数字 id：

| 写法 | 说明 |
|------|------|
| 数字 `repositoryId` | 直接用 |
| profile 别名 | 先 `yunxiao profile repo-add <alias> <repo>` |
| `org/repo` 或 `org/group/repo` | CLI 自动 URL 编码（传普通斜杠即可） |
| 裸仓库名 | 只读搜索；唯一则解析，歧义会列出候选 id |

```bash
yunxiao codeup repos list
yunxiao profile repo-add app org/my-repo
yunxiao codeup +open-mrs --repo app
```

## 按源/目标分支过滤（#96）

OpenAPI 没有 `--source` / `--target` 查询参数，CLI 在拉列表后**客户端过滤**，并在 `meta.filtered_by:"client"` 标明。当前页过滤会漏掉后面页——需要全量时加 `--all`：

```bash
yunxiao codeup mrs list --repo org/my-repo --state opened --source feature/x --target master --all
```

## WIP 标题开关（#97）

约定「标题带 `WIP:` = 未定稿」时：

```bash
yunxiao codeup mrs update --repo org/my-repo --local-id 125 --wip --dry-run
yunxiao codeup mrs update --repo org/my-repo --local-id 125 --unwip --dry-run
# 确认后再去掉 --dry-run
```

`--wip` / `--unwip` 会先只读 GET 当前标题，幂等；可与 `--description` / `--work-item` 组合，但不要和 `--title` 同时用。

> 推送评审下 **`UNDER_DEV`（开发中/WIP）时 `mrs merge` 会被挡**。OpenAPI 无法「取消 WIP」，需在 Codeup 网页 MR 页操作；可用 `yunxiao codeup mrs +push-review-status --repo <r>` 跟踪。

## 合并：先 `--dry-run`，再 `--yes`

合并是 **high-risk-write**：

```bash
yunxiao codeup mrs merge \
  --repo org/my-repo \
  --local-id 12 \
  --merge-type squash \
  --dry-run
```

预检（#130）在真正 POST 前（`--dry-run` 也会做只读 GET）检查：是否已 MERGED/CLOSED、冲突检测中、`mergeable=false`、`--merge-type` 是否在仓库支持列表等。失败会给出结构化错误（如 `mr_conflict` / `merge_type_not_supported`），而不是默默 405。

确认后：

```bash
yunxiao codeup mrs merge \
  --repo org/my-repo \
  --local-id 12 \
  --merge-type squash \
  --yes
```

可选：`--message`、`--remove-source-branch`。

## 合并被拒时：可读的诊断（#127）

若 POST 仍被 API 拒绝，CLI 会再 GET 一次当前 MR，丰富错误：

- `error.subtype`: `merge_rejected`
- `details.current_status` / `state_gap`
- `suggested_actions`
- `details.mr`（status / wip / ahead / behind / mergeable / url 等）

网页上往往只剩一句 API error；CLI 把「现在卡在哪、建议下一步」摊开——适合贴进评审评论（记得打码仓库与人员）。

## 和网页对比

| 场景 | 网页 | CLI |
|------|------|-----|
| 列打开的 MR | 点仓库 → MR 列表 | `codeup +open-mrs` |
| 按分支筛 | 手点过滤 | `--source/--target` + `--all` |
| 合并失败 | 含糊错误 | `merge_rejected` + `suggested_actions` |
| 脚本/Agent | 难 | JSON + 退出码 + `--dry-run` |

下一篇转到 **Projex 工作项**：搜索时间窗、建 Bug/需求、状态流转。欢迎用 `--dry-run` 贴一次打码后的诊断 JSON 到评论区。

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

**本文相关**：Codeup MR：`+open-mrs`、`mrs merge --dry-run`、合并拒绝诊断 `merge_rejected`  
觉得有用请给仓库点个 **Star**，Issues 欢迎提场景。
