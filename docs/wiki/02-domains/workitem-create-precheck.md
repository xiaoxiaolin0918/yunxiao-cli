# workitem create 必填字段预检（0.16.33+ / #95）

## 背景

Projex 创建接口一次只报**一个**缺失的必填字段（如先报「所属模块」，补上后再报「优先级」），agent 需要多轮试错。`workitem create` 现在在 POST 前用字段配置做一次性预检。

## 流程

1. 组装请求体：flag → `--*-file` → profile `workitem_defaults`（除非 `--no-defaults`）。
2. `GET /oapi/v1/projex/organizations/{org}/projects/{spaceId}/workitemTypes/{typeId}/fields`（只读，`--dry-run` 也发；`--no-precheck` 跳过）。
3. 对每个 `isRequired` 字段检查请求体：
   - 原生字段映射到根级 key：`subject`、`assignedTo`、`description`、`sprint`、`labels`/`tag`、`participants`、`trackers`、`verifier`、`versions`、`parentId`；
   - 其余字段看 `customFieldValues[fieldId]`（`pass_via="customFieldValues"`，经 `--custom-fields(-file)` 传入）。
4. 有缺失 → exit 1，不 POST；没有 → 照常 POST，`meta.precheck.status=ok`。

## 跳过规则（避免误报）

| 情况 | 处理 |
|------|------|
| 非必填 | 不查 |
| `showWhenCreate=false` | 不查（创建页不显示） |
| 服务端托管（status、creator、workitemType 等） | 不查 |
| 字段带非空 `defaultValue` | 不查（假设服务端会自动填充，未经实测） |
| 空字符串 / 纯空白 / 空数组 / null | 视为缺失 |
| 数字、布尔 | 视为已填 |

## 错误形状

```json
{"ok":false,"error":{"type":"cli","subtype":"missing_required_fields",
 "message":"workitem create precheck: 2 required field(s) missing for type <tid>: 所属模块 (<fid>), 优先级 (priority)",
 "hint":"pass custom fields via --custom-fields / --custom-fields-file as {\"<field_id>\":\"<option id or value>\"} (options in error.details.missing), system fields via the flag in pass_via; full config: yunxiao workitem fields --space-id <sid> --type-id <tid>; skip this check with --no-precheck",
 "details":{"space_id":"<sid>","type_id":"<tid>","required_checked":4,
  "missing":[{"field_id":"<fid>","name":"所属模块","format":"list","type":"NativeField","pass_via":"customFieldValues","options":[{"id":"…","display_value":"…"}],"options_total":3}]}}}
```

## 降级

字段配置 GET 失败（403/500/网络）或返回形状无法解析时：stderr 打印 warning，`meta.precheck={"status":"skipped","reason":"GET fields -> HTTP 403",…}`，创建继续——预检不应比没有预检更差。

## 已知限制 / 待确认

- 原生字段（如标签 tag/labels）的真实 field id 未全部实测；未映射的 id 按 `customFieldValues` 检查。
- `defaultValue` 是否真的由服务端自动填充未验证，目前直接跳过。
- `--dry-run` 现在需要凭证与网络来读字段配置（失败会降级，不阻塞预览）。
- `+bug-create` 暂未接入（仍依赖 profile `create_required` / `bug_create_fields`）。
