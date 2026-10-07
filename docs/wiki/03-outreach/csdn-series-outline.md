# CSDN 引流系列大纲

> 项目：https://github.com/xiaoxiaolin0918/yunxiao-cli  
> 二进制：`yunxiao` · 当前版本：**0.16.40**（2026-10-07）  
> **安装渠道只有 GitHub Releases**；npm 包 `sanzhi-yunxiao-cli` 已停用，文中一律不要写 `npm i` / `npx yunxiao`。

---

## 1. 目标读者与 SEO 关键词

### 目标读者
- 用阿里云云效（Yunxiao / Codeup / Flow / Projex）的国内研发、运维、测试
- 讨厌网页点点点、想把「查 MR / 跑流水线 / 搜工作项」写进脚本或 AI Agent 的人
- 从 GitHub CLI（`gh`）或旧 npm 包装迁过来的同学
- Windows 为主、偶有 macOS/Linux 的团队（文中给 PowerShell 一行安装）

### 主关键词（标题/首段必出）
| 优先级 | 关键词 |
|--------|--------|
| 核心 | 云效 CLI、yunxiao-cli、云效命令行、阿里云云效 API |
| 场景 | 云效 Codeup MR、云效流水线命令行、云效工作项、Projex CLI |
| 对比 | 云效 vs GitHub CLI、云效 MCP vs CLI |
| 安装 | 云效 CLI 安装、yunxiao update、Windows 安装云效 CLI |
| Agent | 云效 Agent skills、AI 操作云效 |

### 长尾（正文/小标题穿插）
`yunxiao auth login`、`yunxiao doctor`、`yunxiao status`、`yunxiao codeup +open-mrs`、`workitem search`、`--dry-run`、云效 PAT、云效 OAuth、profile doctor、skills install

### 系列统一调性
短句、可复制命令、先只读后写入；每篇结尾同一「安装块」；对比「云效网页点点点」制造痛点，CTA 指向 GitHub Star + Releases。

---

## 2. 系列文章标题（7 篇，可独立也可连载）

| # | 建议标题 | 类型 | 核心卖点 |
|---|----------|------|----------|
| 1 | 云效终于有「像 gh 一样」的 CLI 了：yunxiao-cli 30 秒上手 | 引流入口 / 总览 | Releases 安装 + login + status |
| 2 | Windows 一行装上云效 CLI：install.ps1 与 `yunxiao update` | 安装深挖 | 告别网页下 zip；自更新顺带 skills/profiles |
| 3 | 别再网页里翻 MR：`yunxiao codeup` 列出/合并/诊断拒绝原因 | Codeup 场景 | `+open-mrs`、merge 诊断、`--repo org/repo` |
| 4 | 用命令行管 Projex 工作项：搜索、建 Bug/需求、状态流转 | 工作项场景 | 日期窗搜索、`+bug-create` / `+transition` |
| 5 | 流水线不用盯控制台：`pipeline` 列表、待审批、`browse` 一键开页 | Flow 场景 | 只读观察 + 高风险闸门说明 |
| 6 | 给 AI Agent 用的云效：skills 安装、profile、`--dry-run` 护栏 | Agent / 差异化 | companion skills + 本地 profile |
| 7 | 云效 CLI vs 官方 MCP：脚本/CI 选谁？一张表说清 | 对比 / SEO | 可复现 vs 对话式，团队可双持 |

> 可选加餐（流量够再写）：「从旧 npm / yx 迁到 GitHub Releases」「`yunxiao doctor` + `profile doctor` 排查手册」「周报：用 `workitem search` 拉时间窗 JSON」。

---

## 3. 逐篇大纲

### 第 1 篇 · 总览引流

**标题**：云效终于有「像 gh 一样」的 CLI 了：yunxiao-cli 30 秒上手  

**SEO 首段要点**：点名「阿里云云效 CLI / yunxiao-cli / 命令行替代网页」。  

**目录要点**
1. 痛点：控制台点 MR、工作项、流水线；脚本/Agent 难复用  
2. 是什么：Go 单二进制 `yunxiao`，风格对齐飞书/Lark CLI（`+shortcuts`、typed 命令、原始 `api`、风险闸门）  
3. 30 秒路径：安装 → `auth login --browser` → `whoami` / `doctor` / `status` → `codeup +open-mrs`  
4. 模块地图（一句话）：organization / project / workitem / codeup / pipeline / packages / testhub / appstack  
5. 安全习惯：写操作先 `--dry-run`，高风险再 `--yes`  
6. **不要**用已停用的 npm 包装  

