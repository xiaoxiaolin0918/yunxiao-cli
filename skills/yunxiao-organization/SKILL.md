---
name: yunxiao-organization
version: 1.0.3
description: "云效组织：当前用户、组织列表、成员、部门、角色。"
metadata:
  requires:
    bins: ["yunxiao"]
  cliHelp: "yunxiao organization --help"
---

# organization

开始前先读 [`../yunxiao-shared/SKILL.md`](../yunxiao-shared/SKILL.md)。

> List 响应 `meta` 可能含 `has_more` / `total` / `page`；`has_more==true` 时勿把首页当全集（见 yunxiao-shared）。

```bash
yunxiao organization +whoami
yunxiao organization list
yunxiao organization members list
yunxiao organization members list --include-aliyun-uid
yunxiao organization members search --query alice
yunxiao organization members search --query alice --include-aliyun-uid
yunxiao organization +member-resolve --name 崔健
yunxiao organization +member-resolve --name 崔健 --pick first
yunxiao organization departments list
yunxiao organization departments get --id <id>
yunxiao organization roles list
yunxiao organization roles get --id <id>
```

Risk: **read**.

## 按姓名解析成员 (#367)

现场建单常用中文姓名解析 `assignedTo`，不要全量 `members list` 再客户端过滤：

- `organization +member-resolve --name <显示名>`：精确匹配（trim / 大小写不敏感）；返回 `userId` / `name` / `email`
- 重名：列出候选并 exit ≠ 0；显式 `--pick first` 取第一个
- 原始模糊搜索仍用 `members search --query`
- 工作项 `--assigned-to <中文名>` 走同一解析器（ASCII 形如 userId 的值仍原样透传；`self` 不变）

细节见 [`../../docs/wiki/02-domains/organization-member-resolve.md`](../../docs/wiki/02-domains/organization-member-resolve.md)。

## ManualValidate / 阿里云 UID

Flow `ManualValidate`（`validatorType: users`）需要数字阿里云 UID，不是 hex `userId`。加 `--include-aliyun-uid`（需 `ALIBABA_CLOUD_ACCESS_KEY_ID`/`SECRET`）。概念见 `docs/wiki/02-domains/organization.md` 与 `pipeline.md`。
