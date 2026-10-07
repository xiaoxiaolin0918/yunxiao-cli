# workitem create 必填字段预检（0.16.33+ / #95）

## 背景

Projex 创建接口一次只报**一个**缺失的必填字段（如先报「所属模块」，补上后再报「优先级」），agent 需要多轮试错。`workitem create` 现在在 POST 前用字段配置做一次性预检。

## 流程

1. 组装请求体：flag → `--*-file` → profile `workitem_defaults`（除非 `--no-defaults`）。
2. `GET /oapi/v1/projex/organizations/{org}/projects/{spaceId}/workitemTypes/{typeId}/fields`（只读，`--dry-run` 也发；`--no-precheck` 跳过）。
3. 对每个 `isRequired` 字段检查请求体：
   - 根级字段（`subject`、`assignedTo`、`description`、`sprint`、`labels`/`tag`、`participants`、`trackers`、`verifier`、`versions`、`parentId`）**只认根级 key**（即对应 flag，`pass_via` 给出 flag）；写进 `customFieldValues` 不算已填；
   - 其余字段（包括 `priority`、所属模块等 SystemCustomField / CustomField）**只认** `customFieldValues[fieldId]`（`pass_via="customFieldValues"`，经 `--custom-fields(-file)` 传入）。
4. 有缺失 → exit 1，不 POST；`details.missing[]` 按字段配置返回的顺序排列。没有缺失 → 照常 POST，`meta.precheck={status:"ok",source:"fields",required_checked,skipped_default?}`。

## 跳过规则（避免误报）

| 情况 | 处理 |
|------|------|
| 非必填 | 不查 |
| `showWhenCreate=false` | 不查（创建页不显示） |
| 服务端托管（status、creator、workitemType 等） | 不查 |
| 字段带非空 `defaultValue` | 不查（假设服务端会自动填充，未经实测），但 id 列在 `meta.precheck.skipped_default` |
| 空字符串 / 纯空白 / 空数组 / null | 视为缺失 |
| 数字、布尔 | 视为已填 |

## 错误形状

```json
{"ok":false,"error":{"type":"cli","subtype":"missing_required_fields",
 "message":"workitem create precheck: 2 required field(s) missing for type <tid>: 所属模块 (<fid>), 优先级 (priority)",
 "hint":"pass custom fields via --custom-fields / --custom-fields-file as {\"<field_id>\":\"<option id or value>\"} (options in error.details.missing), system fields via the flag in pass_via; full config: yunxiao workitem fields --space-id <sid> --type-id <tid>; skip this check with --no-precheck",
 "details":{"space_id":"<sid>","type_id":"<tid>","required_checked":4,
  "missing":[{"field_id":"<fid>","name":"所属模块","format":"list","type":"CustomField","pass_via":"customFieldValues","options":[{"id":"…","display_value":"…"}],"options_total":3}]}}}
```

`type` 取自字段配置（所属模块是 CustomField → `pass_via="customFieldValues"`；只有根级映射表中的字段才会给出 flag）。`missing[]` 与 message 中的顺序即字段配置的返回顺序。

## 降级与告警

| 情况 | 结果 |
|------|------|
| 字段配置 GET 返回 401 | **失败**（`type:"api"` 401，不 POST）——凭证问题不降级 |
| 其他 HTTP 错误 / 网络 / 返回形状无法解析 | `status:"skipped"`，创建继续 |
| 返回空列表 | `status:"empty"`（不是 `ok` + `required_checked:0`），创建继续 |

- 重试上限：该 GET 最多重试 1 次，每次退避 ≤1s（含 `Retry-After`），不会像默认 GET 策略那样卡约 90s。
- 整个字段配置读取（两次尝试 + 退避）有 10s 总超时（`context.WithTimeout`），服务端接了连接却不应答时 10s 内降级为 `skipped`（`reason`：`GET fields timed out after 10s`），而不是每次尝试等 HTTP 客户端超时（约 2 分钟）。
- 告警两处都有（与 `browse`、`mrs +create` 关联告警的约定一致）：stderr 打印一行 `warning: <warning>`，同时写在 `meta.precheck`（dry-run：`request.precheck`）的 `warning` / `reason` / `hint` 字段里。stdout 仍只有 JSON 信封。
- 预检为 `skipped` / `empty` 而随后 POST 失败时，`meta.precheck` 不会输出，因此把 `precheck skipped: <reason>`（或 `precheck empty: <reason>`）追加到错误信封的 `error.hint`，便于判断缺字段是否因为预检没跑。
- profile 兜底：若 profile 有 `workitem_defaults[<type_id>].create_required`，降级 / 空配置时改为按这些 id 检查（同样的根级 / customFieldValues 规则），`source:"profile_fallback"`，未提供的 id 列在 `profile_missing[]` 并写进 `warning`——**只告警、不阻塞**。没有该配置时 `source:"none"`。

预检不应比没有预检更差。

## 已知限制 / 待确认

- 原生字段（如标签 tag/labels）的真实 field id 未全部实测；未映射的 id 按 `customFieldValues` 检查。
- `defaultValue` 是否真的由服务端自动填充未验证，目前直接跳过。
- `--dry-run` 现在需要凭证与网络来读字段配置（401 失败，其他失败降级，不阻塞预览）；离线请用 `--no-precheck`。
- 发版前需在 play 沙箱实测：`workitem fields` 的键名 / 原生字段 id，以及一次带齐必填字段的真实 create；ZYPT 上只允许 `--dry-run`。
- `+bug-create` 已接入同一套预检（#107）；`--no-precheck` 可跳过；`+risk-create` / `+req-create`（#128）同样走该预检。`--minimal` / profile `bug_create_fields` 决定的 body 同样参与。
