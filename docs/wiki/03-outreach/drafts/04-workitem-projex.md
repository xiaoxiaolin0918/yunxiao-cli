> **状态：草稿 / 未发布**（仅存仓库 wiki，勿直接当已发 CSDN 文）  
> 系列：yunxiao-cli CSDN 引流 · 基于 **v0.16.40**（main 已含 #170 `auth refresh`，随 0.16.41 发版）  
> 命令以本机 `yunxiao <cmd> --help` / `schema` 为准；写操作示例默认带 `--dry-run`。

# 用命令行管 Projex 工作项：搜索、建 Bug/需求、状态流转

周报、质量窗、Bug 列表——如果每次都靠 Projex 网页点选，很难进 CI。yunxiao-cli 的 `workitem`（别名 `wi`）把搜索、快捷创建和流转收成结构化命令。

## 搜索：时间窗 + 状态 + 标签

```bash
# 本周新建的需求（示例日期请改成你们的窗口）
yunxiao workitem search \
  --category Req \
  --created-after "2026-09-29 00:00:00" \
  --created-before "2026-10-05 23:59:59" \
  --all --as-items

# 指派给我的打开中 Bug
yunxiao workitem search --category Bug --assigned-to self --status-stage 1,2

# 按状态 id / 阶段
yunxiao workitem search --category Bug --status 100005 --status-stage 1,2

# 标签（配合 project labels）
yunxiao project labels list
yunxiao workitem search --labels "P0,线上"
```

常用日期旗标（映射进官方 `conditions` 的 BETWEEN / dateTime 形态）：

- `--created-after` / `--created-before`
- `--updated-after` / `--updated-before`
- `--finish-after` / `--finish-before`

分页：OpenAPI `perPage` 最大 200。看总量用 `meta.total` / `has_more`，需要跟页加 `--all`；**不要用 `len(data)` 当总数**。`--as-items` 可选，把结果收成 `{items, pagination}` 形状，方便脚本。

只读探查参数：

```bash
yunxiao schema workitem.search
```

### 已知限制（诚实写，建立信任）

| 点 | 说明 |
|----|------|
| `finishTime` | 条件过滤可能可用；search/get 响应常无该字段（schema 常见 `gmtCreate` / `gmtModified` / `updateStatusAt`）。CLI **不会**用 `updateStatusAt` 伪造 `finishTime`。 |
| 服务端日期条件 | 偶发返回窗外行——必要时在客户端再按 `gmtCreate` / `gmtModified` / 自定义字段过滤。 |
| 评论 | oapi 侧 list + create；**没有** typed 的 delete/update 评论命令。 |

## 快捷创建：Bug / 需求 / 风险

依赖本地 profile 里的类型与字段映射（先 `yunxiao +onboard` 或 `profile install-example`）：

```bash
yunxiao workitem +bug-create \
  --title "登录页 500" \
  --description "复现步骤…" \
  --expected-completion 2026-10-10 \
  --sprint <sprint-id> \
  --dry-run

yunxiao workitem +req-create --assignee "张三" --title "支持导出 CSV" --dry-run
yunxiao workitem +risk-create --title "依赖方接口不稳" --description "…" --dry-run
```

`+bug-create` 带 MissingRequired 预检（#107，可用 `--no-precheck` 跳过）。选填字段会尽量把显示值解析成 option id（#126）。确认无误后去掉 `--dry-run` 再执行。

## 状态流转

```bash
# 任意类型：多步 BFS 流转
yunxiao workitem +transition --id ZYPT-5768 --to processing --dry-run

# Bug 专用快捷
yunxiao workitem +bug-transition --id ZYPT-5768 --to processing --dry-run
```

若 BFS 找不到路径，可显式 `--direct`（平台仍可能拒绝非法跳转，#123）。需要看工作流图时：

```bash
yunxiao workitem statuses --space-id <id> --type-id <id>
yunxiao workitem +explore-workflow --type-id <id> --cleanup --dry-run
```

## 读取与评论

```bash
yunxiao workitem get ZYPT-5768          # brief
yunxiao workitem get --id ZYPT-5768 --full
yunxiao workitem comments list --id ZYPT-5768
yunxiao workitem comment --id ZYPT-5768 --content "跟进中" --dry-run
# PowerShell 中文易乱码时：
yunxiao workitem comment --id ZYPT-5768 --content-file ./note.md --dry-run
```

## 和网页 / MCP 的分工

- **周报 / 质量窗 / CI**：优先 typed CLI（可版本化、可审计）。
- **IDE 里随口问一句**：MCP 当补充；时间窗搜索仍建议落到上面的 `workitem search` 命令。

下一篇：**流水线不用盯控制台**。欢迎贴一条打码后的真实 `workitem search` 到评论区。

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

**本文相关**：Projex 工作项：`workitem search` 时间窗、`+bug-create` / `+transition`；诚实说明 finishTime 缺口  
觉得有用请给仓库点个 **Star**，Issues 欢迎提场景。
