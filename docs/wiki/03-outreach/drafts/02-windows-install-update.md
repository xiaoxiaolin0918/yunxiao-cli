> **状态：草稿 / 未发布**（仅存仓库 wiki，勿直接当已发 CSDN 文）  
> 系列：yunxiao-cli CSDN 引流 · 基于 **v0.16.40**（main 已含 #170 `auth refresh`，随 0.16.41 发版）  
> 命令以本机 `yunxiao <cmd> --help` / `schema` 为准；写操作示例默认带 `--dry-run`。

# Windows 一行装上云效 CLI：install.ps1 与 `yunxiao update`

上一篇讲了「像 `gh` 一样」的云效命令行。这一篇只回答两件事：

1. **怎么装对**（唯一渠道：GitHub Releases）
2. **怎么升**（`yunxiao update`，顺带刷新 skills / profiles）

## 为什么不要再用 npm

历史上有过 npm 薄包装 `sanzhi-yunxiao-cli`。它已停用（#115），原因很实际：

- 私有 npm 源容易冻在旧版；
- 缺真正的自更新路径；
- 和现在「单二进制 + Release 归档」模型不一致。

**文中、脚本里都不要写 `npm i` / `npx yunxiao`。** 安装与升级只认：

https://github.com/xiaoxiaolin0918/yunxiao-cli/releases/latest

## Windows：一行安装

以管理员或普通用户打开 **PowerShell**：

```powershell
irm https://raw.githubusercontent.com/xiaoxiaolin0918/yunxiao-cli/main/install.ps1 | iex
```

脚本会：

- 拉取当前 latest（或你指定的版本）归档；
- 默认装到 `%USERPROFILE%\.local\bin`；
- 若归档含 `skills/`、`profiles/`，一并落下；
- 尝试把该目录前置到用户 PATH。

指定版本示例：

```powershell
# 先下载脚本再跑（便于传参）
irm https://raw.githubusercontent.com/xiaoxiaolin0918/yunxiao-cli/main/install.ps1 -OutFile install.ps1
.\install.ps1 -Version 0.16.40
```

装完验证：

```powershell
yunxiao --version
# 应对齐 GitHub latest；若找不到命令，新开一个终端或检查 PATH
```

## macOS / Linux

1. 打开 Releases，下载对应平台 `tar.gz`（注意 arch：amd64 / arm64）。
2. 解压，把 `yunxiao` 放到已在 `PATH` 里的目录（如 `~/.local/bin`）。
3. 同样建议保留归档旁的 `skills/`、`profiles/`（Release 从 0.16.38/#116 起会带上）。

```bash
yunxiao --version
yunxiao auth login --browser
```

## 升级：一条命令，不必再翻 zip

```bash
yunxiao update --check    # 只检查；有新版本 exit 2
yunxiao update --dry-run  # 同 --check
yunxiao update            # 有 TTY 时确认后替换；非 TTY 需 --yes
yunxiao update --yes      # 脚本/CI 无提示直接更新
```

更新时除替换二进制外，还会把归档内的 **`skills/` 与 `profiles/`** 解压到可执行文件同目录（覆盖旧目录；之后 `skills install` 的源目录随之刷新，#160）。

可选：

```bash
yunxiao doctor --check-update
```

关闭其它命令里「偶尔提示有新版本」的行为：

```bash
# Windows PowerShell
$env:YUNXIAO_UPDATE_CHECK = "0"
# bash
export YUNXIAO_UPDATE_CHECK=0
```

覆盖发布源（一般用不到）：

```bash
export YUNXIAO_CLI_GITHUB_REPO=owner/repo
export YUNXIAO_CLI_DOWNLOAD_BASE=https://example.com/path
```

## 源码安装注意：`go install` 不带 skills

若你从源码 `go install` / 本地 `go build`：

- 得到的是二进制本身；
- **不会**自动带上仓库里的 `skills/` 目录。

要给 AI Agent 用技能包，请：

- 优先用 **GitHub Release** 安装；或
- 克隆仓库后执行 `yunxiao skills install`（可 `--skill` 多选；已存在会跳过，需刷新加 `--force`）。

## 从旧 npm 渠道迁过来

1. 卸载旧 npm 包装（若还在）。
2. 按上文从 Releases 安装覆盖。
3. `yunxiao --version` 对齐 latest；再 `auth login`（凭证仍在 `~/.config/yunxiao/` 时可直接 `whoami` 试）。

## 小结

| 场景 | 做法 |
|------|------|
| 新装 Windows | `irm …/install.ps1 \| iex` |
| 新装 macOS/Linux | Releases 归档 → PATH |
| 升级 | `yunxiao update --yes` |
| Agent skills | Release 自带，或 `skills install` |
| 错误示范 | `npm i sanzhi-yunxiao-cli` |

装好后跑 `yunxiao --version` 对一下 latest，截图打卡欢迎贴评论区。下一篇：**别再网页里翻 MR**。

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

**本文相关**：安装与自更新：`install.ps1`、`yunxiao update --yes`；勿用 npm  
觉得有用请给仓库点个 **Star**，Issues 欢迎提场景。
