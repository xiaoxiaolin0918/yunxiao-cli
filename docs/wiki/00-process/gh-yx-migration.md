# 从 GitHub CLI / 公共 npm `yx` 迁移到 `yunxiao`

公共 npm 包 `yunxiao-cli`（命令 **`yx`**，AndersonBY）与本仓库 Go CLI（命令 **`yunxiao`**）**不是同一产品**。组织 Agent/CI 请用本仓库。

| gh / `yx` | `yunxiao` | 备注 |
|-----------|-----------|------|
| `gh pr` / `yx pr` | `yunxiao codeup mrs` · `codeup +open-mrs` · `codeup mrs +create` | 创建 MR 高风险需 `--yes`；work item 挂不上则 ok=false（≥0.16.12） |
| `gh issue` / `yx issue` | `yunxiao workitem`（search/get/create/…） | Projex；不是 GitHub Issues |
| `gh repo` / `yx repo` | `yunxiao codeup repos` | |
| `yx workflow` / `yx run` | `yunxiao pipeline` · `pipeline run` · `+pending` / `+approve` | 本仓库流水线更深 |
| `yx browse` | `yunxiao browse` | pipeline / workitem / mr / repo / url；`--print-only` |
| `yx status` | （缺口）可用 `+pending`、`+open-mrs`、`workitem search` 组合 | 见 Known gaps |
| `yx search` | 各域 `list` / `workitem search` | 无统一 search 入口 |
| `yx alias` | （缺口）暂用 shell alias；二进制 alias 见 issue #38 | 别名不得绕过 `--yes` |
| `yx auth login --token` | `yunxiao auth login --browser` 或 `--token` | 推荐 OAuth |
| 配置 `~/.yx` | `~/.config/yunxiao/` + profiles | 多租户用 `--profile` |

## 安全契约（不要从 `yx`「学回去」）

- 全局 `--dry-run` 预览请求
- 高风险写操作需用户确认后的 `--yes`
- 失败不要伪装 `ok=true`（例如 MR 未挂上工作项）

## 安装

- 主路径：[GitHub Releases latest](https://github.com/xiaoxiaolin0918/yunxiao-cli/releases/latest)
- npm 名拟 `sanzhi-yunxiao-cli`（薄包装拉二进制）；**不要** `npm i -g yunxiao-cli`（那是别人的 `yx`）
