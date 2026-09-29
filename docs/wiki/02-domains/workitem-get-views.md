# workitem get 输出视图（0.16.34+ / #98）

## 为什么改默认

GetWorkitem 的 `description` 常有数千字（2026-09-29 查 ZYPT-5916 时一次刷屏），而 agent 多数时候只看状态 / 负责人 / 迭代。与 `workitem create`（#62）、`codeup mrs create/update` 一致：**默认 brief，`--full` 取原始对象**。

## 三种视图（互斥）

| 视图 | `data` | `meta.projection` |
|------|--------|-------------------|
| 默认 / `--brief` | `id`、`serialNumber`、`subject`、`status {id, displayName}`、`assignedTo`、`sprint`、`priority`、`gmtModified`、`description_summary` | `brief` |
| `--full` | 原始 GetWorkitem 对象（与 0.16.33 相同） | 无 |
| `--fields a,b,c` | 仅这些顶层字段的原始值 | `fields` |

- `priority`：GetWorkitem 没有顶层 priority，取 `customFieldValues[fieldId="priority"].values[0]` → `{id, displayValue}`；若 API 返回顶层 `priority` 则原样使用。
- `description_summary`：`(description: <字符数> chars, use --full or --fields description)`；description 为空或缺失时省略。
- 三种视图都保留 `meta.url` / `serial_number` / `resolved_id`。

## 错误路径

| 情况 | 时机 | 结果 |
|------|------|------|
| `--full` / `--brief` / `--fields` 同时给出两个以上 | 请求前（含 `--dry-run`） | exit 1，`type:"cli"`，`mutually exclusive` |
| `--fields ""`、`a,,b`、`status.id`、非法字符 | 请求前（含 `--dry-run`） | exit 1，`type:"cli"`（嵌套路径提示用 `--jq`） |
| `--fields` 中的名字在返回中不存在（大小写敏感） | GET 之后 | exit 1，`subtype:"unknown_fields"`，`details.unknown` / `available` / `suggestions`，stdout 为空 |
| API 错误 | GET | 原样透传 `type:"api"` + 状态码 |

## 与 --jq / 脚本

`--jq` 作用于投影后的信封。旧脚本若读取 `.data.description`、`.data.customFieldValues`、`.data.workitemType` 等，需加 `--full`（或 `--fields description,customFieldValues`）。`--dry-run` 在 `request.projection` 显示将使用的视图。