**引流钩子**
- 对比截图文案：「网页 10 次点击 vs 一条 `yunxiao status`」  
- 强调当前 **v0.16.40**：只读看板 `status`、Windows 一行安装、`update` 同步 skills/profiles  

**CTA**
- Star 仓库 + 从 Releases 安装；评论区贴「你最烦云效网页的哪一步」  

---

### 第 2 篇 · 安装与自更新

**标题**：Windows 一行装上云效 CLI：install.ps1 与 `yunxiao update`  

**目录要点**
1. 唯一渠道：GitHub Releases（说明 npm 停用原因：私有源冻旧版、缺 `update`）  
2. Windows：
   ```powershell
   irm https://raw.githubusercontent.com/xiaoxiaolin0918/yunxiao-cli/main/install.ps1 | iex
   # 指定版本：.\install.ps1 -Version 0.16.40
   ```
   默认 `%USERPROFILE%\.local\bin`，归档含 `skills/`、`profiles/` 时一并落下，并尝试前置用户 PATH  
3. macOS/Linux：下对应 `tar.gz`，解压把 `yunxiao` 放 PATH  
4. 升级：`yunxiao update --check` / `yunxiao update --yes`（校验 checksums；旁路刷新 skills、profiles）  
5. 可选：`yunxiao doctor --check-update`；关闭提示 `YUNXIAO_UPDATE_CHECK=0`  
6. 源码构建注意：`go install` **不带**仓库 `skills/`，要 skills 用 Release 或克隆后 `yunxiao skills install`  

**引流钩子**：「装完一条命令升级，不用重新翻 Releases 找 zip」。  

**CTA**：装好后跑 `yunxiao --version` 对一下 latest；成功截图评论区打卡。  

---

### 第 3 篇 · Codeup / MR

**标题**：别再网页里翻 MR：`yunxiao codeup` 列出/合并/诊断拒绝原因  

**目录要点**
1. 列仓库 / 分支 / 打开的 MR：`codeup repos list`、`+open-mrs`  
2. `--repo` 写法：数字 id、profile 别名、`org/repo` 路径（自动发现，#125）  
3. 合并前：`mrs merge --dry-run`；失败时 `subtype: merge_rejected` + `suggested_actions`（#127）  
4. 列表过滤：`--source` / `--target`（客户端滤，可配 `--all`）  
5. WIP 标题：`mrs update --wip/--unwip`  
6. 与网页对比：合并被拒时网页要自己猜，CLI 给出状态差与建议动作  

**引流钩子**：「合并被拒不再只看一句 API error」。  

**CTA**：用 `--dry-run` 贴一次诊断 JSON（打码）到评论；Star 催 MR 体验。  

---

### 第 4 篇 · 工作项 / Projex

**标题**：用命令行管 Projex 工作项：搜索、建 Bug/需求、状态流转  

**目录要点**
1. 搜索：`--created-after/before`、`--updated-*`、`--finish-*`、`--status` / `--status-stage`、`--labels`、`--all`  
2. 快捷创建：`+bug-create` / `+req-create` / `+risk-create`（MissingRequired 预检）  
3. 流转：`+transition`、`+bug-transition`（BFS 无路可走时 `--direct`）  
4. 读取：`workitem get` brief vs `--full`；评论 list/comment  
5. 标签：`project labels list|create` + `workitem search --labels`  
6. 诚实缺口：oapi 响应常无 `finishTime`，时间窗要以文档说明为准，勿编造字段  

**引流钩子**：「周报/质量窗从点选变成一条可进 CI 的命令」。  

**CTA**：附一条你们团队真实的 `workitem search`（打码 id）；引导 Star。  

---

### 第 5 篇 · 流水线 / Flow

**标题**：流水线不用盯控制台：`pipeline` 列表、待审批、`browse` 一键开页  

**目录要点**
1. `pipeline list`、运行观察（`run watch` 等，以 `--help` 为准）  
2. 待处理：`+pending` / 审批类快捷命令（写明高风险需确认 + `--yes`）  
3. `yunxiao browse pipeline --pipeline-id <id> --print-only`：终端出链接再开浏览器  
4. 与 `yunxiao status` 联动：工作项 + 打开的 MR + 可选流水线闸门一屏看  
5. 安全：变更 YAML / 闸门类操作先 `--dry-run`  

**引流钩子**：「值班时一条 `status`，比三个控制台页签快」。  

**CTA**：收藏本篇 + Star；下篇预告 Agent skills。  

---

### 第 6 篇 · AI Agent / skills / profile

**标题**：给 AI Agent 用的云效：skills 安装、profile、`--dry-run` 护栏  

