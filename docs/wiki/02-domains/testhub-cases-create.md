# Testhub cases create / meta.url

## cases create：assignedTo 必填预校验（0.16.45+ / #357）

平台实际要求必须有负责人；缺 assignedTo 时 HTTP 400（「负责人不能为空」）。CLI 在 POST 前预校验：

```bash
yunxiao testhub cases create --repo-id <id> --subject "case" --assigned-to <userId> --dry-run
# 或用 --data JSON 内含 assignedTo
```

- 须传 `--assigned-to` 旗标，或 `--data` 里的 `assignedTo`
- 裸 create 响应通常只有 `{id}`；成功后 CLI 会 refresh GET，带回 customCode / subject
- repos list 返回 id 别名（= testRepoIdentifier）

## meta.url / webPathHint（#361，模板已在 #415 纠正）

`cases get` / `cases create`（refresh 后）/ `repos list` / `directories list` / `plans list` 的 `meta.url` / 资源 `url` 使用 TestHub 控制台路由，不再依赖 `profile.space_id`，也不再拼 `/projex/project/{spaceId}/testcase` 或 `/projex/project/{spaceId}/testplan/{planId}`（前者登录后白屏，后者登录后 404）：

- 单条用例：`https://devops.aliyun.com/testhub/case/{customCode}`。缺 `customCode` 时退到所属仓库页 `/testhub/repo/{repoId}/case`（内部 case id 暂无可靠官方 hash，目录链接为准）。
- 目录：`https://devops.aliyun.com/testhub/repo/{repoId}/case#selectedTreeIdentifier={directoryId}`
- 仓库：`https://devops.aliyun.com/testhub/repo/{repoId}/case`
- 测试计划：`https://devops.aliyun.com/testhub/plan/{planId}/case`。有 directoryId 时加 `#selectedTreeIdentifier={directoryId}`。不要用 `/projex/.../testplan/{planId}`（登录后 404）。

缺必要 id 时给出 `meta.webPathHint`。Agents 展示用例时优先给可点击 `meta.url`。`yunxiao browse testhub` 尚未提供。
