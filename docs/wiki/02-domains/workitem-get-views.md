# workitem get 输出视图（0.16.34+ / #98）

## 为什么改默认

GetWorkitem 的 `description` 常有数千字（2026-09-29 查 ZYPT-5916 时一次刷屏），而 agent 多数时候只看状态 / 负责人 / 迭代。与 `workitem create`（#62）、`codeup mrs create/update` 一致：**默认 brief，`--full` 取原始对象**。

## 三种视图（互斥）

| 视图 | `data` | `meta.projection` |
|------|--------|-------------------|
| 默认 / `--brief` | `id`、`serialNumber`、`subject`、`status {id, displayName}`、`assignedTo`、`sprint`、`priority`、`workitemType {id, name}`、`categoryId`、`gmtModified`、`description_summary` | `brief` |
| `--full` | 原始 GetWorkitem 对象（与 0.16.33 相同，含 `meta.pagination*` 等） | 无 |
| `--fields a,b,c` | 仅这些顶层字段的原始值 | `fields` |

- brief 中值为 `null` 或空字符串 `""` 的字段省略（顶层 `priority: ""` 同样省略 / 回退派生）。
- `priority`：顶层 `priority` 非 null 且非 `""` 时原样使用；否则在 `customFieldValues` 中找 `fieldId == "priority"` **或** `fieldName` 为 `优先级` / `Priority`（大小写不敏感；自定义字段 id 常是哈希）的条目，依次跳过空条目（无 values、值为空），取第一个非空值 → `{id, displayValue}`。`id` 取值对象的 `identifier`，没有时取 `id`；两者都没有就不输出 `id`（不会出现 `{"id": null}`）。brief 与 `--fields priority` 用同一规则。
  - 派生失败：brief 省略 `priority`；`--fields priority` 输出 `null`，`meta.hint` 提示用 `--full --jq '.data.customFieldValues'` 或 `yunxiao workitem fields` 核对字段（exit 0，不报 `unknown_fields`）。
- `description_summary`：`(description: <n> chars, use --full or --fields description)`，n 为字符数（rune）。非 MARKDOWN（RICHTEXT 即 HTML）且含标签时，先去掉 HTML 标签、解码实体再计数，文案为 `<n> chars of text excluding HTML tags`；MARKDOWN 按原文计数。description 不是字符串时给出 `(description: non-string <object|array|number|boolean> value, …)`；为空、null 或缺失时省略。
- 三种视图都保留 `meta.url` / `serial_number` / `resolved_id`。

## `--fields` 名字

- 该工作项上存在的顶层字段：原样输出（存在但为 `null` 的保留 `null`）。
- 官方 GetWorkitem 返回结构中的字段（`assignedTo`、`categoryId`、`creator`、`customFieldValues`、`description`、`formatType`、`gmtCreate`、`gmtModified`、`id`、`idPath`、`labels`、`logicalStatus`、`modifier`、`parentId`、`participants`、`serialNumber`、`space`、`sprint`、`status`、`statusStageId`、`subject`、`trackers`、`updateStatusAt`、`verifier`、`versions`、`workitemType`）以及 `priority`，但该工作项没有（如未排迭代没有 `sprint`）：输出 `null`，名字列入 `meta.absent_fields`。
- 其他名字才报 `unknown_fields`。

## 兼容开关 `YUNXIAO_WORKITEM_GET_VIEW`

- 取值 `full` / `brief`（去空格、大小写不敏感；空串视为未设）。**优先级：flag > env > 默认 brief**，即任一 `--full` / `--brief` / `--fields` 都覆盖它。
- 用途：脚本需要同时在 0.16.34 之前和之后的 CLI 上跑、又要原始输出时，设 `YUNXIAO_WORKITEM_GET_VIEW=full` 而不是传 `--full`（旧 CLI 不认识 `--full` 会报错，但会忽略这个环境变量）。
- 其他取值在请求前报错：exit 1，`type:"cli"`，`subtype:"invalid_env"`（有 view flag 时不检查）。
- `--dry-run` 的 `request.projection` 带 `source`（`flag` / `env` / `default`）；env 选中 full 时为 `{mode:"full", source:"env"}`（`--full` flag 仍不加该字段）。

## 错误路径

| 情况 | 时机 | 结果 |
|------|------|------|
| `--full` / `--brief` / `--fields` 同时给出两个以上 | 请求前（含 `--dry-run`） | exit 1，`type:"cli"`，`mutually exclusive` |
| `--fields ""`、`a,,b`、`status.id`、非法字符 | 请求前（含 `--dry-run`） | exit 1，`type:"cli"`（嵌套路径提示用 `--jq`） |
| `YUNXIAO_WORKITEM_GET_VIEW` 非 full / brief 且无 view flag | 请求前（含 `--dry-run`） | exit 1，`subtype:"invalid_env"` |
| `--fields` 中的名字既不在返回中也不是 GetWorkitem 字段（大小写敏感） | GET 之后 | exit 1，`subtype:"unknown_fields"`，`details.unknown` / `available` / `suggestions`，stdout 为空 |
| `--fields` 时 API 返回数组或 null | GET 之后 | exit 1，`subtype:"non_object_response"`，提示用 `--full` 查看；stdout 为空（brief / `--full` 仍原样透传） |
| API 错误 | GET | 原样透传 `type:"api"` + 状态码 |

## 与 --jq / 脚本

`--jq` 作用于投影后的信封。旧脚本若读取 `.data.description`、`.data.customFieldValues` 等，需加 `--full`（或 `--fields description,customFieldValues`），或设 `YUNXIAO_WORKITEM_GET_VIEW=full`。`--dry-run` 在 `request.projection` 显示将使用的视图。

## 测试

`cmd/workitem_get_view_test.go` 用 0.16.33 的 `Run`（经 `runRead`，响应带分页头）与 `--full` / env=full 逐字节比对；`cmd/testdata/workitem_get/*.golden.json` 固定各视图 stdout（`legacy` 与 `full` 必须相同），用 `go test ./cmd -run TestWorkitemGetGolden -update-workitem-get-golden` 重新生成。
