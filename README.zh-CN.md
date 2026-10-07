语言：中文 | [English](README.md)

# yunxiao-cli

云效 CLI，对标飞书 / Lark CLI：渐进发现、`+shortcuts`、类型化 API 命令、原始 `api` 逃生舱、风险门禁与 Agent skills。

CLI 二进制名：**`yunxiao`**。

## 人类 30 秒快速开始

1. 安装：打开 [https://github.com/xiaoxiaolin0918/yunxiao-cli/releases/latest](https://github.com/xiaoxiaolin0918/yunxiao-cli/releases/latest)，下载对应平台归档，把 `yunxiao` 加入 PATH。
2. 登录：`yunxiao auth login --browser`（CI 用 `--token`）。
3. 试跑：`yunxiao whoami` · `yunxiao doctor` · `yunxiao status` · `yunxiao codeup +open-mrs`。
4. 打开控制台页：`yunxiao browse pipeline --pipeline-id <id> --print-only`。
5. Shell 补全：`yunxiao completion powershell | Out-String | Invoke-Expression`（bash/zsh 见 [usage 索引](docs/wiki/01-usage/README.md)）。

GitHub Releases 是唯一安装渠道。npm 薄包装（`sanzhi-yunxiao-cli`）已停用：私有 npm 源把它冻结在旧版本，装出来的机器没有 `update` 命令（#115）。

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
   提示：oat- 令牌约 1 天短效。登录成功会打印人类可读过期时间（expires_at_local）与续期命令；
   `auth status` / `doctor` 在剩余 <24h 且无 refresh_token 时会提醒，重跑 yunxiao auth login --browser 续期（#122）。
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

**git-bash / MSYS 路径改写（#117）：** MSYS 会在 CLI 收到参数前，把以 `/` 开头的参数改写成 Windows 路径，例如 `yunxiao api GET "/oapi/v1/platform/user"` 实际变成 `C:/Program Files/Git/oapi/v1/platform/user`。CLI 会识别该特征并自动还原 `/oapi/...`，同时在 stderr 打一行 `note:`（`YUNXIAO_API_NO_UNMANGLE=1` 可关闭；`//oapi/...` 双斜杠写法也会被折叠）。无法还原时，非 JSON（HTML）响应会以 `error.subtype=non_json_response` 报出最终请求 URL、HTTP 状态码、content-type 与截断正文——检查 `error.details.url` 是否含 `<盘符>:/` 段。Shell 侧规避：`MSYS_NO_PATHCONV=1`、`MSYS2_ARG_CONV_EXCL='*'`，或加双斜杠 `//oapi/v1/...`。

对于 AI Agent，CLI 通过 Agent 粘贴指令并执行 `yunxiao …`；MCP 通过工具调用。MCP 可以减少对命令记忆的要求，但在审计性和可复现性方面通常弱于 CLI。

## 安装

**推荐 — [GitHub Releases](https://github.com/xiaoxiaolin0918/yunxiao-cli/releases/latest)：**

按系统下载归档，解压后把 `yunxiao` 加入 `PATH`。

```bash
yunxiao --version   # 应与 GitHub latest Release 一致
```

本项目**仅在 GitHub 上维护**（`xiaoxiaolin0918/yunxiao-cli`；旧名 `sliverTwo/yunxiao-cli` 依赖 GitHub 重定向）。

**从源码安装（次要）：**

```bash
make build          # 生成 ./yunxiao（-ldflags 注入 Version）
make install        # 安装到 ~/.local/bin/yunxiao
go build -o yunxiao .   # 无 ldflags 时回退包内默认见 internal/version 或 ldflags
# 显式注入：
# go build -ldflags "-X github.com/yunxiao-cli/yunxiao/internal/version.Version=<release>" -o yunxiao .
```

需要 Go 1.24.4+。`make build` / `make ci` 通过 `-ldflags -X …version.Version=$(VERSION)` 注入版本（`VERSION` 默认 `git describe`，或回退 `internal/version` 包内默认值）。

**已知限制：** `go install` / 单独二进制**不包含**仓库 `skills/` 目录；请在源码检出目录运行（或使用会解压 `skills/` 的安装器），或另行复制 / `npx skills add`。需要技能时优先检出目录 `make build`，再执行 `yunxiao skills install`。


## 更新

版本迭代较快——请用 `yunxiao update` 升级。若 GitHub 上有更新的 Release，CLI 偶尔会在 **stderr** 打印一行提示（网络检查最多每 24 小时一次，缓存写在 `~/.config/yunxiao/update_check.json`）。`update` / `self-update` / `completion`、默认的 `--format json`、以及通过环境变量关闭时都会跳过提示。检查失败不会阻塞或导致命令失败；不会自动下载。

提示示例（打印到 **stderr**）：`发现新版本 yunxiao：0.16.28 → 0.16.29。运行：yunxiao update`

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

下载源覆盖：`YUNXIAO_CLI_GITHUB_REPO`、`YUNXIAO_CLI_DOWNLOAD_BASE`。

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
npx skills add https://github.com/xiaoxiaolin0918/yunxiao-cli.git -y -g
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
yunxiao project labels list --space-id <id>   # id/name/color (#141)
yunxiao project +my-open-items
yunxiao project +created-by-me --status-stage 1,2
yunxiao workitem search --assigned-to self --category Req --priority <id>
yunxiao workitem search --category Req --created-after "2026-09-01 00:00:00" --created-before "2026-09-07 23:59:59"
yunxiao workitem search --category Bug --finish-after "2026-09-01 00:00:00" --finish-before "2026-09-07 23:59:59"
yunxiao workitem get --id <id>                 # brief（CLI ≥0.16.34）：关键字段；status 仅 {id,displayName}
yunxiao workitem get --id <id> --full          # 原始完整对象（含 status.name / nameEn / description / customFieldValues）
yunxiao workitem get --id <id> --fields subject,description,customFieldValues
YUNXIAO_WORKITEM_GET_VIEW=full yunxiao workitem get --id <id>   # 兼容开关：新旧 CLI 都输出原始对象（旧版忽略 env）
# 混用新旧 CLI：先试 --full，不识别该 flag 时回退
yunxiao workitem get <id> --full 2>/dev/null || yunxiao workitem get <id>
yunxiao workitem comments list --id <id>
yunxiao workitem comment --id <id> --content "note" --dry-run
yunxiao workitem create --space-id <sid> --type-id <tid> --subject "title" --assigned-to self --dry-run
yunxiao workitem update --id <id> --assigned-to self --dry-run
yunxiao workitem +transition --id <id|serial> --to <alias|statusId> --dry-run
yunxiao workitem +transition --id <id> --to <alias> --fields '{"<fieldId>":"<value>"}' --yes
# +transition 成功输出默认 brief（serialNumber、from_status→to_status 显示名、url、
# refresh_ok；item 为简要投影——与 create/get 同惯例，#114）；--full（或
# YUNXIAO_WORKITEM_GET_VIEW=full）输出刷新后的完整对象。
# 状态入场必填字段无 OpenAPI 配置（fields 只标类型级 required）；PUT 400「xx必填」时
# CLI 会把中文字段名映射回 fieldId 并给出可直接复制的 --fields 草稿
# （error.subtype=transition_required_fields，#113）。

# codeup
# --repo 支持：数字 id | profile 别名 | org[/group]/repo 路径（CLI 自动 URL 编码——
# 直接写斜杠即可，不要 %2F）| 裸仓库名（组织内唯一时自动解析，发一次只读搜索；#125）。
# 注册稳定别名：
yunxiao profile repo-add zhiyi_doc sanzhi/zhiyi/zhiyi_doc --profile zhiyi
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
yunxiao codeup mrs merge --repo <id> --local-id 1 --merge-type no-fast-forward --dry-run   # API 拒绝时：subtype merge_rejected + current_status/state_gap/suggested_actions + error.details.mr（#124/#127）
yunxiao codeup mrs merge --repo <id> --local-id 1 --merge-type no-fast-forward --dry-run
#        ^ merge 前会预检（#130）：状态/冲突/mergeable + --merge-type 对照仓库合并方式；
#          不支持时 exit 1 merge_type_not_supported（不会 POST）
yunxiao codeup mrs close --repo <id> --local-id 1 --dry-run
yunxiao codeup mrs review --repo <id> --local-id 1 --opinion PASS --dry-run

yunxiao codeup mrs get --repo <id> --local-id 1
yunxiao codeup mrs list --state opened --source feat/x --repo <id> --all   # 客户端分支过滤，meta.filtered_by=client（#96）
yunxiao codeup mrs update --repo <id> --local-id 1 --unwip   # 去掉 WIP: 标题前缀；--wip 加前缀；幂等（#97）
yunxiao codeup mrs list --state opened --status UNDER_DEV   # 客户端推送评审状态过滤；每条注入 status/wip（#132）
yunxiao codeup mrs +push-review-status --repo <id>          # open MR：status/wip/ahead/behind/mergeable/评审（#132）
yunxiao codeup mrs get --repo <id> --local-id 1 --full    # 原始对象；默认为 summary 中间档
#   三档视图（#130）：默认 summary = brief + mergeable/conflictCheckStatus/checkList/reviewers
#   [{name, opinion}]；--brief 最小；--full 原始；env YUNXIAO_MRS_GET_VIEW=full|summary|brief
yunxiao codeup mrs diffs --repo <id> --local-id 1   # 每项 latest + meta.latest_patchset_biz_id（#94）
yunxiao codeup mrs comments list --repo <id> --local-id 1
yunxiao codeup mrs comments create --repo <id> --local-id 1 --content "LGTM" --dry-run   # 全局评论缺省取最新 patchset（#93）
yunxiao codeup mrs labels list --repo <id> --local-id 1
yunxiao codeup mrs labels attach --repo <id> --local-id 1 --label-ids 1,2 --dry-run
yunxiao codeup mrs reopen --repo <id> --local-id 1 --dry-run
yunxiao codeup compare --repo <id> --from master --to feature
yunxiao pipeline job retry --pipeline-id <id> --run-id <r> --job-id <j> --dry-run
yunxiao pipeline job pass --pipeline-id <id> --run-id <r> --job-id <j> --dry-run
yunxiao pipeline job refuse --pipeline-id <id> --run-id <r> --job-id <j> --dry-run
yunxiao packages artifacts delete --repo-id <id> --repo-type GENERIC --id <aid> --dry-run
yunxiao workitem types list --space-id <sid>            # 缺省合并全部类别（含缺陷类）（#99）
yunxiao workitem types list --space-id <sid> --category Bug
yunxiao workitem statuses --space-id <sid> --type-id <tid>   # 类型状态表 + meta.default_status_id（#118）
yunxiao workitem create --space-id <sid> --type-id <tid> --subject "t" --assigned-to self --custom-fields '{"fid":"v"}' --dry-run
# create 会先预检必填字段（一次只读 GET，--dry-run 也会发）：缺失字段一次性全部报出
# （error.details.missing[]：field_id / name / pass_via / options）；--no-precheck 跳过（#95）。`+bug-create` 复用同一套预检（#107）。`--priority` / list 型 custom-fields 支持显示值（#126）
# 未启用的 type-id 报错自动附带 error.details.available_types（id/name/category），无需再查一遍（#99）
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
```bash
yunxiao status                      # 我的未完成工作项 + 开放 MR（加 --pipeline-id 含人工卡点）
yunxiao status --category Bug --repo <id>
yunxiao status --pipeline-id <id>
```


## Profile：play vs zhiyi（可选）

运行 `yunxiao +onboard` 可从所选 `space_id` 在 `~/.config/yunxiao/profiles/` 下创建通用本地 profile。

租户级 Projex 常量放在 **profile JSON**，不写进 CLI 全局默认。按项目（`space_id`）隔离；`workflows` 按 **`type_id`** 存放已探索状态图。
`workitem_defaults` 同样按 **`type_id`** 存放 OpenAPI 字段默认值与创建必填；`workitem create` / `+bug-create` / `+risk-create` / `+req-create`（#128）会自动填入（可用 `--no-defaults` 跳过）；`profile doctor` 会列出并校验这些字段 id。`allowed_environments` / `allowed_modules` 是线上字段选项的**快照**（用于校验 `+bug-create --environment` / `--module`）；`profile doctor` 会将其与线上 options 比对，漂移以 `enum_stale_in_profile` / `enum_missing_in_profile` findings 报告（#121）——请保持同步，或清空列表以停用该门禁。 findings 附带线上 status 的 displayName/nameEn 与字段名；`--fix-suggest`（默认开）为 profile 未收录的线上状态给出别名回填建议，`--write` 写回本地 profile（不碰 edges；#120）。

| Profile | 用途 |
|---------|------|
| **zhiyi** | 智衣/ZYPT 全字段（module/environment/ExpCompletionTime + 完整流转必填） |
| **play** | 沙箱/YXCLI 回归 — 精简 `bug_create_fields`（仅 priority + seriousLevel）；`bug_transition_required` 仅 `{"100010":["80"]}` |

```bash
yunxiao profile install-example zhiyi   # 或 play
# 示例只有占位符：编辑安装后的文件填 org/space/type id，再回填状态图
yunxiao profile path zhiyi              # -> ~/.config/yunxiao/profiles/zhiyi.json
export YUNXIAO_PROFILE=zhiyi            # 或 play（也可每条命令带 --profile zhiyi）
# 或一次写入默认，免去每个 shell 手动 export（#130）：优先级 --profile > YUNXIAO_PROFILE > 默认
yunxiao profile use zhiyi               # 写 config.json 的 "profile"；--unset 清除
yunxiao profile show
# 回填状态图。会写数据：创建探测工作项、流转其状态，--cleanup 会删除它。
# 先在沙箱（play）验证；切勿对生产 ZYPT 自动执行。--dry-run 只打印计划；要写回请去掉 --dry-run 并加 --yes。
yunxiao workitem +explore-workflow --profile zhiyi --type-id <type-id> --category Bug --cleanup --write-profile --dry-run
yunxiao profile doctor                  # 对比 profile 与线上 fields/workflow 及 allowed_* 枚举（只读）
                                       # findings 带 displayName/nameEn/fieldName，默认 --fix-suggest（#120）
yunxiao profile doctor --write --dry-run   # 预览将建议状态 id 回填到 profile
yunxiao profile doctor --write         # 回填 bug_statuses[别名] + workflows[type].statuses（不碰 edges）
yunxiao workitem +bug-create --profile play --title "标题" --description "描述" --sprint <id> --dry-run
yunxiao workitem +bug-create --minimal --title "…" --description "…" --sprint <id> --dry-run
# 风险 / 需求快捷创建（#128）：profile 映射 type id（risk_type_id / req_type_id 或唯一
# category 条目）与优先级（别名 / 显示值 / option id），#95 预检，默认不挂迭代；
# --assignee 可直接传组织成员显示名：
yunxiao workitem +risk-create --title "风险" --description "影响" --priority high --dry-run
yunxiao workitem +req-create --assignee "成员显示名" --title "需求" --description "…" --dry-run
yunxiao workitem relations create --id <id> --related-id <rid> --relation-type ASSOCIATED --dry-run
# codeup --repo 别名存于 profile.repositories，用
# `yunxiao profile repo-add <别名> <repo-id|org/repo路径|仓库名>` 注册（#125）
```

详见 skill `yunxiao-zhiyi-ops`、`profiles/zhiyi.example.json`、`profiles/play.example.json`。两个示例已内嵌进二进制，GitHub Release 归档也随二进制携带，任何安装方式下 `install-example` 都可直接使用；磁盘上存在 `profiles/<name>.example.json` 时优先（#92）。安装后的 profile 只有占位符：编辑该文件（`yunxiao profile path <name>`）填 org/space/type id，用 `workitem +explore-workflow --profile <name> --cleanup --write-profile --yes` 回填状态图（会创建/流转/删除探测工作项：先在沙箱验证，切勿对生产 ZYPT 自动执行；`--dry-run` 只打印计划），再用 `yunxiao profile doctor` 校验。

**使用约束（已封装，非缺口）：** 部分 Topic / Risk 类型未启用**迭代**字段时会返回 `未启用此字段【迭代】`，请省略 `--sprint`（CLI 会提示）。关联类型可用 `ASSOCIATED` / `DEPEND_ON`（`RELATED` / `PARENT_SUB` 常因类型约束失败）；Task 父子关系在创建时用 `--parent-id`。

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

- `.github/workflows/ci.yml` — 推送到 `main` 或 Pull Request（任意 base 分支，含叠放 PR）时跑 CI，`ubuntu-latest` + `windows-latest` 矩阵（#108/#109）
- `.github/workflows/release.yml` — 推送 `v*` 标签时构建各平台归档并发布 GitHub Release

## 已知缺口

**鉴权双通道：** 个人令牌 **OAPI** 仍无工作项评论 delete/update（raw DELETE 404）。CLI 已通过 AccessKey RPC 封装 workitem comments delete / update（凭证同 --include-aliyun-uid 的 ALIBABA_CLOUD_ACCESS_KEY_*）。详见 [docs/wiki/02-domains/workitem-comments-oapi-gaps.md](docs/wiki/02-domains/workitem-comments-oapi-gaps.md)。

以下能力**有意未封装**（有已确认的 OpenAPI 路径时可用 `yunxiao api`），与 [README.md](README.md) 的 Known gaps 表一致：

| 缺口 | 原因 |
|------|------|
| Packages **上传** / 制品仓库创建删除 | MCP `operations/packages` 不明确 / 无上传 OpenAPI |
| Codeup **blame**、**cherry-pick** | 未确认可靠 OpenAPI，不臆造 |
| 在项目上启用 Projex **Topic / Risk** 类型 | 组织可定义类型，但须在**项目设置 UI** 启用；创建返回 `工作项类型未启用！`；无启用 OpenAPI，CLI 无法启用 |
| MR 标签 **detach** | MCP 无对应 OpenAPI（仅 Get + Attach） |
| AppStack 完整变更单生命周期（超出已提供的 list/create 能力） | 仅在 MCP 明确时扩展 |
| Flow 结构化流水线 YAML 生成器（`createPipelineWithOptions`） | 仅 MCP helper；CLI 接收原始 YAML `--file` |

## 变更摘要

- **0.16.39** — `codeup mrs merge`：API 拒绝后 best-effort GET 当前 MR，信封增加 `subtype:"merge_rejected"` 与 `details.current_status` / `state_gap` / `suggested_actions` / `diagnose.source` / `mr`（状态表对齐 #127）；保留 #124/#130 门控与诊断失败透传（#157）
- **0.16.38** — 自 v0.16.37 起本批：`+bug-create` 复用 MissingRequired 预检（`--no-precheck` 跳过）（#107）；create 选项字段接受显示值并解析为 option id（#126）；`+bug-transition` BFS 无路回退直试 + `--direct`，区分无边与平台拒绝（#123）；`+risk-create` / `+req-create` 快捷命令对齐 `+bug-create`（#128）；`--repo` 支持 org/repo 路径与裸名自动发现 + `profile repo-add`（#125）；`project labels list|create` 与 `workitem search --labels`（#141）；OAuth 临期提醒（#122）；doctor 比对 `allowed_*` 与线上枚举漂移（#121）；findings 附带 live displayName/nameEn/fieldName + 默认 `--fix-suggest` / `--write` 回填（#120）；Release 归档像 skills/ 一样打包 profiles/ 示例（#116）
- **0.16.37** — 批量：`codeup mrs list --source/--target` 分支过滤（客户端侧，`meta.filtered_by:"client"`；配合 `--all` 过滤全量）（#96）；`codeup mrs update --wip/--unwip` 幂等标题前缀开关，与 `--title` 互斥（#97）；`workitem types list` 缺省合并全部类别（`--category all`；显式 `--category <单个>` 保持旧的单类别查询——缺省输出现在包含 Bug/Task/Risk/Topic 类型，每个 item 带 `category`），create/`+bug-create` 报「工作项类型未启用」时附 `error.details.available_types[]{id,name,category}`（#99）；新增 `workitem statuses` 只读命令（从类型工作流投影状态表，默认态标 `default:true`）（#118）；`+explore-workflow` 在任何请求前校验 `--write-profile`，`--dry-run` 完全离线（零 API 请求，`plan.network_reads:0`；category 只从 profile 解析——请显式传 `--category`）（#110）；`skills install` 如实上报被跳过的已存在技能（`installed_count`/`skipped_count`、`skipped[].reason:"exists"` + `--force` 提示）（#119）；CI 增加 `windows-latest` 并修复 3 个仅 Windows 失败的测试（含真实的 `assertRelativePath` 盘根逃逸：Windows 上 `/etc/passwd` 这类路径会解析到 cwd 之外），`.gitattributes` 强制 LF（#108）；叠放 PR（base ≠ main）重新有 CI 且被取代的运行自动取消（#109）；仓库 slug 统一为 `xiaoxiaolin0918/yunxiao-cli`（update/自更新目标 + 文档；旧名 `sliverTwo` 依赖 GitHub 重定向），Makefile `VERSION` 回退值改从 `internal/version` 提取而非硬编码
- **0.16.36** — npm-publish：改为 checkout 已刷新 checksums 的 ref（默认 `main`）而非 release tag；校验 package.json 与 Release 资产 checksums；支持 `dry_run`；文档 + `scripts/npm-publish-verify.sh`（#105）
- **0.16.35** — `profile install-example`：校验磁盘示例（拒绝空/截断/name 不匹配并回退内嵌）；去掉 cwd/`runtime.Caller` 候选；raw 提示 URL 固定 `v<version>`；仓库 `profiles/` 缺失时 sync-profiles 失败；npm/profiles 漂移检测（#104）
- **0.16.34** — **不兼容（输出）：** `workitem get` 默认 brief（id/serialNumber/subject/status 仅 `{id,displayName}`/assignedTo/sprint/priority/workitemType/categoryId/gmtModified，description 以 `description_summary` 占位；`meta.url` 保留）。`.data.status.name` / `nameEn` / `.data.description` / `.data.customFieldValues` 需 `--full`（或 `YUNXIAO_WORKITEM_GET_VIEW=full`）；CLI ≥0.16.34。兼容示例：`yunxiao workitem get X --full 2>/dev/null || yunxiao workitem get X`。拉代码后请 `yunxiao skills install --force` 刷新伴生技能（#98）
- **0.16.33** — workitem create 创建前一次性预检必填字段（dry-run 同样执行）：列出全部缺失字段及 field_id / 可选值；`--no-precheck` 跳过（#95）；后 `+bug-create` 亦接入（#107）。**兼容性变化：** (1) `--dry-run` 现在需要凭证与网络（一次 GET），缺必填时 exit 1；(2) 缺字段的报错由服务端 `type:"api"` 400 变为 `type:"cli"` + `subtype:"missing_required_fields"`；(3) 每次 create 多一次 GET；(4) 字段配置读不到时最多重试 1 次（退避 ≤1s，而不是默认 GET 重试策略的约 90s）后降级（整个读取最多 10s），告警写在 `meta.precheck` 并在 stderr 打印一行 `warning:`，401 直接失败；(5) `--no-precheck` 恢复旧行为（离线时使用）；(6) 根级字段（subject、assignedTo、sprint、labels 等）只认对应 flag，写在 `--custom-fields` 里不算已填；(7) 预检被跳过且随后 POST 失败时，`error.hint` 会带上 `precheck skipped: <原因>`
- **0.16.32** — mrs diffs 标记最新 patchset（每项 `latest: true|false`，`meta.latest_patchset_biz_id` / `meta.latest_version_no`；规则同 #93；原字段与顺序不变）(#94)
- **0.16.31** — mrs comments create：GLOBAL_COMMENT 的 `--patchset-biz-id` 改为可选（缺省取最新 MERGE_SOURCE patchset；dry-run 见 `request.resolved`；INLINE 仍必填）；`--comment-type` 改为校验（大小写不敏感，非法值直接报错）(#93)
- **0.16.30** — npm 包携带 `profiles/*.example.json`，二进制内嵌 zhiyi/play 示例，npm / GitHub Release 安装后 `profile install-example` 可用 (#92)
- **0.16.29** — workitem +bug-create 增加 `--title-file` / `--description-file`（UTF-8 去 BOM；Windows 中文安全；对齐 #85）(#89)
- **0.16.28** — workitem create 增加 `--subject-file` / `--description-file` / `--custom-fields-file`（UTF-8 去 BOM；Windows 中文安全）(#85)
- **0.16.27** — dry-run 在仅有 `hinted_edges` 时仍分类（#82）；#81 确认无 CLI 清屏 CSI，Configure 门控足够
- **0.16.26** — 非 TTY/`YUNXIAO_NO_TUI` 清屏防护（#76）；`+transition --dry-run` edges∪hinted 校验（#75）；`+bug-create --verifier` 与 serious-level 同义（#77）
- **0.16.25** — AccessKey RPC 接入 workitem comments delete|update（OAPI 仍仅 list/create）（#71）
- **0.16.24** — 标明 OAPI 工作项评论无 delete/update；workitem comment --content-file UTF-8（#69）
- **0.16.23** — +explore-workflow 区分 verified/hinted 边；支持 --custom-fields/--fields/--from（#61）（edges 仅 verified；hinted 另列）
- **0.16.22** — mrs update --repo <数字id> 拒绝不在 profile/org 可达仓列表中的 id（#63）
- **0.16.21** — +explore-workflow 按 --type-id 解析 category（默认自动覆盖；显式不匹配则报错）（#60）
- **0.16.20** — +transition --dry-run 有边缓存时校验流转边，无边时标注 skipped（#59）。
- **0.16.19** — workitem create 成功响应保留 serialNumber/status（默认 brief；create 返回 null 时自动再 GET）（#62）
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
- **0.10.0** — 智衣 profile；ZYPT workitem get；`workitem +bug-transition（BFS 无路时 direct_fallback / --direct，#123）`；skill `yunxiao-zhiyi-ops`
- **0.9.1** — `yunxiao skills install`；AGENTS.md；README 技能安装说明
- **0.9.0** — AppStack 发布流/部署主机；Flow VM 部署单与资源成员写；工时/项目集；Codeup 建库

## 许可证

MIT — 见 [LICENSE](LICENSE)。


## ManualValidate / 阿里云 UID

yunxiao organization members list --include-aliyun-uid（需 ALIBABA_CLOUD_ACCESS_KEY_ID/SECRET）。详见 skill yunxiao-organization / yunxiao-pipeline。
