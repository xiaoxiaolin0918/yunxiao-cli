> **RETIRED (#115).** The npm channel (sanzhi-yunxiao-cli) is no longer used.
> Private npm registries froze the package at old versions, leaving installs without the
> update command. **GitHub Releases is the only install channel.**

# npm 薄包装（已停用）

## 现状

- 安装只走 [GitHub Releases](https://github.com/xiaoxiaolin0918/yunxiao-cli/releases/latest)（多平台归档 + checksums）。
- 仓库内的 
pm/ 包、
pm-publish workflow、scripts/npm-publish-verify.sh **已删除**；本页仅作退役记录，不再是可执行发布手册。
- 公共 npm 包名 yunxiao-cli 已被他人占用（命令 yx），**不要** 
pm i -g yunxiao-cli。

## 历史背景（勿再执行）

停用前曾用 GitHub Actions OIDC 发布薄包装到 npm，并要求「先打 tag / 建 Release → 刷 checksums → 再 npm publish」。该流程与仓库现状不符：相关目录与 workflow 已移除，**请勿**再按旧 changelog（如 0.16.36）里的 npm-publish 步骤操作。

自助升级请用：

`ash
yunxiao update
# 或重新下载最新 Release 二进制
`
