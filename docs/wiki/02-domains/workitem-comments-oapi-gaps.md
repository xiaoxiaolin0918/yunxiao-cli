# 工作项评论：OAPI 与 OpenAPI RPC 差异（#69）

## 结论（yunxiao-cli 使用的个人令牌 OAPI）

| 操作 | OAPI 路径 | CLI |
|------|-----------|-----|
| 列表 | `GET /oapi/v1/projex/organizations/{org}/workitems/{id}/comments` | `workitem comments list` |
| 创建 | `POST .../workitems/{id}/comments` body `{content}` | `workitem comment` |
| 删除 | **无文档**；`DELETE .../comments/{commentId}` 实测 404 | **不封装** |
| 更新 | **无文档** | **不封装** |

官方 OAPI 目录「工作项评论」仅列出 [ListWorkitemComments](https://help.aliyun.com/zh/yunxiao/developer-reference/listworkitemcomments) 与 [CreateWorkitemComment](https://help.aliyun.com/zh/yunxiao/developer-reference/createworkitemcomment)。

## 另一套：阿里云 OpenAPI RPC（AccessKey / SDK）

与 CLI 的 `x-yunxiao-token` OAPI **不是同一接入面**：

| 操作 | RPC | 备注 |
|------|-----|------|
| 删除单条 | `POST /organization/{organizationId}/workitems/deleteComent`（官方路径拼写缺 `m`） | [DeleteWorkitemComment](https://help.aliyun.com/zh/yunxiao/developer-reference/api-devops-2021-06-25-deleteworkitemcomment) body: `identifier`, `commentId` |
| 更新 | `POST /organization/{organizationId}/workitems/commentUpdate` | [UpdateWorkitemComment](https://help.aliyun.com/zh/yunxiao/developer-reference/api-devops-2021-06-25-updateworkitemcomment) |
| 删除全部 | `DELETE .../workitems/deleteAllComment` | DeleteWorkitemAllComment |

yunxiao-cli **不**用 AccessKey 签名调 RPC，故不提供 `workitem comments delete|update`。需要删评时用控制台，或自建 AccessKey SDK 调用。

## Windows PowerShell 中文乱码

`--content "中文"` 常被本机代码页弄乱。请写入 **UTF-8** 文件（可带 BOM；CLI 会去掉 BOM）后：

```bash
yunxiao workitem comment --id <id> --content-file ./note.md --dry-run
```
