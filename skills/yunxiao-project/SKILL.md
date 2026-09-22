---
name: yunxiao-project
version: 1.2.0
description: "云效 Projex：列项目、搜/看/建工作项、评论、关联、自定义字段、附件上传。用户问需求/任务/缺陷/主题/风险/关联/项目列表时使用。"
metadata:
  requires:
    bins: ["yunxiao"]
  cliHelp: "yunxiao project --help"
---

# project / workitem (Projex)

开始前先读 [`../yunxiao-shared/SKILL.md`](../yunxiao-shared/SKILL.md)。

> List / search 的 `meta` 可能含 `has_more`；缺席 ≠ 全集完整（见 shared）。

## Shortcuts（优先）

| Shortcut | 说明 | Risk |
|----------|------|------|
| `project +my-open-items` | 指派给我且未完成的工作项（默认 category=Req，statusStage=1,2） | read |
| `project +created-by-me` | 我创建的工作项（可用 `--status-stage` 过滤未完成） | read |

```bash
yunxiao project +my-open-items
yunxiao project +my-open-items --category Task --space-id <projectId>
```

## Typed commands

```bash
yunxiao project list --name demo
yunxiao workitem search --assigned-to self --category Req
yunxiao workitem search --subject "登录" --status-stage 1,2
# 周报日期窗口 + 跟页（meta.total / --all；oapi 响应可能无 finishTime）
yunxiao workitem search --category Req --created-after "2026-09-01 00:00:00" --created-before "2026-09-07 23:59:59" --all
yunxiao workitem search --category Bug --status 100005,100010 --status-stage 1,2
yunxiao workitem get --id <workItemId>
yunxiao workitem comments list --id <id>   # newest first; --sort asc for oldest
yunxiao workitem search --creator self --category Task --status-stage 1,2
yunxiao workitem comment --id <id> --content "进度更新" --dry-run   # write
yunxiao workitem comment --id <id> --content "进度更新"             # write
yunxiao workitem create --space-id <sid> --type-id <tid> --subject "t" --assigned-to self --custom-fields '{"fid":"v"}' --dry-run
yunxiao workitem relations list --id <id> --relation-type ASSOCIATED
yunxiao workitem relations create --id <id> --related-id <rid> --relation-type ASSOCIATED --dry-run
yunxiao workitem create --space-id <sid> --type-id <tid> --subject "title" --assigned-to self --dry-run
yunxiao workitem update --id <id> --assigned-to self --dry-run      # write
yunxiao workitem update --id <id> --status <statusId>               # write
yunxiao workitem update --id <id> --status <cancelStatusId> --cancel-reason "不再需要" --dry-run
```

| Command | Risk |
|---------|------|
| `project list` / `workitem search` / `workitem get` / `workitem comments list` | read |
| `workitem types list` | read |
| `workitem create` / `workitem comment` / `workitem update` | write（先 `--dry-run` 预览） |
| `workitem delete` | high-risk-write（`--yes`） |

## 参数提示

- `--category`：`Req` | `Task` | `Bug` | `Topic` | `Risk` 等
- `--assigned-to self`：自动解析为当前用户 id
- `--space-id`：Projex 项目/空间 id
- 不确定 schema 时：`yunxiao schema workitem.search`

## 主题 / 风险 (Topic / Risk)

- **只能在项目设置 UI 启用类型**：没有用于启用 Topic/Risk 的 OpenAPI；未启用时 create 会返回 `工作项类型未启用！`。
- **不要传 `--sprint`**：Topic/Risk 通常没有迭代字段；API 会返回 `未启用此字段【迭代】`。`yunxiao workitem create --help` 也会提示这一点。
- 创建后用 `profile.workflows` 对应 `type_id` 的状态别名做 `+transition`；类别使用 `Topic` 或 `Risk`。

```bash
# 先按 Topic 类别找到已启用的 type_id；创建时不要带 --sprint
yunxiao workitem types list --space-id <sid> --category Topic
yunxiao workitem create --space-id <sid> --type-id <topicTypeId> \
  --subject "产品主题" --assigned-to self --dry-run
yunxiao workitem +transition --id <topicId> --to <alias> --profile play --dry-run

# Topic ↔ Req 关联：ASSOCIATED（sandbox 与 ZYPT 均已验证）
yunxiao workitem relations create --id <topicId> --related-id <reqId> \
  --relation-type ASSOCIATED --dry-run
yunxiao workitem relations list --id <topicId> --relation-type ASSOCIATED
```

