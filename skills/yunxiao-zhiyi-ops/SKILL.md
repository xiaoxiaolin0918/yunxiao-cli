---
name: yunxiao-zhiyi-ops
version: 1.3.0
description: "智衣云效运维：ZYPT 工作项查询、缺陷创建/流转、迭代建议、Codeup MR 挂单。触发词：开缺陷 / ZYPT / 智衣云效运维 / bug create / bug transition / sprint current / 建 MR。"
metadata:
  requires:
    bins: ["yunxiao"]
  cliHelp: "yunxiao workitem +bug-create --help"
---

# 智衣云效运维（Zhiyi / ZYPT）

开始前先读 [`../yunxiao-shared/SKILL.md`](../yunxiao-shared/SKILL.md)。通用 Projex 见 [`../yunxiao-project/SKILL.md`](../yunxiao-project/SKILL.md)。

本 skill 面向 **智衣租户** 的缺陷工作流常量（状态机、必填字段、仓库别名、ZYPT 编号）。常量在 **profile JSON**，不硬编码进 CLI 默认值。

## 安装 profile

```bash
yunxiao profile install-example zhiyi
export YUNXIAO_PROFILE=zhiyi          # 或每次 --profile zhiyi
yunxiao profile show
```

也可手动：`cp profiles/zhiyi.example.json ~/.config/yunxiao/profiles/zhiyi.json`。

有 `organization_id` 时，若未设 `YUNXIAO_ORGANIZATION_ID` / `--organization-id`，会用 profile 的 org。

Wave 2 profile 额外字段：`repositories`、`bug_create_fields`（priority/serious_level 别名 + module/environment/ExpCompletionTime）、`allowed_environments`、`allowed_modules`、`default_assigned_to`。

## 产品主题与需求关联

产品主题与需求的关联使用 `ASSOCIATED`；创建主题/风险时不要带 `--sprint`（这类工作项通常未启用迭代字段）。正式空间先用 `--dry-run` 预览，再执行写操作。

`zhiyi` 的 `workitem_defaults` 会为产品类需求自动补齐默认字段，包括测试负责人、验收负责人（显式传值或 `--no-defaults` 可覆盖/跳过）。sandbox 回归使用 `--profile play`；`play` 与 `zhiyi` 的一行区别是：前者是沙箱配置，后者是智衣租户配置。

## 查工作项（ZYPT 或内部 id）

```bash
yunxiao workitem get ZYPT-<nnnn> --profile zhiyi
yunxiao workitem get --id ZYPT-<nnnn>
yunxiao workitem get --id <internalId>   # 无需 profile
```

成功时 `meta.resolved_id`（内部 id）、`meta.serial_number`（若有）与 `meta.url`（Projex 可点击链接）。

## 当前迭代建议（`sprint +current`）

搜索最近 10 条 Bug，按 sprint 出现频次给出候选与 `suggested`。

```bash
yunxiao sprint +current --profile zhiyi
yunxiao sprint +current --profile zhiyi --dry-run
```

Risk: **read**。

## 创建缺陷（`workitem +bug-create`）

```bash
# 未带 --sprint 时会先查最近迭代并报错提示（不创建）
yunxiao workitem +bug-create --profile zhiyi \
  --title "标题" --description "描述" --expected-completion 2026-09-20

# 预览
yunxiao workitem +bug-create --profile zhiyi \
  --title "标题" --description "描述" --expected-completion 2026-09-20 \
  --sprint <id> --verifier <userId|self> --dry-run

# 真发：write，需 --yes
yunxiao workitem +bug-create --profile zhiyi \
  --title "标题" --description "描述" --expected-completion 2026-09-20 \
  --sprint <id> --yes
```

默认：`--environment 测试环境`、`--module MES`、`--priority high`、`--serious-level normal`；`--assigned-to` 默认 profile `default_assigned_to`（空则 `self`）。

通用任意类型流转（无智衣命名必填 flags）见 `workitem +transition`（profile.`workflows`）。

## 缺陷流转（`+bug-transition`）

按实证状态图 **BFS 多步 PUT**；途经每一态的必填字段取 **并集**（不只看终点）。

| 途经状态 | 必填选项 |
|---|---|
| processing / testing | `--plan-due-date` `--developer` |
| deploy-test | `--responsible-person` `--bug-reason` `--bug-impact-scope` |

`confirm → testing` 必须五条都带。图内不可达（如已关闭→待确认）直接报错，不会单跳。

```bash
# 预览（推荐）
yunxiao workitem +bug-transition --profile zhiyi --id ZYPT-<nnnn> --to processing \
  --plan-due-date 2026-09-20 --developer <userId> --dry-run

# 真发：write 多步，需 --yes
yunxiao workitem +bug-transition --profile zhiyi --id ZYPT-<nnnn> --to processing \
  --plan-due-date 2026-09-20 --developer <userId> --yes
```

`--to` 可用别名：`confirm` / `processing` / `deploy-test` / `testing` / `deploy-prod` / `acceptance` / `fixed` / `regression` / `closed-fixed` / `reopen` / `deferred` / `wont-fix` / `cancelled-nofix` / `cancelled-wontfix` / `closed-unfixed`，或裸 statusId。

## 建 MR 挂单（`codeup mrs +create`）

不破坏既有 `codeup mrs create`；shortcut 支持仓库别名、WIP 标题、工作项解析。

```bash
yunxiao codeup mrs +create --profile zhiyi \
  --repo <repo-alias> --source feat/x --target master \
  --title "fix" --work-item ZYPT-<nnnn> --wip --dry-run

yunxiao codeup mrs +create --repo <repo-id> --source feat/x \
  --title "fix" --work-item ZYPT-<nnnn> --yes
```

Risk: **high-risk-write**（`--dry-run` / 真发需 `--yes`）。`--repo` 可为数字或 `<repo-alias>`（见 profile.`repositories`）；仅数字时可无 profile。


## `refresh_ok`（流转）

`workitem +bug-transition` / `+transition` 成功时信封可能含 `refresh_ok`。PUT 已成功；若随后刷新 GET 失败则为 `false` 并打 stderr warning——**不要当成流转失败**（见 yunxiao-shared）。

## 坑位

- A 旗标：`--dry-run` / `--yes`（**不是** B 的 `--no-dry-run`）。
- **date 自定义字段**（`plan_due_date`）发网为 ISO8601：`YYYY-MM-DDT00:00:00+08:00`。创建缺陷的 `ExpCompletionTime` 传 `YYYY-MM-DD`。
- 更新状态 PUT body 为摊平顶层 `{status, <fieldId>: value, …}`（与 `workitem update` 一致）。
- MR `workItemIds` 必须是内部 id；`+create` 会先 `workitem get` 再换。
- 写后用 `workitem get` 复核 `status.id`。

## 主线别名 → 状态（profile）

见 `profiles/zhiyi.example.json` 的 `bug_statuses` / `bug_edges` / `bug_transition_required` / `bug_create_fields` / `repositories`。

## +bug-create verifier (#77)

创建缺陷时必须带验证者（SOP）：

```bash
yunxiao workitem +bug-create --profile zhiyi   --title "…" --description "…" --expected-completion YYYY-MM-DD   --sprint <id> --verifier <userId|self> --yes
```

缺省顺序：`--verifier` → `profile.default_verifier` → `workitem_defaults[bug_type_id].verifier`。未设置会 stderr 告警。成功回显 `data.verifier`。
