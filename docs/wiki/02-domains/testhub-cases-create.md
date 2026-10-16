# Testhub cases create / meta.url

## cases create：assignedTo 必填预校验（0.16.45+ / #357）

平台实务要求用例负责人：缺 ssignedTo 时 HTTP 400「负责人不能为空」。CLI 在 POST 前预校验：

`ash
yunxiao testhub cases create --repo-id <id> --subject "case" --assigned-to <userId> --dry-run
# 或在 --data JSON 里带 assignedTo
`

- 可用 --assigned-to 旗标，或 --data 中的 ssignedTo
- 裸 create 响应通常只有 {id}；成功后 CLI 会 refresh GET，带回 customCode / subject
- 
epos list 另补 id 别名（= 	estRepoIdentifier）

## meta.url / webPathHint（0.16.45+ / #361）

cases get / 
epos list / plans list / directories list 在已知 profile.space_id（或 plan 自带 spaceIdentifier）时拼 Projex 直链到 meta.url；否则回落 meta.webPathHint。

Agents 展示结果时优先附可点击 meta.url。
