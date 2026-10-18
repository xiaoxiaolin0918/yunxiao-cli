# 组织成员按姓名解析 (#367)

## 背景

现场建单（缺陷/风险/需求）常需按中文姓名解析 `assignedTo`。全量 `organization members list` 再客户端过滤成本高，且易误挂非业务账号。

## 命令

```bash
yunxiao organization +member-resolve --name 崔健
yunxiao organization +member-resolve --name 崔健 --pick first
```

- HTTP：`POST .../members:search`（与 `members search --query` 同一接口）
- 客户端做**精确**姓名匹配（trim、大小写不敏感）；不模糊猜测
- 成功：`data` 含 `userId` / `name` / `email`（无邮箱则为空串）
- 0 命中或未加 `--pick first` 的重名：stderr 错误 + exit ≠ 0；重名错误会列出候选 id

原始分页/模糊搜索仍用：

```bash
yunxiao organization members search --query <q>
```

## 与 `--assigned-to` 打通

`workitem create` / `update` / `+bug-create` / `+risk-create` / `+req-create` 的 `--assigned-to`：

| 输入 | 行为 |
|------|------|
| `self` | 当前用户 id |
| ASCII id 形（字母数字 `_-`） | 原样透传（兼容旧用法） |
| 含中文等显示名 | 与 `+member-resolve` 同一精确匹配；唯一命中用其 userId；重名/无命中报错 |

`+risk-create` / `+req-create` 的 `--assignee` 仍可用（同解析器，#128）；与 `--assigned-to` 二选一。

## 相关

- skill：`skills/yunxiao-organization/SKILL.md`
- 实现：`cmd/member_resolve.go`（`resolveMemberByName` / `resolveAssignedToRef`）
