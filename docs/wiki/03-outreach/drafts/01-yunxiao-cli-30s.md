> **状态：草稿 / 未发布**（仅存仓库 wiki，勿直接当已发 CSDN 文）  
> 系列：yunxiao-cli CSDN 引流 · 基于 **v0.16.40**（main 已含 #170 `auth refresh`，随 0.16.41 发版）  
> 命令以本机 `yunxiao <cmd> --help` / `schema` 为准；写操作示例默认带 `--dry-run`。

# 云效终于有「像 gh 一样」的 CLI 了：yunxiao-cli 30 秒上手

你是不是也经历过这种上午：

1. 打开云效 Projex，点自己的工作项；
2. 切到 Codeup，翻打开的 MR；
3. 再切 Flow，看流水线卡在哪。

三套控制台、十次鼠标点击，信息还不好复制进周报或脚本。GitHub 有 `gh`，飞书/Lark 有自家 CLI——**阿里云云效也该有一条能进 PATH 的命令行**。

这就是 **yunxiao-cli**：Go 单二进制，命令名 `yunxiao`，风格对齐「+shortcuts / typed 命令 / 原始 `api` / 风险闸门」。安装渠道**只有 GitHub Releases**（不要再用已停用的 npm 包装）。

## 30 秒路径（复制即跑）

```powershell
# Windows 一行安装
irm https://raw.githubusercontent.com/xiaoxiaolin0918/yunxiao-cli/main/install.ps1 | iex

yunxiao --version
yunxiao auth login --browser
yunxiao whoami
yunxiao doctor
yunxiao status
yunxiao codeup +open-mrs
```

macOS / Linux：从 [Releases](https://github.com/xiaoxiaolin0918/yunxiao-cli/releases/latest) 下对应归档，把 `yunxiao` 放进 `PATH`，后面四条命令一样。

| 命令 | 作用 |
|------|------|
| `auth login --browser` | 浏览器 OAuth（交互/Agent 推荐） |
| `whoami` | 当前身份与 token 状态（JSON，不打印裸 token） |
| `doctor` | 配置、认证、连通性体检 |
| `status` | 只读看板：我的打开工作项 + 打开的 MR（可选流水线闸门） |
| `codeup +open-mrs` | 列出打开的合并请求 |

CI / 无头环境用 PAT：

```bash
yunxiao auth login --token "<PAT>"
```

PAT 控制台：https://account-devops.aliyun.com/settings/personalAccessToken  
**不要把真实 PAT 贴进博客或聊天记录**，示例一律用 `"<PAT>"`。

## 模块地图（一句话）

装好后 `yunxiao --help` 能看到这些域：

| 域 | 典型用途 |
|----|----------|
| `organization` / `project` | 组织、项目空间、标签 |
| `workitem` | Projex 工作项搜索/创建/流转 |
| `codeup` | 仓库、分支、MR |
| `pipeline`（别名 `flow`） | 流水线列表、运行、待审批 |
| `packages` / `testhub` / `appstack` | 制品、测试、应用变更 |
| `browse` | 终端出控制台链接（或直接开浏览器） |
| `skills` / `profile` | Agent 技能包、本地租户常量 |
| `api` / `schema` | 任意 OpenAPI 逃生舱 + 参数探查 |

偏好顺序（Agent 也一样）：

1. **`+shortcut`**（任务级，如 `codeup +open-mrs`、`pipeline +status`）
2. **typed 命令**（一个 API 方法，如 `codeup mrs list`）
3. **`schema <domain.resource.method>`**（写之前先看参数与风险）
4. **`api GET|POST …`**（官方尚未封装的路径）

## 安全习惯：先只读，再写入

每个命令的 `--help` 会标风险：`read` / `write` / `high-risk-write`。

- 写操作先加 **`--dry-run`**（只预览请求，不真正调用）
- **high-risk-write**（合并 MR、触发流水线、改 YAML、删工作项等）需要用户确认后再加 **`--yes`**
- 长 JSON 用 `--data-file ./body.json`，避免 PowerShell 编码把中文弄坏
- 输出默认 JSON，可用 `--jq` 过滤；脚本侧用 Node/Python 解析 stdout，少用 PowerShell `>` 重定向（编码易踩坑）

```bash
# 示例：合并前只看预检，不真正 merge
yunxiao codeup mrs merge --repo org/repo --local-id 12 --merge-type squash --dry-run
```

## 和「网页点点点」差在哪

| | 云效网页 | yunxiao-cli |
|--|----------|-------------|
| 晨检 | 三个页签 | 一条 `yunxiao status` |
| 进脚本/CI | 难 | 命令可复制、可审计 |
| 写操作 | 点确认 | `--dry-run` → 确认 → `--yes` |
| Agent | 难复用 | skills + 结构化 JSON + 退出码 |

当前稳定版以 GitHub **latest** 为准；本文基于 **v0.16.40**（含 Windows 一行安装、`update` 同步 skills/profiles、只读 `status`）。main 上 #170 已合入：`doctor`/`whoami` 可静默刷新 OAuth，并新增 `yunxiao auth refresh`（随 0.16.41 发版）。

下一篇会深挖：**Windows 一行安装与 `yunxiao update`**。你最烦云效网页的哪一步？欢迎评论区吐槽——顺手给仓库点个 Star。

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

**本文相关**：30 秒上手：`auth login` → `whoami` / `doctor` / `status` → `codeup +open-mrs`  
觉得有用请给仓库点个 **Star**，Issues 欢迎提场景。
