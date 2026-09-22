语言：中文 | [English](README.md)

# yunxiao-cli

云效 CLI，对标飞书 / Lark CLI：渐进发现、`+shortcuts`、类型化 API 命令、原始 `api` 逃生舱、风险门禁与 Agent skills。

CLI 二进制名：**`yunxiao`**。

## 人类 30 秒快速开始

1. 安装（主路径）：打开 [https://github.com/xiaoxiaolin0918/yunxiao-cli/releases/latest](https://github.com/xiaoxiaolin0918/yunxiao-cli/releases/latest)，下载对应平台归档，把 `yunxiao` 加入 PATH。
2. （可选）npm 薄包装：`npm i -g sanzhi-yunxiao-cli`（下载 GitHub Release 二进制；尚未 publish 时用仓库 `npm/` 本地装）。
3. 登录：`yunxiao auth login --browser`（CI 用 `--token`）。
4. 试跑：`yunxiao whoami` · `yunxiao doctor` · `yunxiao codeup +open-mrs`。
5. 打开控制台页：`yunxiao browse pipeline --pipeline-id <id> --print-only`。
6. Shell 补全：`yunxiao completion powershell | Out-String | Invoke-Expression`（bash/zsh 见 [usage 索引](docs/wiki/01-usage/README.md)）。

写操作先 `--dry-run`；高风险确认后再加 `--yes`。

详细模块说明：[docs/wiki/01-usage/README.md](docs/wiki/01-usage/README.md) · 从 gh/`yx` 迁移：[docs/wiki/00-process/gh-yx-migration.md](docs/wiki/00-process/gh-yx-migration.md)

---

## 面向 AI Agent

将以下内容粘贴给 AI Agent（安装 → 认证 → 安装 skills → 只读列项目 → 由用户选择项目 → **仅本机**初始化 profile）：

```text
请帮我安装并初始化 yunxiao CLI（本地 profile）：

1) 安装（主路径：GitHub Releases）：
   https://github.com/xiaoxiaolin0918/yunxiao-cli/releases/latest
   按用户系统下载归档、解压，把 `yunxiao` 加入 PATH。
   yunxiao --version   # 应与 GitHub latest Release 一致

2) 认证（优先浏览器 OAuth；无图形界面再用 PAT。禁止把完整 token 打到回复/聊天里）
   推荐：yunxiao auth login --browser
   注意：OAuth 同意 = 账号 API 全能力（平台不按模块限权，宽于细粒度 PAT）。
   登录后探测：yunxiao auth probe-oauth
   PAT 回落（CI/无浏览器）：
     控制台：https://account-devops.aliyun.com/settings/personalAccessToken
     帮助：https://help.aliyun.com/zh/yunxiao/user-guide/personal-access-token
     yunxiao auth login --token "<PAT>"
   yunxiao whoami && yunxiao doctor && yunxiao auth status

3) 安装 companion skills：
   yunxiao skills install

4) 只读列项目，请用户挑选一个 space_id / 项目：
   yunxiao project list

5) 仅为所选项目初始化 **本地** profile（写入 ~/.config/yunxiao/profiles/）：
   推荐交互：yunxiao +onboard
   或非交互：yunxiao +onboard --space-id <id> --profile <name>

6) 校验：
   export YUNXIAO_PROFILE=<name>
   yunxiao profile show
   yunxiao profile doctor
   yunxiao doctor

7) 风险规则：写操作先 --dry-run；高风险需用户确认后再加 --yes；长 JSON 用 --data-file。
```

## 快速开始

### 1. 安装

```bash
# 主路径：从 GitHub Releases 下载对应平台归档，解压后把 yunxiao 加入 PATH
# https://github.com/xiaoxiaolin0918/yunxiao-cli/releases/latest

yunxiao --version   # 应与 GitHub latest Release 一致
```

### 2. 认证

优先使用浏览器 OAuth；仅在 CI/无图形界面时使用 PAT（不要把完整 token 打到回复/聊天里）：

```bash
yunxiao auth login --browser
yunxiao auth probe-oauth
yunxiao whoami && yunxiao doctor
```

PAT 回落：打开 [个人访问令牌控制台](https://account-devops.aliyun.com/settings/personalAccessToken) 新建 PAT（名称建议 `yunxiao-cli`；勾选组织读 + 项目/代码/流水线读写，按需制品/测试/应用；令牌只显示一次），然后：

```bash
yunxiao auth login --token "<PAT>"
```

### 3. 常用只读命令

```bash
yunxiao organization +whoami
yunxiao pipeline list
yunxiao codeup repos list
```

### 4. 使用提示

- 写操作先 `--dry-run`；高风险要确认后再加 `--yes`
- 长 JSON 用 `--data-file ./body.json`
- Skills 向导可多选；也可 `yunxiao skills install --skill ...`

## CLI 与 MCP 对比

| 维度 | `yunxiao-cli` | Yunxiao/Alibaba Cloud DevOps MCP |
|------|---------------|----------------------------------|
| 形态 | 本地命令行可执行程序 | 向 MCP 客户端提供工具的 MCP 服务器 |
| 典型用途 | 脚本、CI、终端和可复制命令 | IDE 或聊天 Agent 中的对话式工作流 |
| 安装 | 下载发布版二进制或从源码构建；需要时另行安装 skills | 在支持 MCP 的客户端中配置 MCP 服务器 |
| 认证 | 使用 CLI profile、环境变量、PAT 或浏览器 OAuth | 通过 MCP 服务器和客户端的配置管理凭证与授权 |
| 发现能力 | 通过 `--help`、`schema`、类型化命令、`+快捷命令` 和 companion skills 发现 | 由 MCP 客户端展示工具目录和输入参数 schema |
| 输出 | stdout/stderr、结构化 JSON、退出码，以及 `--jq` 等 Shell 过滤 | 由客户端呈现结构化工具结果 |
| 写操作安全 | 明确支持 `--dry-run`；高风险写操作在确认后才需要 `--yes` | 取决于工具和 MCP 客户端的确认控制，不提供 CLI 统一的参数门禁 |
| 可复现性 | 命令可复制、版本化、编排进脚本并审计 | 更依赖客户端上下文和设置，审计性与可复现性通常弱于 CLI |
| IDE 依赖 | 无 | 需要支持 MCP 的 IDE、Agent 或其他客户端 |

脚本、CI 和可复制命令使用 CLI；IDE 内聊天使用 MCP；许多团队会同时使用两者。

**质量周报 / 按日期窗口出数**（日期窗口 → Req/Bug → JSON → 脚本）：优先用类型化 CLI（`workitem search` 的 `--created-after` / `--created-before`、`--updated-*`、`--finish-*`，以及 `--status` / `--status-stage`，加 `--all` 跟页，可选 `--as-items` 让 `data` 变为 `{items, pagination}`）。已在 IDE Agent 里对话时再用 MCP 作回落。按 `finishTime` 写入 conditions 过滤可能可用；oapi SearchWorkitems / get 响应不含 `finishTime`（schema 列的是 `gmtCreate` / `gmtModified` / `updateStatusAt`）——CLI **不会**用 `updateStatusAt` 伪造或 enrich `finishTime`。OpenAPI `perPage` 上限 200：看 `meta.total` / `has_more` / `--all`，不要只看 `len(data)`。服务端日期条件仍可能返回窗口外行——脚本请按需对 `gmtCreate` / `gmtModified` / `customFieldValues` 做客户端过滤。类型化 `workitem search` 与原始 `api POST …/workitems:search` 均为 **read**（不需要 `--yes`）。原始 `api` 若在顶层传入 MCP 风格的 `createdAfter` / `updatedAfter` / `finishTimeAfter` 等，会规范化进官方 `conditions`（见 `meta.request`）。

### MCP → CLI 对照（周报 / 发现）

| MCP 风格意图 | CLI |
|--------------|-----|
| `search_workitems` + `createdAfter` / `createdBefore` | `yunxiao workitem search --created-after … --created-before …`（可加 `--all`、可选 `--as-items`） |
| 更新 / 完成时间窗口 | `--updated-after/before`、`--finish-after/before` |
| 工作项评论 | `yunxiao workitem comments list --id <id>` |
| 组织 / 项目（空间）列表 | `yunxiao organization list`、`yunxiao project list` |
| 查看 search 参数 | `yunxiao schema workitem.search` |
| 响应中的 `finishTime` | **缺口：** 过滤可能可用；oapi search/get 响应通常不含 `finishTime` |

### Windows 上的 Agent / 脚本

消费 CLI JSON 时，优先用 **Node / Python 子进程**（以 Buffer/bytes 捕获 stdout 再 `JSON.parse`），避免 PowerShell `>` 重定向改写编码导致解析失败。用 `yunxiao doctor` 查看已解析的可执行文件路径与当前 profile（`organization_id`、`space_id`）。

对于 AI Agent，CLI 通过 Agent 粘贴指令并执行 `yunxiao …`；MCP 通过工具调用。MCP 可以减少对命令记忆的要求，但在审计性和可复现性方面通常弱于 CLI。

## 安装

**推荐 — [GitHub Releases](https://github.com/xiaoxiaolin0918/yunxiao-cli/releases/latest)：**

按系统下载归档，解压后把 `yunxiao` 加入 `PATH`。

```bash
yunxiao --version   # 应与 GitHub latest Release 一致
```

本项目**仅在 GitHub 上维护**（`sliverTwo/yunxiao-cli`）。

**从源码安装（次要）：**

```bash
make build          # 生成 ./yunxiao（-ldflags 注入 Version）
make install        # 安装到 ~/.local/bin/yunxiao
go build -o yunxiao .   # 无 ldflags 时回退包内默认见 internal/version 或 ldflags
# 显式注入：
# go build -ldflags "-X github.com/yunxiao-cli/yunxiao/internal/version.Version=<release>" -o yunxiao .
```

需要 Go 1.24.4+。`make build` / `make ci` 通过 `-ldflags -X …version.Version=$(VERSION)` 注入版本（`VERSION` 默认 `git describe` 或 git describe / VERSION）。

**已知限制：** `go install` / 单独二进制**不包含**仓库 `skills/` 目录；请在源码检出目录运行（或使用会解压 `skills/` 的安装器），或另行复制 / `npx skills add`。需要技能时优先检出目录 `make build`，再执行 `yunxiao skills install`。


## 更新

版本迭代较快——请用 `yunxiao update` 升级。若 GitHub 上有更新的 Release，CLI 偶尔会在 **stderr** 打印一行提示（网络检查最多每 24 小时一次，缓存写在 `~/.config/yunxiao/update_check.json`）。`update` / `self-update` / `completion`、默认的 `--format json`、以及通过环境变量关闭时都会跳过提示。检查失败不会阻塞或导致命令失败；不会自动下载。

提示示例（打印到 **stderr**）：`发现新版本 yunxiao：0.16.6 → 0.16.13。运行：yunxiao update`

**二进制（GitHub Releases）— 推荐：**

```bash
yunxiao update --check     # 仅检查；若有新版本则 exit 2（适合 CI/Agent）
yunxiao update             # TTY：确认后替换当前二进制
yunxiao update --yes       # 非交互直接更新（脚本）
# 别名：yunxiao self-update
```

`yunxiao update` 从 [GitHub Releases](https://github.com/xiaoxiaolin0918/yunxiao-cli/releases) 下载对应平台归档（`yunxiao-cli-<ver>-<os>-<arch>.tar.gz|.zip`），在有 `checksums.txt` 时校验 SHA-256，并安全替换正在运行的二进制（先写到旁边再 rename；Windows 上若 `--version` 仍显示旧版本，请重启进程）。门禁：`--dry-run` / `--check` 不写入；真正替换需 TTY 确认或 `--yes`。

可选 doctor 探测（仍需显式打开；不下载）：

```bash
yunxiao doctor --check-update
```

关闭机会性提示以及 doctor 的 `--check-update`：

```bash
export YUNXIAO_UPDATE_CHECK=0   # 亦可：false | off | no
```

**npm 安装器（`sanzhi-yunxiao-cli`）：**

```bash
npm install -g sanzhi-yunxiao-cli@latest
```

与安装器相同的环境变量：`YUNXIAO_CLI_GITHUB_REPO`、`YUNXIAO_CLI_DOWNLOAD_BASE`。

## 认证

1. 在云效控制台创建个人访问令牌（PAT，首选一键链接）：  
   https://account-devops.aliyun.com/settings/personalAccessToken  
   帮助文档：https://help.aliyun.com/zh/yunxiao/user-guide/personal-access-token

   推荐勾选：组织/成员**读**；项目管理/代码管理/流水线**读+写**（只读场景可只开读）；制品/测试/应用交付按需。令牌名建议 `yunxiao-cli`。
2. 推荐环境变量：

```bash
export YUNXIAO_ACCESS_TOKEN="<PAT>"
export YUNXIAO_ORGANIZATION_ID="<企业ID>"   # 可选
```

或写入配置文件：

```bash
yunxiao auth login --token "<PAT>"
yunxiao auth status
yunxiao doctor
```

令牌优先级（高→低）：`YUNXIAO_ACCESS_TOKEN` → `~/.config/yunxiao/credentials.json`（最近一次成功的 `auth login`，browser 或 token）→ 当前 profile 的 `access_token` → 旧版 `config.json`。OAuth 凭证只写 `credentials.json`（0600），不写 profile JSON。`yunxiao auth status` 含 `token_source` / `token_kind`（`pat`|`oauth`），不打印明文。推荐：`yunxiao auth login --browser`（授权=账号 API 全能力）；CI 仍用 `--token`/env。默认 API：`https://openapi-rdc.aliyuncs.com`；OAuth 探测通过后按记录的头发送（优先 `x-yunxiao-token`，否则 `Authorization: Bearer`）。

## Agent 快速上手

```text
浏览：  yunxiao <domain> --help
查看：  yunxiao schema <id>
优先：  +快捷命令 → 类型化命令 → yunxiao api
风险：  read | write | high-risk-write（高风险需用户确认后再加 --yes）
预览：  --dry-run    过滤：--jq '...'
```

## Agent 技能

仓库 `skills/yunxiao-*` 下技能（均含 `SKILL.md`）：

| 技能 | 用途 |
|------|------|
| `yunxiao-shared` | 认证、配置、doctor、JSON 约定、`--dry-run` / `--yes` |
| `yunxiao-organization` | 组织、成员、部门、角色 |
| `yunxiao-project` | Projex 项目与工作项 |
| `yunxiao-codeup` | 代码库、分支、文件、MR |
| `yunxiao-pipeline` | Flow 流水线、运行、任务、YAML |
| `yunxiao-packages` | 制品仓库与制品 |
| `yunxiao-testhub` | 测试计划、结果、计划用例评论 |
| `yunxiao-appstack` | 应用、变更单、编排、标签、变量组 |
| `yunxiao-zhiyi-ops` | 智衣/ZYPT 迭代建议、开缺陷、流转、建 MR 与租户 profile（可选） |

**安装**（默认目录 `~/.agents/skills`，供 AI 工具发现）：

```bash
# 1) 推荐 — 本地 CLI 安装
yunxiao skills install
yunxiao skills install --skill yunxiao-shared --skill yunxiao-codeup
yunxiao skills install --dir /custom/skills --dry-run
yunxiao skills install --symlink --force

# 2) 从本地仓库路径
npx skills add /path/to/yunxiao-cli -y -g

# 3) 从 GitHub（URL 必须以 .git 结尾）
npx skills add https://github.com/sliverTwo/yunxiao-cli.git -y -g
```

安装后请重启 / 重载 AI 工具。查看：`yunxiao skills list|path|read <name>`。

贡献者与 AI Agent 请先读 **[AGENTS.md](AGENTS.md)**。

## 分域示例

长 JSON 请求体请用 `--data-file path.json` 或 `--data @path.json`（避免 shell 引号长度限制）。


```bash
# organization
yunxiao organization +whoami
yunxiao organization list
yunxiao organization members search --query alice

# project / work items
yunxiao project list --name demo
yunxiao project +my-open-items
yunxiao project +created-by-me --status-stage 1,2
yunxiao workitem search --assigned-to self --category Req --priority <id>
yunxiao workitem search --category Req --created-after "2026-09-01 00:00:00" --created-before "2026-09-07 23:59:59"
yunxiao workitem search --category Bug --finish-after "2026-09-01 00:00:00" --finish-before "2026-09-07 23:59:59"
yunxiao workitem get --id <id>
yunxiao workitem comments list --id <id>
yunxiao workitem comment --id <id> --content "note" --dry-run
yunxiao workitem create --space-id <sid> --type-id <tid> --subject "title" --assigned-to self --dry-run
yunxiao workitem update --id <id> --assigned-to self --dry-run
yunxiao workitem +transition --id <id|serial> --to <alias|statusId> --dry-run

# codeup
yunxiao codeup repos list
yunxiao codeup branches list --repo <repoId>
yunxiao codeup tags list --repo <repoId>
yunxiao codeup tags create --repo <repoId> --tag-name v1.0 --ref master --dry-run
yunxiao codeup protected-branches list --repo <repoId>
yunxiao codeup protected-branches create --repo <repoId> --branch master --allow-push-roles 40,30 --dry-run
yunxiao codeup files tree --repo <repoId> --ref master
yunxiao codeup commits list --repo <repoId> --ref master
yunxiao codeup files create --repo <id> --path a.txt --branch master --message "add" --content "hi" --dry-run
yunxiao codeup files delete --repo <id> --path a.txt --branch master --message "rm" --dry-run
yunxiao codeup mrs merge --repo <id> --local-id 1 --merge-type no-fast-forward --dry-run
yunxiao codeup mrs close --repo <id> --local-id 1 --dry-run
yunxiao codeup mrs review --repo <id> --local-id 1 --opinion PASS --dry-run

yunxiao codeup mrs get --repo <id> --local-id 1
yunxiao codeup mrs diffs --repo <id> --local-id 1
yunxiao codeup mrs comments list --repo <id> --local-id 1
yunxiao codeup mrs comments create --repo <id> --local-id 1 --content "LGTM" --patchset-biz-id <biz> --dry-run
yunxiao codeup mrs labels list --repo <id> --local-id 1
yunxiao codeup mrs labels attach --repo <id> --local-id 1 --label-ids 1,2 --dry-run
yunxiao codeup mrs reopen --repo <id> --local-id 1 --dry-run
yunxiao codeup compare --repo <id> --from master --to feature
yunxiao pipeline job retry --pipeline-id <id> --run-id <r> --job-id <j> --dry-run
yunxiao pipeline job pass --pipeline-id <id> --run-id <r> --job-id <j> --dry-run
yunxiao pipeline job refuse --pipeline-id <id> --run-id <r> --job-id <j> --dry-run
yunxiao packages artifacts delete --repo-id <id> --repo-type GENERIC --id <aid> --dry-run
yunxiao workitem types list --space-id <sid> --category Req
yunxiao workitem create --space-id <sid> --type-id <tid> --subject "t" --assigned-to self --custom-fields '{"fid":"v"}' --dry-run
yunxiao workitem relations list --id <id> --relation-type ASSOCIATED
yunxiao workitem relations create --id <id> --related-id <rid> --relation-type ASSOCIATED --dry-run
yunxiao workitem delete --id <id> --dry-run
yunxiao testhub results update --plan-id <p> --testcase-id <t> --status PASSED --dry-run
yunxiao testhub plan-comments list --plan-id <p> --testcase-id <t>
yunxiao appstack change-orders job-logs --app my-app --sn <sn> --job-sn <jsn>
yunxiao appstack orchestrations list --app my-app
yunxiao appstack change-orders create --app my-app --data '{...}' --dry-run
yunxiao appstack change-orders create --app my-app --data-file order.json --dry-run
yunxiao codeup +open-mrs
yunxiao codeup mrs create --repo <id> --source feat --target master --title "x" --dry-run
yunxiao codeup mrs create --repo <id> --source feat --target master --title "x" --yes   # after user OK

# pipeline
yunxiao pipeline list
yunxiao pipeline +status --pipeline-id <id>
yunxiao pipeline run list --pipeline-id <id>
yunxiao pipeline run latest --pipeline-id <id>
yunxiao pipeline +failed --pipeline-id <id>
yunxiao pipeline job log --pipeline-id <id> --run-id <rid> --job-id <jid>
yunxiao pipeline run trigger --pipeline-id <id> --branch master --dry-run
yunxiao pipeline run cancel --pipeline-id <id> --run-id <rid> --dry-run

# packages (upload skipped — see Known gaps)
yunxiao packages repos list
yunxiao packages artifacts list --repo-id <id> --repo-type GENERIC

# testhub / appstack
yunxiao testhub plans list --project-id <id>
yunxiao testhub plans progress --plan-id <id>
yunxiao appstack apps list
yunxiao appstack change-orders versions --app my-app
yunxiao appstack change-orders job-logs --app my-app --sn <sn> --job-sn <jsn>
yunxiao appstack orchestrations list --app my-app


# v0.7
yunxiao pipeline get --id <id>
yunxiao pipeline create --name ci --file ./pipeline.yaml --dry-run
yunxiao pipeline update --id <id> --name ci --file ./pipeline.yaml --dry-run
yunxiao workitem attachments list --id <id>
yunxiao workitem attachments create --id <id> --file ./shot.png --dry-run
yunxiao appstack tags search --search demo
yunxiao appstack tags create --name t --color "#4676e5" --dry-run
yunxiao appstack tags bind --app my-app --tag-names t --dry-run
yunxiao appstack variable-groups list --app my-app
yunxiao appstack variable-groups revision --app my-app

# v0.8
yunxiao organization departments list
yunxiao organization roles list
yunxiao project get --id <id>
yunxiao sprint list --space-id <id>
yunxiao versions list --space-id <id>
yunxiao workitem fields --space-id <s> --type-id <t>
yunxiao pipeline service-connections list --type codeup
yunxiao pipeline host-groups list
yunxiao pipeline flow-variable-groups list
yunxiao codeup repos get --repo <id>
yunxiao codeup branches create --repo <id> --branch feat --ref master --dry-run
yunxiao appstack apps create --name demo --dry-run
yunxiao appstack change-requests list --app my-app
yunxiao appstack global-vars list
yunxiao testhub cases search --repo-id <id>
yunxiao testhub directories create --repo-id <id> --name folder --dry-run

# v0.9
yunxiao appstack release-workflows list --app my-app
yunxiao appstack release-workflows stage execute --app a --workflow-sn w --stage-sn s --dry-run
yunxiao appstack deploy machine-log --tunnel-id 1 --machine-sn sn
yunxiao appstack deploy add-hosts --instance n --host-sns a,b --dry-run
yunxiao pipeline vm-deploy get --pipeline-id p --deploy-id d
yunxiao pipeline vm-deploy stop --pipeline-id p --deploy-id d --dry-run
yunxiao pipeline resource-members create --resource-type pipeline --resource-id id --role-name viewer --user-id u --dry-run
yunxiao workitem efforts list --id <id>
yunxiao workitem efforts mine --start-date 2026-01-01 --end-date 2026-01-31
yunxiao workitem estimated-efforts create --id <id> --owner self --spent-time 4 --dry-run
yunxiao programs search --name demo
yunxiao codeup repos create --name my-repo --path my-repo --dry-run
# escape hatch
yunxiao api GET /oapi/v1/platform/user
yunxiao schema
```

## Profile：play vs zhiyi（可选）

运行 `yunxiao +onboard` 可从所选 `space_id` 在 `~/.config/yunxiao/profiles/` 下创建通用本地 profile。

租户级 Projex 常量放在 **profile JSON**，不写进 CLI 全局默认。按项目（`space_id`）隔离；`workflows` 按 **`type_id`** 存放已探索状态图。
`workitem_defaults` 同样按 **`type_id`** 存放 OpenAPI 字段默认值与创建必填；`workitem create` / `+bug-create` 会自动填入（可用 `--no-defaults` 跳过）；`profile doctor` 会列出并校验这些字段 id。

| Profile | 用途 |
|---------|------|
| **zhiyi** | 智衣/ZYPT 全字段（module/environment/ExpCompletionTime + 完整流转必填） |
| **play** | 沙箱/YXCLI 回归 — 精简 `bug_create_fields`（仅 priority + seriousLevel）；`bug_transition_required` 仅 `{"100010":["80"]}` |

```bash
yunxiao profile install-example zhiyi   # 或 play
export YUNXIAO_PROFILE=zhiyi
yunxiao profile doctor
yunxiao workitem +bug-create --profile play --title "标题" --description "描述" --sprint <id> --dry-run
yunxiao workitem +bug-create --minimal --title "…" --description "…" --sprint <id> --dry-run
yunxiao workitem relations create --id <id> --related-id <rid> --relation-type ASSOCIATED --dry-run
```

详见 skill `yunxiao-zhiyi-ops`、`profiles/zhiyi.example.json`、`profiles/play.example.json`。

## 风险门禁

- `write`：先确认意图，尽量 `--dry-run`
- `high-risk-write`：缺少 `--yes` 时退出码 **10**，stderr 含 `confirmation_required`；**必须**向用户确认后再重试，禁止静默加 `--yes`

## 开发

```bash
make test && make build
make ci                 # go build -ldflags … ./... && go test ./... && go vet ./...
./scripts/ci.sh         # 同上的 POSIX 本地 / 通用 CI 脚本
```

CI/CD **仅使用 GitHub Actions**（本仓库在 GitHub 维护，不再镜像到 Codeup Flow）：

- `.github/workflows/ci.yml` — 推送到 `main` 或 Pull Request 时跑 CI
- `.github/workflows/release.yml` — 推送 `v*` 标签时构建各平台归档并发布 GitHub Release

## 已知缺口

Packages **上传**、Codeup **blame/cherry-pick**、MR label detach 等仍无明确 OpenAPI；**Topic/Risk** 工作项类型需在项目设置 UI 启用（CLI 无法启用）。部分类型未启用**迭代**时请省略 `--sprint`。关联类型可用 `ASSOCIATED`/`DEPEND_ON`（`RELATED`/`PARENT_SUB` 常失败）。`profile doctor` 可对照线上字段/工作流。Codeup tags / protected-branches 已支持；`--content-file` 支持绝对路径。详见 [README.md](README.md) 的 Known gaps。

## 变更摘要

- **0.16.20**：+transition --dry-run 有边缓存时校验流转边，无边时标注 skipped（#59）。

**0.16.19** — workitem create 成功响应保留 serialNumber/status（默认 brief；create 返回 null 时自动再 GET）（#62）
- **0.16.18** — mrs comments resolve/reopen (#56)
- **0.16.17** — 已有 MR 支持 `codeup mrs link|unlink` 与 `mrs update --work-item`（#54）
- **0.16.16** — 写操作网络失败附查重命令；GET 传输层错误已自动重试（#47）
- **0.16.15** — bug-create 别名校验；mrs get `--brief` + status→state；mrs update 改标题/描述；create 默认摘要（#45–#46, #48–#51）
- **0.16.14** — `yunxiao alias`（禁止内嵌 `--yes`）；命令↔文档 CI 检查；npm/OIDC 发版评估（#38–#40）
- **0.16.13** — `yunxiao browse`（pipeline/workitem/mr/repo/url，`--print-only`）；README 人类 30 秒；usage 索引与 gh/yx 迁移映射；completion 说明（#34–#37）
- **0.16.12** — Codeup MR workItemIds as OpenAPI string + fail if link missing
- **0.16.11** — +pending --all-pipelines soft-fail meta; update --validate noop only when full YAML+name match; --check (#29/#30)
- **0.16.10** — pipeline queue observability: +queue, runner-groups, run meta.queue, 403 hints (#23)
- **0.16.9** — pipeline change safety: get --yaml, diff, update --validate, 1209300 details (#21)
- **0.16.8** — Codeup MR workItemIds precheck + list ignored-param WARNING (#24)
- **0.16.7** — 人工卡点闭环：`pipeline +pending`、`run watch`、`job pass|refuse --yes` / `+approve|+refuse`（#22）
- **0.16.6** — `yunxiao codeup mrs reviewers add` 为已有 MR 添加评审人（#18）
- **0.16.4** — 中文 `update --help` + yunxiao-shared skill 自更新（#15）；`codeup mrs create --reviewer`（Fixes #16）+ `+create` reviewerUserIds 修复（#17）
- **0.16.3** — 使用时机会性更新提示（中文 stderr、24h 缓存、`YUNXIAO_UPDATE_CHECK=0`）
- **0.16.2** — 工作项搜索日期过滤（`--created-after`/`--updated-before` 等）、`--all`、只读 `:search`；周报跟进（`--as-items`、doctor 可执行路径、MCP 映射）；`yunxiao update` / `self-update`
- **0.16.1** — 冒烟修复：`appstack apps list` 补齐必填 `pagination=keyset`；`workitem search` / `project +my-open-items` 回退 profile `space_id` 或给出清晰 CLI 错误；`programs search` 非高级版组织返回更友好提示
- **0.16.0** — 浏览器 OAuth（`auth login --browser` / `--dry-run`）、`credentials.json`（0600）、`auth probe-oauth`、oauth 自动 refresh；「面向 AI Agent」优先 browser OAuth；CI 保留 `--token`
- **0.15.7** — `yunxiao +onboard`：按所选项目/`space_id` 写入 `~/.config/yunxiao/profiles/` 的通用 profile（TTY 选择或 `--space-id`）；README「面向 AI Agent」；缺 token 时提示 PAT 控制台链接与模块权限清单
- **0.15.6** — 评论/活动/历史类列表默认最新在前（`--sort asc|desc`，非法值报错）；评论按**创建时间**排序；活动/MR/流水线运行/工时等仍偏好更新时间；分页列表的客户端 `--sort` 仅作用于**当前页**（`--all` 时对已拉取页整体排序）
- **0.15.5** — 长 JSON 支持 `--data-file` / `--data @file.json`（`api`、appstack、testhub 等）
- **0.15.2** — companion skills 对齐 CLI 0.15.x（`has_more` / `meta.url` / `refresh_ok`）；`client.ListAll` + `pipeline list --all` / `codeup mrs list --all`；`scripts/flow-ci.sh`（阿里云 golang 镜像 + `GOPROXY=goproxy.cn`）
- **0.15.1** — B5 wave2：更多命令迁到 `runRead`/`runJSONMutating`（workitem update/relations list；codeup 写；pipeline 变更+剩余读；org/project/sprint/versions/packages/testhub/appstack/effort/programs 读与简单写）。仍自定义：multipart 附件、cancel-reason soft-warn dry-run、pipeline create/update YAML 预览脱敏、多步快捷命令
- **0.15.0** — 结构重构：B5 `runRead`/`runJSONMutating` 命令模板（部分迁移）；C1 拆分 `workitem.go`；C2 预编译日期正则；C3 `Do` 返回 headers；C4 ldflags 注入 Version
- **0.14.11** — pipeline/run 输出附带 Flow 控制台 `url`（`meta.url`；列表项注入）`https://flow.aliyun.com/pipelines/{id}` 与 `.../builds/{runId}`
- **0.14.10** — A1：文档记录接受 git 历史残留（≤v0.14.5 示例 ID）；D1：Retry-After 睡眠上限 30s；C5：清理 README 英文重复示例；标注 `go install` 与 skills 探测限制
- **0.14.9** — B1：HTTP 客户端 `context.Context` + GET/HEAD 重试（429/5xx/网络错误，尊重 Retry-After）；P2：`has_more` 结合 total/page/per_page；更多 list 接入 MetaWithPagination
- **0.14.8** — cobra Execute→exit 10 E2E；`refreshAfterTransition` + warning/`refresh_ok` 单测；列表 `meta.has_more`/`total`/`page`（MR list 接入 MetaWithPagination）
- **0.14.7** — help/skills 真实 ID 占位化；B3 Write/门禁契约测 + PostMultipart httptest；流转成功 JSON 增加 `refresh_ok`
- **0.14.6** — 示例 profile 脱敏为占位符；`go mod tidy`；`make ci` / `scripts/ci.sh`；流转后刷新失败打 stderr warning
- **0.14.4** — workitem/MR 输出附带可点击 `url`（`meta.url`；列表项注入 `url`）；URL 构建集中在 `internal/zhiyi`
- **0.14.3** — profile 可选 `access_token`；优先级 env > profile > config；`auth status` / doctor 报告 `token_source`
- **0.14.2** — `workitem create` / `+bug-create` 自动应用 `workitem_defaults`（priority/trackers/测试负责人/验收负责人），可用 `--no-defaults` 跳过
- **0.14.1** — profile `workitem_defaults`（按 `type_id` 存字段默认值与创建必填）；`profile doctor` 报告/校验
- **0.14.0** — 沙箱准确 `play` profile；`+bug-create --minimal`；`profile doctor`；关联类型文档；`--content-file` 绝对路径
- **0.13.1** — Codeup `--repo` 别名解析覆盖 branches/files/commits/compare/mrs/repos
- **0.13.0** — `workitem +transition`；Codeup `tags` / `protected-branches`；Topic/Risk 需项目 UI 启用
- **0.12.1** — profile 按 `type_id` 存 `workflows`；`--write-profile` 写入该映射（Bug 仍保留 `bug_*`）
- **0.12.0** — `workitem +explore-workflow` 探测状态流转图；`--write-profile` 写回 profile
- **0.11.0** — 智衣 `sprint +current`、`workitem +bug-create`、`codeup mrs +create`；profile 仓库/创建字段
- **0.10.0** — 智衣 profile；ZYPT workitem get；`workitem +bug-transition`；skill `yunxiao-zhiyi-ops`
- **0.9.1** — `yunxiao skills install`；AGENTS.md；README 技能安装说明
- **0.9.0** — AppStack 发布流/部署主机；Flow VM 部署单与资源成员写；工时/项目集；Codeup 建库

## 许可证

MIT — 见 [LICENSE](LICENSE)。


## ManualValidate / 阿里云 UID

yunxiao organization members list --include-aliyun-uid（需 ALIBABA_CLOUD_ACCESS_KEY_ID/SECRET）。详见 skill yunxiao-organization / yunxiao-pipeline。
