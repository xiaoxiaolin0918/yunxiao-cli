# Codeup `--repo` 引用形态：org/repo 路径、裸名自动发现与别名注册（#125）

## 背景

`--repo` 引用仓库对 agent/shell 不友好（#125 实证，2026-10-06）：

- 别名未注册直接失败，报错没有可复制的后续动作；
- `org%2Frepo` URL 编码路径反直觉：escape 一次不对就失败，bash/PowerShell 里 `/` vs `%2F` 极易踩坑。

## 支持的形态（按解析顺序）

1. **数字 `repositoryId`** —— 原样透传，无网络请求。
2. **`profile.repositories` 别名** —— 命中即映射为数字 id。
3. **`org/repo`、`org/group/repo` 路径** —— 直接写斜杠；CLI 内部用 `client.EncodeRepoID` 做 URL 编码（也接受已编码的 `%2F` 形式，原样透传）。**不要**自己转义成 `%2F`。
4. **裸仓库名**（无 `/`、无 `%2f`、非数字、非已注册别名）—— CLI 发一次只读 `GET .../repositories?search=<name>`（分页跟随，上限 5 页 × 20 条；`--dry-run` 下**也会发**，同 #93/#95 只读预检约定）：
   - 组织内唯一命中（按 `name` 或 `pathWithNamespace`/`path` 末段精确匹配）→ 解析为该仓数字 id；
   - 多命中 → 报错列出全部候选（`id` + 路径），并附 `--repo <id|org/repo>` 与 `yunxiao profile repo-add` 提示；
   - 零命中 / 无法完成查找（缺凭证、网络失败）→ 返回原「unknown repository alias」错误（内含可复制的注册命令一行），并附查找失败原因。

## 别名注册：`yunxiao profile repo-add`

```
yunxiao profile repo-add <alias> <repo> [--force] [--profile <name>]
```

- `Risk: write`（本地 profile 文件写入，非 API 变更；无 `--yes` 门禁）。
- `<repo>` 接受上述全部形态；存储值恒为解析后的**数字 id**：
  - 数字 → 原样存储（不发网络请求）；
  - `org[/group]/repo` 路径 → 一次只读 `GET .../repositories/{repo}` 取响应 `id`；
  - 已注册别名 → 复用其数字 id（可用来给别名改名）；
  - 裸名 → 走唯一命中自动发现。
- 解析用的只读 GET 在 `--dry-run` 下也会发（预览最终写入值），但**不写文件**。
- 已存在且指向不同 id 的别名需要 `--force` 才覆盖；相同 id 幂等成功（`unchanged: true`）。

## 脚本/agent 建议

- 稳定脚本：先用 `yunxiao profile repo-add` 注册别名（或直接数字 id）——无额外网络请求、不受组织内重名影响。
- 交互/调试：`org/repo` 斜杠路径或裸名都可以；多命中报错里直接给了候选 id。
- 未知别名的报错 stderr 里带可整行复制的 `yunxiao profile repo-add <alias> <repo-id-or-org/repo-path>`。

相关：`internal/zhiyi.ResolveRepositoryID` / `IsBareRepoName` / `MatchRepoCandidates`，`cmd.resolveCodeupRepoContext` / `discoverRepositoryIDByName`（cmd/helpers.go），`cmd/profile_repo_add.go`。
