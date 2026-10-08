> **状态：草稿 / 未发布**（仅存仓库 wiki，勿直接当已发 CSDN 文）  
> 系列：yunxiao-cli CSDN 引流 · 基于 **v0.16.40**（main 已含 #170 `auth refresh`，随 0.16.41 发版）  
> 命令以本机 `yunxiao <cmd> --help` / `schema` 为准；写操作示例默认带 `--dry-run`。

# 给 AI Agent 用的云效：skills 安装、profile、`--dry-run` 护栏

yunxiao-cli 从第一天就按「人可以复制、Agent 可以驱动」设计：结构化 JSON、稳定退出码、`--jq`、风险在 `--help` 上写死。这一篇把 **skills / 本地 profile / 认证续期** 串成可粘贴工作流。

## 为什么适合 Agent

| 能力 | 对 Agent 的意义 |
|------|-----------------|
| JSON stdout + 退出码 | 易解析、易分支 |
| `--jq` | 少写胶水代码 |
| `schema` | 调用前自描述参数与风险 |
| `+shortcuts` | 任务级入口，少记底层 path |
| `--dry-run` / `--yes` | 写入可预览；高风险必须人点头 |

## 七步（可直接贴给 Agent，勿贴裸 token）

```text
1) Install from GitHub Releases (not npm).
   Windows: irm https://raw.githubusercontent.com/xiaoxiaolin0918/yunxiao-cli/main/install.ps1 | iex
   Then: yunxiao --version

2) Auth:
   Interactive: yunxiao auth login --browser
   CI/PAT:      yunxiao auth login --token "<PAT>"
   yunxiao whoami && yunxiao doctor && yunxiao auth status

3) yunxiao skills install

4) Read-only: yunxiao project list — ask the user to pick a project/space_id

5) Local profile under ~/.config/yunxiao/profiles/:
   Prefer: yunxiao +onboard
   Or: yunxiao +onboard --space-id <id> --profile <name>

6) export YUNXIAO_PROFILE=<name>   # 或 yunxiao profile use <name>
   yunxiao profile show
   yunxiao profile doctor
   yunxiao doctor

7) Writes: --dry-run first; high-risk needs user confirm then --yes;
   long JSON via --data-file. Never paste raw tokens into chat.
```

## skills 安装

```bash
yunxiao skills install
yunxiao skills install --skill yunxiao-codeup --skill yunxiao-pipeline
yunxiao skills install --force   # 覆盖已存在目录
```

默认装到 `~/.agents/skills`（可用 `--dir` 改）。已存在会跳过并计入 `skipped_count`（reason `exists`），避免「installed 0」被误当成刷新成功（#119）。技能包大致覆盖：shared / organization / project / codeup / pipeline / packages / testhub / appstack（以 `yunxiao skills` 列表为准）。

> `go install` **不带**仓库 `skills/`——要给 Agent 用，请用 Release 或显式 `skills install`。

## 本地 profile

```bash
yunxiao +onboard
yunxiao profile list
yunxiao profile show
yunxiao profile use zhiyi          # 写入 config 默认，新 shell 免 export（#130）
yunxiao profile doctor
yunxiao profile doctor --fix-suggest
yunxiao profile doctor --write     # 回写漂移（含 allowed_modules / allowed_environments，#166）
yunxiao profile repo-add app org/my-repo
```

`profile doctor` 会对比 `allowed_*` 与线上枚举漂移（#121），findings 带 live 显示名，并默认给出修复建议（#120）。示例 profile 随 Release 归档的 `profiles/` 下落，也可用 `profile install-example zhiyi|play`。

## 认证与续期

- 浏览器 OAuth（`oat-`）约 **1 天**有效；登录成功会打印本地可读过期时间。
- 剩余 `<24h` 且**没有**可用 refresh 时，`auth status` / `doctor` 会催续期。
- CI / 无头环境用 PAT，不要把 PAT 写进仓库或聊天。
- **main / 即将随 0.16.41 发布（#170）**：`doctor` / `whoami` 在存有 `refresh_token` 时可静默刷新；也可显式：

```bash
yunxiao auth refresh
yunxiao auth refresh --dry-run   # 只看计划，不联网、不改凭证
```

无 refresh 时仍需 `yunxiao auth login --browser`。

凭证优先级：`YUNXIAO_ACCESS_TOKEN` > `credentials.json`（最近一次 login）> profile > `config.json`。

## 护栏口诀

1. 先只读：`project list` / `status` / `schema`
2. 写入先 `--dry-run`
3. high-risk 必须**用户确认**后再 `--yes`（别让 Agent 自己猜）
4. Windows 消费 JSON：优先 Node/Python subprocess，慎用 PowerShell 重定向
5. git-bash 下若路径被 MSYS 改写，CLI 会尝试还原 `/oapi/...`（#117）；仍异常时可设 `MSYS_NO_PATHCONV=1`

Agent 能跑，但**写入仍要你点头**。欢迎提 skill 场景 Issue；Star + Watch Releases。下一篇用一张表说清 **CLI vs 官方 MCP**。

---

### 安装 yunxiao-cli（GitHub Releases，勿用 npm）

**仓库**：https://github.com/xiaoxiaolin0918/yunxiao-cli  
**最新 Release**：https://github.com/xiaoxiaolin0918/yunxiao-cli/releases/latest  

**Windows（PowerShell）一行安装：**

```powershell
irm https://raw.githubusercontent.com/xiaoxiaolin0918/yunxiao-cli/main/install.ps1 | iex
```

默认装到 `%USERPROFILE%\.local\bin`（归档内 `skills/`、`profiles/` 会一并落下）。之后升级：

```powershell
yunxiao update --yes
```

**macOS / Linux**：在 Releases 下载对应平台归档，解压后把 `yunxiao` 放到 `PATH`，然后：

```bash
yunxiao auth login --browser
yunxiao whoami && yunxiao doctor && yunxiao status
```

> npm 包装 `sanzhi-yunxiao-cli` 已停用，请勿再 `npm install`。

**本文相关**：Agent：`skills install`、`+onboard` / `profile doctor`、`auth refresh`（#170）与 `--dry-run` 护栏  
觉得有用请给仓库点个 **Star**，Issues 欢迎提场景。
