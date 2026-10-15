---
name: yunxiao-testhub
version: 1.0.1
description: "云效 Testhub：测试计划/结果/计划用例评论。用户问测试计划、执行结果、用例评论时使用。"
metadata:
  requires:
    bins: ["yunxiao"]
  cliHelp: "yunxiao testhub --help"
---

# testhub

开始前先读 [`../yunxiao-shared/SKILL.md`](../yunxiao-shared/SKILL.md)。

> List 响应 `meta` 可能含 `has_more` / `total` / `page`；`has_more==true` 时勿把首页当全集（见 yunxiao-shared）。

```bash
yunxiao testhub plans list --project-id <projectId>
yunxiao testhub plans progress --plan-id <planId>
yunxiao testhub plans directories --plan-id <planId>
yunxiao testhub results list --plan-id <planId> --directory-id <dirId>
yunxiao testhub results update --plan-id <planId> --testcase-id <id> --status PASSED --dry-run
yunxiao testhub plan-comments list --plan-id <planId> --testcase-id <id>   # newest first; --sort asc
yunxiao testhub plan-comments create --plan-id <planId> --testcase-id <id> --content "note" --dry-run
yunxiao testhub repos list
```

Risk: list*=**read**；`results update` / `plan-comments create`=**write**。

## Cases / directories

```bash
yunxiao testhub directories list --repo-id <id>
yunxiao testhub directories create --repo-id <id> --name folder --dry-run
yunxiao testhub cases search --repo-id <id>
yunxiao testhub cases get --repo-id <id> --id <caseId>   # meta.url when profile.space_id set (#361)
yunxiao testhub cases create --repo-id <id> --subject "case" --assigned-to <userId> --dry-run
```

**cases create (#357)**：`assignedTo` 实务必填（缺则平台 HTTP 400「负责人不能为空」）——用 `--assigned-to` 或 `--data` 里的 `assignedTo`；CLI 会预校验。成功响应会 refresh GET，带回 `customCode` / `subject`（裸 create 只回 `{id}`）。

`--data` 提示：`testSteps` 可用 `{"contentType":"TABLE","content":[{"step":"...","expected":"..."}]}`；`preCondition` 建议 RICHTEXT（`<article class="4ever-article">...</article>`）或 `preConditionFormat: TEXT`。

**meta.url (#361)**：`cases get` / `repos list` / `plans list` / `directories list` 在已知 `profile.space_id`（或 plan 自带 `spaceIdentifier`）时拼 Projex 直链；否则 `meta.webPathHint`。`repos list` 另补 `id` 别名（= `testRepoIdentifier`）。