**目录要点**
1. 为何适合 Agent：结构化 JSON、退出码、`--jq`、可粘贴命令  
2. `yunxiao skills install`（可 `--skill` 多选；已存在跳过，`--force` 覆盖）  
3. 技能表（点名即可）：shared / organization / project / codeup / pipeline / packages / testhub / appstack  
4. 本地 profile：`+onboard` → `YUNXIAO_PROFILE` → `profile show` / `profile doctor`  
5. doctor 能力：`allowed_*` 与线上枚举漂移检测；`--fix-suggest` / `--write` 回写（含 0.16.40 对 modules/environments）  
6. 认证提醒：浏览器 OAuth（oat- 约 1 天）；`<24h` 且无 refresh 时 `auth status`/`doctor` 会催续期；CI 用 PAT  
7. 粘贴「For AI agents」七步（安装→认证→skills→只读选项目→本地 profile→校验→写操作护栏），勿在对话里贴裸 token  

**引流钩子**：「Agent 能跑，但写入仍要你点头（`--yes`）」。  

**CTA**：欢迎提 skill 场景 Issue；Star + Watch Releases。  

---

### 第 7 篇 · CLI vs MCP 对比

**标题**：云效 CLI vs 官方 MCP：脚本/CI 选谁？一张表说清  

**目录要点**（直接复用 README 对比表，改成博客语气）
1. 形态：本地可执行文件 vs MCP Server  
2. 用途：脚本/CI/可审计 vs IDE 对话  
3. 发现方式：`--help` / `schema` / `+shortcuts` vs 工具目录  
4. 写入安全：统一 `--dry-run`/`--yes` vs 客户端确认  
5. 映射示例：`search_workitems` + 时间窗 → `workitem search --created-after …`  
6. 结论：**脚本与 CI 用 CLI；对话用 MCP；很多团队两个都留**  

**引流钩子**：「不是二选一，是把可复现的那部分留给 CLI」。  

**CTA**：两篇系列入口互链 + Star。  

---

## 4. 发文节奏（简）

| 周 | 动作 |
|----|------|
| 第 1 周 | 发第 1 + 第 2（入口 + 安装，转化最重要） |
| 第 2 周 | 发第 3 + 第 4（MR / 工作项，搜索流量大） |
| 第 3 周 | 发第 5 + 第 6（流水线 + Agent） |
| 第 4 周 | 发第 7（对比收束）；视评论补「doctor 排障」或「周报 JSON」 |
| 持续 | 跟版发「0.16.x 更新速览」短文（200–400 字），链回第 1 篇 |

配套：CSDN 专栏名建议「云效命令行 / yunxiao-cli」；标签固定：`云效` `DevOps` `CLI` `Codeup` `阿里云`；每篇首图可用终端 `yunxiao status` / `doctor` 截图。

---

## 5. 文末统一模板（每篇原文粘贴，仅改「本文相关」一句）

````markdown
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

**本文相关**：〈一句话点题，如：Codeup MR 合并诊断见上文 `mrs merge --dry-run`〉  
觉得有用请给仓库点个 **Star**，Issues 欢迎提场景。
````

---

## 6. 写作红线（避免踩坑）

- 不写 `npm i sanzhi-yunxiao-cli` / 私有 npm 源安装  
- 不鼓励在博客或 Agent 对话里粘贴裸 PAT；示例用 `"<PAT>"`  
- 写操作示例默认带 `--dry-run`  
- 命令以读者本机 `yunxiao <cmd> --help` / `schema` 为准，博客只给主路径  
- 版本号写「以 GitHub latest 为准，本文基于 0.16.40」  

---

## 附录：相对 README 的能力缺口 / 可诚实写进文的点

| 点 | 说明 | 文中怎么用 |
|----|------|------------|
| `finishTime` | 条件过滤可能可用，search/get 响应常无该字段；CLI 不会用 `updateStatusAt` 伪造 | 第 4 篇「已知限制」一小节，建立信任 |
| npm 停用 | 私有源冻旧版、缺 `update`（#115） | 第 2 篇正面讲「为什么只推 Releases」 |
| `go install` 无 skills | 源码安装不带 `skills/` | 第 2 / 6 篇提醒用 Release 或 `skills install` |
| OAuth 短时 | oat- ≈ 1 天；无 refresh 时临期提醒 | 第 6 篇认证小节 |
| OpenAPI `perPage`≤200 | 要用 `meta.total` / `has_more` / `--all` | 第 4 篇搜索小节 |
| git-bash 路径改写 | MSYS 改写 `/oapi/...`；CLI 已自动还原（#117） | 可选加餐，Windows + Git Bash 读者 |
