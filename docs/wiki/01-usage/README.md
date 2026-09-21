# 使用索引（人类）

> Flag 以 `yunxiao <cmd> --help` / `yunxiao schema <id>` 为准；此处只给路径与风险提示。

| 模块 | 入口 | 风险提示 |
|------|------|----------|
| 认证 | `yunxiao auth` | login 写本地凭证 |
| 健康检查 | `yunxiao doctor` / `whoami` | read |
| 打开控制台 | `yunxiao browse` | read；`--print-only` / `--dry-run` 只打印 URL |
| 本地别名 | `yunxiao alias` | set/list/delete；禁止别名内嵌 `--yes`/`-y` |
| 组织/项目 | `organization` / `project` / `+onboard` | 读多写少；删项目高风险 |
| 工作项 | `workitem` | 创建/更新/流转多为 write；高风险看 help |
| Codeup | `codeup`（`mrs` / `+open-mrs` / `+create`） | 创建 MR/保护分支等高风险需 `--yes` |
| 流水线 | `pipeline`（`+pending` / `+approve` / `run watch`） | 闸口/触发高风险 |
| AppStack / 制品 / 测试 | `appstack` / `packages` / `testhub` | 部署类高风险 |
| Skills | `yunxiao skills install` | 写本地 skills 目录 |
| 原始 API | `yunxiao api` | 按方法风险；先 `--dry-run` |
| Shell 补全 | `yunxiao completion bash|zsh|powershell` | 见本页下方 |

## Shell 补全

```bash
# bash
yunxiao completion bash > /etc/bash_completion.d/yunxiao
# zsh
yunxiao completion zsh > "${fpath[1]}/_yunxiao"
```

```powershell
# Windows PowerShell（当前会话）
yunxiao completion powershell | Out-String | Invoke-Expression
# 持久化：写入 $PROFILE
```

## 相关

- 从 GitHub CLI / 公共 npm `yx` 迁移：[../00-process/gh-yx-migration.md](../00-process/gh-yx-migration.md)
- yx 对照规划：[../00-process/yx-parity-ux-plan.md](../00-process/yx-parity-ux-plan.md)
- Agent 入口：[../../../AGENTS.md](../../../AGENTS.md)