Task 的父子层级在创建时用 `--parent-id <taskId>`；不要用 `PARENT_SUB` 关系类型。工作项关联优先用 `ASSOCIATED`，依赖关系用 `DEPEND_ON`（`RELATED` 常失败）。

创建工作项时 profile 中的 `workitem_defaults` 会自动补齐默认字段（如 priority 等）；要跳过默认值时加 `--no-defaults`。`workitem get/create/update` 在 CLI 0.14.4+ 的结果含可点击的 `meta.url`。

## 通用状态流转 / 智衣缺陷

```bash
# any type via profile.workflows[<type_id>] (fallback: bug_* when type=bug_type_id)
yunxiao workitem +transition --id <id|serial> --to <alias|statusId> --dry-run
yunxiao workitem +transition --id <id> --to 处理中 --fields '{"80":"2026-09-20T00:00:00+08:00"}' --yes
```

Risk: **write**（真发需 `--yes`）。缺图时先 `+explore-workflow --write-profile`。
`--dry-run`：有 `workflows` 边时本地校验路径（非法 → `ok:false`）；无边时 `request.edge_validation=skipped` + `warning`（勿当成必能流转；#59）。
+explore-workflow：--category 默认 Bug；会按 type-id 自动覆盖（勿对 Req 硬塞 Bug）（#60）。
+explore-workflow 边契约（#61 MVP）：`edges`/`verified_edges` = 实测成功；`hinted_edges`+`required_hints`/`missing_fields` = needs_fields 假阳性（勿当 verified 消费）。可用 `--custom-fields`（create）与 `--fields`（每次 PUT）补齐字段，`--from` 先定位再探。`--write-profile` 写入 `workflows[].edges`（verified）与 `hinted_edges`。逐状态必填表（通用化 bug_transition_required）仍待做。

ZYPT / 缺陷命名必填字段 → 见 [`../yunxiao-zhiyi-ops/SKILL.md`](../yunxiao-zhiyi-ops/SKILL.md)（`workitem +bug-transition` + profile）。


## meta.url / relations 富化（CLI 0.15.x）

- `workitem get/create/update` 与 `+transition` 成功时 `meta` 常含可点击 `url`（及 `serial_number` / `resolved_id`）。
- `workitem create` 默认 brief：`data` 保留 `id` / `serialNumber` / `status.displayName` / `subject`（create API 若返回 null 会再 GET 补齐）；`--full` 输出完整对象（#62）。依赖完整 create JSON 的脚本请加 --full。
- `workitem relations list` 会尽力为每条关联补齐 `serial_number` / `subject` / `url`（及 category）。
- 取消态更新可用 `--cancel-reason <text>`（自动查找「取消原因」字段）；dry-run 可能带 soft-warn 提示。

## 不负责

Codeup MR → `yunxiao-codeup`；流水线 → `yunxiao-pipeline`。

## 附件

```bash
yunxiao workitem attachments list --id <id>
yunxiao workitem attachments create --id <id> --file ./note.png --dry-run
```

`--file` 仅允许 cwd 相对路径。Risk: list=**read**；create=**write**（multipart）。来源：`operations/projex/attachment.ts`。

## Sprint / versions / fields

```bash
yunxiao project get --id <id>
yunxiao sprint list --space-id <id>
yunxiao versions list --space-id <id>
yunxiao workitem fields --space-id <s> --type-id <t>
yunxiao workitem workflow --space-id <s> --type-id <t>
yunxiao workitem activities --id <id>
```

## Efforts / programs (v0.9)

```bash
yunxiao workitem efforts list --id <id>
yunxiao workitem efforts mine --start-date 2026-01-01 --end-date 2026-01-31
yunxiao workitem efforts create --id <id> --actual-time 2 --gmt-start 2026-01-01 --gmt-end 2026-01-01 --dry-run
yunxiao workitem estimated-efforts list --id <id>
yunxiao programs search --name demo
```

efforts create/update = **write**。来源：`operations/projex/effort.ts`、`searchProgramsFunc`。
