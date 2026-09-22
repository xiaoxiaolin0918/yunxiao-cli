# Bot / Agent 可交接工作模式（yunxiao-cli）

目标：换 bot / 换会话后，**不靠聊天记忆**也能继续发版与修 issue。权威信息以 **GitHub + 本仓库文档** 为准。

## 权威来源（按优先级）

1. main 分支代码与 git tag / GitHub Releases
2. 本仓库：AGENTS.md（英文入口）+ docs/wiki/**（中文细节）+ skills/**
3. GitHub Issues / PRs（含评审评论）
4. 聊天记录（易丢，只作线索）

## 常驻约束（用户偏好）

- **GitHub-only**：xiaoxiaolin0918/yunxiao-cli；Codeup 仅作产品域，不作托管远程。
- **忙**：在范围内主动实现 / 评审后合并发版，少问「可不可以做」。
- **TDD + DDD + PDCA**；改代码同步改 wiki / skill（中英分工：AGENTS 短英文，wiki 中文细讲）。
- **skills/**：命令怎么用；**wiki**：边界、陷阱、演进，避免整表复制 flag。
- **npm**：发版后刷新 
pm/checksums.txt；**默认不** 
pm publish（除非用户明确要求）。
- 云端 Cloud Agent 若连不上该仓库：在本机 checkout **就地改**（勿空等）。

## 本机仓库

- Windows 常用路径：D:\work\grokbot\yunxiao-cli
- gh 已登录本机；box 上 gh 可能未认证 → 用 **git bundle** 把提交带到本机再 push / 开 PR。

### Bundle 交接（box → Windows）

`	ext
# box
git bundle create /workspace/fix-NN-....bundle main..branch
# 拷到本机后：
git fetch <bundle> <branch>:<branch>   # 或 HEAD:branch
git checkout <branch>
git rebase origin/main
git push -u origin <branch>
gh pr create ...
`

## Issue 默认节奏

1. gh issue list --state open 扫一眼；给优先级建议后开搞。
2. 分支命名：ix/NN-... 或 eature/NN-...。
3. **TDD** → 实现 → 必要时 bump 版本（见下）→ README 双边 changelog 列表格式必须连续：
   - **0.16.x** — … (#NN)（用 em dash，勿空行打断上一条的 - ）。
4. 开 PR；请 **云效评审** + **文档评审**（Critical / Important / Minor + 是否可发版）。
5. Important 修完再合；两边放行后：gh pr merge → git tag vX.Y.Z → push tag → 等 
elease.yml → 下载 Release checksums.txt 写入 
pm/checksums.txt → commit push main。
6. **不要**在 PR 里预更新 checksums（二进制还不存在）；合入打 tag 后再刷。

## 双鉴权面（易踩坑）

| 面 | 凭证 | 典型能力 |
|----|------|----------|
| 个人令牌 OAPI | x-yunxiao-token / profile token | 多数 CLI 读写；工作项评论 **仅 list/create** |
| AccessKey RPC | ALIBABA_CLOUD_ACCESS_KEY_ID/SECRET | 如 organization members --include-aliyun-uid、workitem comments delete\|update（0.16.25+） |

缺口说明：docs/wiki/02-domains/workitem-comments-oapi-gaps.md。

## README Known gaps 写法

- 「故意未封装」表：**只列仍未封装的表面**。
- 已封装但走另一鉴权面的能力：写在 **鉴权双通道** 短注，并链 wiki；**不要**写进「未封装」表造成自相矛盾。

## 评审角色（用户侧 bot）

- **云效评审**：代码 / 发版门槛 / Critical·Important。
- **文档评审**：README / wiki / skill / changelog 一致性。
- 文档 Important（尤其 README 列表断裂、Known gaps 矛盾）**合前必改**。

## 换 bot 时 30 秒清单

`ash
cd D:\work\grokbot\yunxiao-cli   # 或本机实际路径
git fetch origin && git checkout main && git pull
git describe --tags --abbrev=0
gh issue list --state open
gh pr list --state open
`

然后读：

1. 本文件
2. AGENTS.md
3. 相关 skills/yunxiao-* / SKILL.md
4. 未合并 PR 的描述与最新评审评论

## 当前快照（请发版或合 PR 后更新本段）

- 最近发版：0.16.24（#69 content-file + OAPI 评论缺口文档）
- 进行中：PR [#72](https://github.com/xiaoxiaolin0918/yunxiao-cli/pull/72) → 0.16.25（#71 AccessKey 评论 delete/update）；文档 Important 已修 cc396af，等复扫 / 云效评审
- npm publish：仍默认跳过

