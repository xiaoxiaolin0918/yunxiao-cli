# 工作项评论：OAPI 与 OpenAPI RPC（#69 / #71）

## 结论（yunxiao-cli 个人令牌 OAPI）

| 操作 | OAPI 路径 | CLI |
|------|-----------|-----|
| 列表 | `GET /oapi/v1/projex/organizations/{org}/workitems/{id}/comments` | `workitem comments list` |
| 创建 | `POST .../workitems/{id}/comments` body `{content}` | `workitem comment` |
| 删除 | **无文档**；`DELETE .../comments/{commentId}` 实测 404 | **不走 OAPI** |
| 更新 | **无文档** | **不走 OAPI** |

官方 OAPI 目录「工作项评论」仅列出 [ListWorkitemComments](https://help.aliyun.com/zh/yunxiao/developer-reference/listworkitemcomments) 与 [CreateWorkitemComment](https://help.aliyun.com/zh/yunxiao/developer-reference/createworkitemcomment)。

## AccessKey OpenAPI RPC（已接线）

与 CLI 的 `x-yunxiao-token` OAPI **不是同一接入面**。删除/更新复用与 `organization members list --include-aliyun-uid` 相同的 AccessKey ACS3 签名路径（`orguid.DevOpsMembersClient` / `SignACS3` / `DoROA`）：

| 操作 | RPC | CLI |
|------|-----|-----|
| 删除单条 | `POST /organization/{organizationId}/workitems/deleteComent`（官方路径拼写缺 `m`） | `workitem comments delete --id … --comment-id … --yes`（`--dry-run` 可预览） |
| 更新 | `POST /organization/{organizationId}/workitems/commentUpdate` | `workitem comments update --id … --comment-id … --content\|--content-file` |
| 删除全部 | `DELETE .../workitems/deleteAllComment` | **未封装** |

OpenAPI：
- [DeleteWorkitemComment](https://help.aliyun.com/zh/yunxiao/developer-reference/api-devops-2021-06-25-deleteworkitemcomment) body: `identifier`, `commentId`
- [UpdateWorkitemComment](https://help.aliyun.com/zh/yunxiao/developer-reference/api-devops-2021-06-25-updateworkitemcomment) body: `content`, `formatType`, `workitemIdentifier`, `commentId`

### 凭证

```bash
export ALIBABA_CLOUD_ACCESS_KEY_ID=...
export ALIBABA_CLOUD_ACCESS_KEY_SECRET=...
# 可选：ALIBABA_CLOUD_REGION_ID（默认 cn-hangzhou）
```

企业 ID 仍由个人令牌客户端 `ResolveOrgID`（`YUNXIAO_ORGANIZATION_ID` / profile）解析。`--id` 可为工作项 identifier 或流水号；流水号会先经 OAPI GET 解析为 identifier。

缺 AccessKey 时 CLI 会明确报错并指向上述环境变量（与 `--include-aliyun-uid` 同源）。

### 风险

- `comments delete`：`high-risk-write`（实跑需 `--yes`）
- `comments update`：`write`

## Windows PowerShell 中文乱码

`--content "中文"` 常被本机代码页弄乱。请写入 **UTF-8** 文件（可带 BOM；CLI 会去掉 BOM）后：

```bash
yunxiao workitem comment --id <id> --content-file ./note.md --dry-run
yunxiao workitem comments update --id <id> --comment-id <cid> --content-file ./note.md --dry-run
```
