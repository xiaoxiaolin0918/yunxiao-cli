> **状态：草稿 / 未发布**（仅存仓库 wiki，勿直接当已发 CSDN 文）  
> 系列：yunxiao-cli CSDN 引流 · 基于 **v0.16.40**（main 已含 #170 `auth refresh`，随 0.16.41 发版）  
> 命令以本机 `yunxiao <cmd> --help` / `schema` 为准；写操作示例默认带 `--dry-run`。

# 云效 CLI vs 官方 MCP：脚本/CI 选谁？一张表说清

「云效要不要上 MCP？」和「要不要装 CLI？」经常被问成二选一。结论先说：

> **脚本、CI、可审计复现 → CLI；IDE 里对话式操作 → MCP；很多团队两个都留。**

## 对比表（博客语气版）

| 维度 | yunxiao-cli | 云效 / 阿里云 DevOps MCP |
|------|--------------|---------------------------|
| 形态 | 本地可执行文件 `yunxiao` | MCP Server，由客户端暴露工具 |
| 典型用途 | 脚本、CI、终端、可粘贴命令 | IDE / 聊天 Agent 里对话完成任务 |
| 安装 | GitHub Releases 二进制（或源码构建）；skills 按需 `skills install` | 在支持 MCP 的客户端里配置 Server |
| 认证 | CLI profile / 环境变量 / PAT / 浏览器 OAuth | 随 MCP Server/客户端配置管理凭证 |
| 能力发现 | `--help`、`schema`、typed 命令、`+shortcuts`、companion skills | 客户端展示的工具目录与 input schema |
| 输出 | stdout/stderr、结构化 JSON、退出码、`--jq` | 由客户端渲染的工具结果 |
| 写入安全 | 统一 `--dry-run`；high-risk 需确认后 `--yes` | 依赖具体工具与客户端确认控件，非全局统一旗标 |
| 可复现 / 审计 | 命令可复制、可进 Git、可进 CI 日志 | 更依赖客户端上下文，审计与复现通常较弱 |
| IDE 依赖 | 无 | 需要 MCP 客户端（IDE / Agent 等） |

## 意图映射示例（周报 / 发现）

| MCP 风格意图 | CLI |
|--------------|-----|
| `search_workitems` + 创建时间窗 | `yunxiao workitem search --created-after … --created-before …`（可加 `--all`、`--as-items`） |
| 更新 / 完成时间窗 | `--updated-after/before`、`--finish-after/before` |
| 工作项评论 | `yunxiao workitem comments list --id <id>` |
| 列组织 / 项目空间 | `yunxiao organization list`、`yunxiao project list` |
| 探查搜索参数 | `yunxiao schema workitem.search` |
| 响应里的 `finishTime` | **缺口**：过滤可能可用；oapi search/get 响应常无该字段——CLI 不伪造 |

## 怎么选（决策树）

```text
要不要进 CI / 定时任务 / 周报脚本？
  ├─ 是 → 用 CLI（把最终命令写进仓库）
  └─ 否 → 是否主要在 IDE 里「问一句就干」？
        ├─ 是 → MCP 很合适；关键写入仍建议回落 CLI 命令以便审计
        └─ 否 → 终端日常：CLI；两者都装也不冲突
```

## 组合用法（推荐）

1. Agent 在 IDE 用 MCP **探索**（「这个空间有哪些打开的 Bug？」）。
2. 定稿后让 Agent / 人输出等价的 **`yunxiao …` 命令**，进 README 或 CI。
3. 写入路径统一要求：`--dry-run` → 人确认 → `--yes`。

这样既享受对话效率，又把「可复现的那部分」留给 CLI——不是二选一，是分层。

## 系列回顾

1. [30 秒上手](./01-yunxiao-cli-30s.md)  
2. [Windows 安装与 update](./02-windows-install-update.md)  
3. [Codeup MR](./03-codeup-mr.md)  
4. [Projex 工作项](./04-workitem-projex.md)  
5. [流水线 Flow](./05-pipeline-flow.md)  
6. [Agent skills / profile](./06-agent-skills-profile.md)  
7. 本文 · CLI vs MCP  

安装仍只认 GitHub Releases；给仓库一个 **Star**，下个版本的更新速览会更好写。

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

**本文相关**：CLI vs MCP：脚本/CI 用 CLI，对话用 MCP；团队可双持  
觉得有用请给仓库点个 **Star**，Issues 欢迎提场景。
