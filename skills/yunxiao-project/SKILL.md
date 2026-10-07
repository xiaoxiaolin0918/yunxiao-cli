---
name: yunxiao-project
version: "1.4.4"
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
yunxiao workitem get --id <workItemId>                      # 默认 brief（0.16.34+）
yunxiao workitem get --id <workItemId> --fields description  # 只要描述全文
yunxiao workitem get --id <workItemId> --full               # 原始完整对象（customFieldValues 等）
yunxiao workitem comments list --id <id>   # newest first; --sort asc for oldest
# OAPI 仅 list+create；delete/update 走 AccessKey RPC（需 ALIBABA_CLOUD_ACCESS_KEY_*）
yunxiao workitem comments delete --id <id|serial> --comment-id <cid> --dry-run

See wiki: [workitem-comments-oapi-gaps.md](../../docs/wiki/02-domains/workitem-comments-oapi-gaps.md)（OAPI vs AccessKey RPC）。
yunxiao workitem comments delete --id <id|serial> --comment-id <cid> --yes   # high-risk-write
yunxiao workitem comments update --id <id> --comment-id <cid> --content-file ./note.md --dry-run
yunxiao workitem search --creator self --category Task --status-stage 1,2
yunxiao workitem comment --id <id> --content "进度更新" --dry-run   # write
yunxiao workitem comment --id <id> --content "进度更新"             # write
# Windows 中文：勿依赖 PowerShell --content；写 UTF-8 文件后：
yunxiao workitem comment --id <id> --content-file ./note.md --dry-run
# Windows 中文：subject/description/custom-fields 优先写 UTF-8 文件（去 BOM），勿依赖 WinPS 内联 argv
yunxiao workitem create --space-id <sid> --type-id <tid> --assigned-to self \
  --subject-file ./title.txt --description-file ./desc.md --custom-fields-file ./cf.json --dry-run
yunxiao workitem create --space-id <sid> --type-id <tid> --subject "t" --assigned-to self --custom-fields '{"fid":"v"}' --dry-run
yunxiao workitem types list --space-id <sid>                  # 缺省合并全部类别（含缺陷）（#99）
yunxiao workitem statuses --space-id <sid> --type-id <tid>    # 类型状态表 + meta.default_status_id（#118）
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
| `workitem types list` / `workitem statuses` | read |
| `workitem create` / `workitem comment` / `workitem update` | write（先 `--dry-run` 预览） |
| `workitem comments update` | write（AccessKey RPC；先 `--dry-run`） |
| `workitem comments delete` | high-risk-write（AccessKey RPC；`--yes`；勿用 OAPI raw DELETE） |
| `workitem delete` | high-risk-write（`--yes`） |

## 参数提示

- `--category`：`Req` | `Task` | `Bug` | `Topic` | `Risk` 等；`workitem types list` 缺省（或 `--category all`）会**并发拉全部类别合并**（每项注入 `category`，`meta.categories` 注明；个别类别失败只 warning + `meta.categories_failed`）（#99）
- `--assigned-to self`：自动解析为当前用户 id
- `--space-id`：Projex 项目/空间 id
- `--content-file`：评论正文 UTF-8 文件（自动去 BOM）；Windows 写中文评论优先用此，避免 PowerShell 编码乱码
- `workitem create` 的 `--subject-file` / `--description-file` / `--custom-fields-file`：同上（UTF-8 去 BOM；与内联 flag 互斥）；Windows 含中文建单优先用文件入参（#85）
- `workitem create` 必填预检；`--priority`/list 自定义字段支持显示值→option id（#126）（0.16.33+，#95）：POST 前（`--dry-run` 也会）读一次类型字段配置，缺失字段**一次性**报出：`error.subtype=missing_required_fields`，按 `error.details.missing[]` 的 `field_id` / `pass_via` / `options` 一次补齐（通常写进 `--custom-fields-file`），不要逐个试错。字段配置读不到 / 为空时只告警（`meta.precheck.status=skipped|empty`，看 `meta.precheck.warning`）照常创建，401 直接失败；带服务端 `defaultValue` 的字段不检查（列在 `skipped_default`）。实测验证只在 **play 沙箱**做（ZYPT 只允许 `--dry-run`）。**不要默认加 `--no-precheck`**；怀疑误报时，把 `error.details.missing` 报告给用户并询问，而不是绕过。
- `workitem create` / `+bug-create` 遇 `工作项类型未启用`（#99）：报错自动附带该 space 已启用类型 `error.details.available_types[]`（`id` / `name` / `category`，`subtype=workitem_type_not_enabled`），直接挑一个 id 重试，不必再跑一次 types list
- `workitem +bug-create`（必填预检同 create / #107，`--no-precheck` 可跳过） 的 `--title-file` / `--description-file`：同上；对齐 create（#89）；租户快捷建缺陷见 skill `yunxiao-zhiyi-ops`
- 不确定 schema 时：`yunxiao schema workitem.comment` / `workitem.search`

## 主题 / 风险 (Topic / Risk)

- **只能在项目设置 UI 启用类型**：没有用于启用 Topic/Risk 的 OpenAPI；未启用时 create 会返回 `工作项类型未启用！`（CLI 会附 `error.details.available_types` 可用类型清单，#99）。
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

