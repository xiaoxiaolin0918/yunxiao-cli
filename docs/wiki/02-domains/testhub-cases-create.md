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

## meta.url / webPathHint（0.16.45+ / #361）

cases get / repos list / plans list / directories list 在已知 profile.space_id（或 plan 自带 spaceIdentifier）时拼 Projex 直链到 meta.url；否则给出 meta.webPathHint。

Agents 展示用例时优先给可点击 meta.url。
