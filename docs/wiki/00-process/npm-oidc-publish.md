# npm 薄包装与 OIDC 发版

## 现状

- GitHub Releases 是**主安装路径**（五平台归档 + checksums）。
- 仓库 `npm/` 包名 **`sanzhi-yunxiao-cli`**：postinstall 按平台下载 Release 二进制（薄包装，不是把 Go 编进 npm）。
- 公共 npm 裸名 `yunxiao-cli` 已被他人占用（命令 `yx`），**不要**抢该名。

## 发布顺序（必读，#105）

约定：**先打 tag / 出 Release，再刷 `npm/checksums.txt`，最后才 npm publish**。因此 **不要**在 release tag 上跑 publish——tag 树里的 checksums 永远是上一版。

| 步骤 | 做什么 | 谁 |
|------|--------|-----|
| 1 | 合入版本 bump PR → `git tag vX.Y.Z` → push tag | 维护者 |
| 2 | 等 `release.yml` 产出五平台归档 + Release 资产 `checksums.txt` | Actions |
| 3 | 下载 Release `checksums.txt` → 写入 `npm/checksums.txt` → `chore/checksums-X.Y.Z` PR 合入 `main` | 维护者 / bot |
| 4 | 手动触发 `npm-publish`：`tag=vX.Y.Z`，`ref=main`（或 checksums 提交 SHA） | 仅晓霖明确要求时 |

`npm-publish.yml` 会：

1. **checkout `ref`（默认 `main`）**，而不是 checkout tag；
2. 断言 `npm/package.json` 的 `version` == tag 去掉 `v`；
3. 断言 `npm/checksums.txt` 含 `yunxiao-cli-${ver}-`；
4. 用 `gh release download` 把 Release 资产 `checksums.txt` 与仓库文件 `diff`（必须一致）；
5. `npm publish --access public --provenance`（可勾选 `dry_run` 只做 `--dry-run`）。

本地等效校验（不写 registry）：

```bash
# 在已刷新 checksums 的提交上：
ver=0.16.29
test "$(node -p "require('./npm/package.json').version")" = "$ver"
grep -F "yunxiao-cli-${ver}-" npm/checksums.txt
gh release download "v${ver}" -p checksums.txt -O /tmp/release-checksums.txt
diff -u /tmp/release-checksums.txt npm/checksums.txt
(cd npm && npm publish --dry-run)
```

或触发 Actions：`gh workflow run npm-publish.yml -f tag=v0.16.29 -f ref=main -f dry_run=true`。

**不要**在功能 PR 里预更新 checksums（二进制还不存在）。Windows / CRLF 打包与 Windows CI job 见 #108（从本 issue 拆出）。

## OIDC Trusted Publisher（建议）

目标：用 GitHub Actions OIDC 发布 `sanzhi-yunxiao-cli`，避免长期 npm token。

步骤（需维护者在 npmjs.com 操作一次）：

1. npm → 包 `sanzhi-yunxiao-cli` → Trusted Publisher → GitHub
2. 填 org/repo：`xiaoxiaolin0918/yunxiao-cli`，workflow 如 `npm-publish.yml`，环境可选 `npm`
3. 本仓库保持仅 `workflow_dispatch` 的 publish job：`actions/setup-node` + `npm publish --access public`，`id-token: write`
4. 发布前校验 `npm/checksums.txt` 已与该 tag Release 对齐（workflow 已自动做）

**阻塞：** 当前环境未登录 npm；在 Trusted Publisher 配置完成并登录前，继续 **跳过自动 publish**，只刷 checksums。

## 过渡态（当前 workflow）

仓库里的 `.github/workflows/npm-publish.yml` 已声明 `id-token: write` 与 `--provenance`，**同时**仍传入 `NODE_AUTH_TOKEN: ${{ secrets.NPM_TOKEN }}`。

| 阶段 | 怎么发 |
|------|--------|
| Trusted Publisher **未**在 npmjs 配好 | 需要仓库 Secret `NPM_TOKEN`（经典 token）；否则 `npm publish` 会鉴权失败 |
| Trusted Publisher **已**生效 | 应去掉 `NPM_TOKEN` / `NODE_AUTH_TOKEN`，只靠 OIDC + provenance；workflow 可改为不再引用该 secret |

不要按「已经是纯 OIDC」理解当前 YAML：在 Publisher 配好之前，**仍依赖** `NPM_TOKEN`。配好后请改 workflow 删掉经典 token，避免双轨混淆。

## 文档口径

README「30 秒」已写：Releases 主路径；`npm i -g sanzhi-yunxiao-cli` 为旁路（未 publish 时用 `./npm` 本地装）。