Risk: **write**（真发需 `--yes`）。缺图时先 `+explore-workflow --cleanup --write-profile --yes`（写操作：沙箱先行；`--cleanup` 删探测工作项）。
`--dry-run`（#75/#82）：按 edges∪hinted_edges 分类——实证边 → `validated`；仅 hinted（**含 verified `edges` 为空但 `hinted_edges` 非空**）→ `hinted`+warning；两边皆有图但目标不在并集（含源不在边表）→ `ok:false`。仅当 **edges 与 hinted_edges 皆空**（或 `direct_status`）→ `edge_validation=skipped` + `warning`（#59）。勿把 `skipped`/`hinted` 当成必能流转。
`--write-profile`：`MergeWorkflow` 对 verified `edges` 做**并集**（保留手工回填实证边）；`hinted_edges` 若写入方非空则**整表替换**（非并集）。
+explore-workflow：--category 默认 Bug；会按 type-id 自动覆盖（勿对 Req 硬塞 Bug）（#60）。
+explore-workflow 边契约（#61 MVP）：`edges`/`verified_edges` = 实测成功；`hinted_edges`+`required_hints`/`missing_fields` = needs_fields 假阳性（勿当 verified 消费）。可用 `--custom-fields`（create）与 `--fields`（每次 PUT）补齐字段，`--from` 先定位再探。`--write-profile` 写入 `workflows[].edges`（verified）与 `hinted_edges`。逐状态必填表（通用化 bug_transition_required）仍待做。

**成功输出默认 brief（#114，对齐 create/get）**：`data.item` 为简要投影（id/serialNumber/status`{id,displayName}`/subject，无 description），并新增 `from_status` / `to_status`（`{id,displayName}`）；`serial_number` / `steps` / `applied` / `refresh_ok` / `url` 保留。`--full` 输出刷新后的完整对象（旧版行为）；`YUNXIAO_WORKITEM_GET_VIEW=full|brief` 同 `workitem get` 的兼容开关（flag > env > 默认 brief；非法值在任何请求前报 `invalid_env`）。`+bug-transition（BFS 无路时 direct_fallback / --direct，#123）` 同样适用。

**状态入场必填（#113）**：fields 端点只标类型级 `required`，状态入场必填**无 OpenAPI 配置**，dry-run 无法预判（`required_fields_note` 会提示）。真实 PUT 若 HTTP 400「xx必填」中文字段名列表，CLI 自动 GET 字段配置把名字映射回 fieldId：`error.subtype=transition_required_fields`，`error.details.fields[]`（field_id / name / current_value / 可枚举 options / pass_via / draft）+ `details.fields_draft`（可直接复制的 `--fields` 骨架；迭代等系统字段走具名根键 sprint ≠ customFields）；无匹配名字原文透出（`unmapped_names`，不猜测），映射失败降级（`mapping.source=fields_endpoint_error|empty`）。

ZYPT / 缺陷命名必填字段 → 见 [`../yunxiao-zhiyi-ops/SKILL.md`](../yunxiao-zhiyi-ops/SKILL.md)（`workitem +bug-transition` + profile）。


## meta.url / relations 富化（CLI 0.15.x）

- `workitem get/create/update` 与 `+transition` 成功时 `meta` 常含可点击 `url`（及 `serial_number` / `resolved_id`）。
- `+transition` / `+bug-transition` 成功默认 brief（#114，同 create/get 惯例）：`data.item` 简要投影 + `from_status`/`to_status` 显示名；`--full` / `YUNXIAO_WORKITEM_GET_VIEW=full` 取回完整对象。核对字段落值才需要 `--full`。
- `workitem get` 默认 brief（0.16.34+，#98）：`data` 只含 id / serialNumber / subject / **status 仅 `{id,displayName}`** / assignedTo / sprint / priority / workitemType / categoryId / gmtModified，description 以 `description_summary`（字符数）占位；`.data.status.name` / `nameEn`、描述全文、customFieldValues 等用 `--full`（或 `--fields` / `YUNXIAO_WORKITEM_GET_VIEW=full`）。`--full` / `--brief` / `--fields` 互斥；`--fields` 名字大小写敏感：GetWorkitem 字段在该工作项上缺失时为 `null`（列入 `meta.absent_fields`），完全未知的名字报 `unknown_fields` 并列出 `details.available`。兼容示例：`yunxiao workitem get X --full 2>/dev/null || yunxiao workitem get X`。详见 [workitem-get-views.md](../../docs/wiki/02-domains/workitem-get-views.md)。**先用默认视图，确需时再 `--full`**，避免把长描述塞进上下文。
- `workitem create` 创建前必填预检（0.16.33+，#95 / #103）：缺字段一次列全；详见 [workitem-create-precheck.md](../../docs/wiki/02-domains/workitem-create-precheck.md)。
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
yunxiao workitem workflow --space-id <s> --type-id <t>       # 原始 workflows 载荷
yunxiao workitem statuses --space-id <s> --type-id <t>       # 状态表（含 default 标记）（#118）
yunxiao workitem activities --id <id>
```

`workitem statuses` 从同一 workflows 端点投影状态表（`id` / `name` / `displayName` / `nameEn` + 默认态 `default:true`，`meta.default_status_id`）——配置 `profile.workflows[].statuses` / `bug_statuses` 别名时的只读数据源，不再需要 `yunxiao api` 逃生口。

## Efforts / programs (v0.9)

```bash
yunxiao workitem efforts list --id <id>
yunxiao workitem efforts mine --start-date 2026-01-01 --end-date 2026-01-31
yunxiao workitem efforts create --id <id> --actual-time 2 --gmt-start 2026-01-01 --gmt-end 2026-01-01 --dry-run
yunxiao workitem estimated-efforts list --id <id>
yunxiao programs search --name demo
```

efforts create/update = **write**。来源：`operations/projex/effort.ts`、`searchProgramsFunc`。
