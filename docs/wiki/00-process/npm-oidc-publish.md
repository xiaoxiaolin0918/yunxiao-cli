# npm 薄包装与 OIDC 发版评估

## 现状

- GitHub Releases 是**主安装路径**（五平台归档 + checksums）。
- 仓库 `npm/` 包名 **`sanzhi-yunxiao-cli`**：postinstall 按平台下载 Release 二进制（薄包装，不是把 Go 编进 npm）。
- 公共 npm 裸名 `yunxiao-cli` 已被他人占用（命令 `yx`），**不要**抢该名。

## OIDC Trusted Publisher（建议）

目标：用 GitHub Actions OIDC 发布 `sanzhi-yunxiao-cli`，避免长期 npm token。

步骤（需维护者在 npmjs.com 操作一次）：

1. npm → 包 `sanzhi-yunxiao-cli` → Trusted Publisher → GitHub
2. 填 org/repo：`xiaoxiaolin0918/yunxiao-cli`，workflow 如 `npm-publish.yml`，环境可选 `npm`
3. 本仓库增加仅 `workflow_dispatch`（或 tag `v*`）的 publish job：`actions/setup-node` + `npm publish --access public`，`id-token: write`
4. 发布前校验 `npm/checksums.txt` 已与该 tag Release 对齐

**阻塞：** 当前环境未登录 npm；在 Trusted Publisher 配置完成并登录前，继续 **跳过自动 publish**，只刷 checksums。

## 文档口径

README「30 秒」已写：Releases 主路径；`npm i -g sanzhi-yunxiao-cli` 为旁路（未 publish 时用 `./npm` 本地装）。
