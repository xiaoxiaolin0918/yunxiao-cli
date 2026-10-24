# 多身份：per-profile credentials（#380）

目标：**不同 profile = 不同云效身份**。`yunxiao auth login --profile <name>` 把凭证绑到该 profile；之后 `--profile` / `YUNXIAO_PROFILE` / `profile use` 切换时自动用对应身份。

## Token 优先级

1. `YUNXIAO_ACCESS_TOKEN` 环境变量（强制覆盖，适合 CI 单身份）
2. `~/.config/yunxiao/credentials.json` 的 **`profiles[<name>]`**（当前 active profile 有名字时）
3. 同文件的 **`active`**（未选 profile 时的遗留全局槽）
4. profile JSON 里可选的 `access_token` 字段
5. `~/.config/yunxiao/config.json` 的 `access_token`（更旧遗留）

**有 profile 名时不会回落到全局 `active`**，避免「login 到 A 却被上次全局 login 盖掉」。

## login / logout 目标（不对称，#380）

| 命令 | 分槽目标怎么定 | 三者皆空时 |
|------|----------------|------------|
| `auth login` | `--profile` > `YUNXIAO_PROFILE` > **config 默认**（`profile use` 写入的 `config.json` `profile`） | 写入全局 `credentials.active`（并同步遗留 `config.json` token） |
| `auth logout` | **仅** `--profile` / `YUNXIAO_PROFILE`（**不用** config 默认，避免误清） | 清全局 `active` + 遗留 `config.json` token |

因此：若已 `yunxiao profile use work`，裸跑 `yunxiao auth login --token …`（无 flag/env）会写入 `credentials.profiles[work]`，**不是**全局 Active。要清该槽须显式 `auth logout --profile work`（或设 `YUNXIAO_PROFILE=work`）；裸 `auth logout` 只动全局 Active。

## 常用命令

```bash
# 绑到 profile（PAT）
yunxiao auth login --token "<PAT>" --profile work
yunxiao auth login --token "<PAT>" --profile personal

# 或浏览器 OAuth
yunxiao auth login --browser --profile work

# 已 profile use work 时，裸 login 也绑到 work（见上表）
yunxiao profile use work
yunxiao auth login --token "<PAT>"

# 切身份看状态（永不打印原始 token）
yunxiao auth status --profile work
yunxiao auth status --profile personal
# 输出含 token_source / credential_profile / token_masked

yunxiao doctor --profile work   # token 检查同样带 credential_profile

# 只清某一个 profile 槽（须 flag 或 env；不动其它 profile / 全局 active）
yunxiao auth logout --profile work
```

## 与 env 的关系

- 设了 `YUNXIAO_ACCESS_TOKEN` 时，**所有 profile 都用这一份**（CI 单身份场景）。
- 本机多身份：不要设该 env，靠 `--profile` 切换即可。

## 示例勿写真实 token

文档 / skill / 回复里只用占位符（`<PAT>`）或 `token_masked`；禁止把完整 token 写进仓库或复活包。

## 相关

- `yunxiao auth --help` / `auth login --help`
- skill：`skills/yunxiao-shared/SKILL.md`（认证节）
- 使用索引：[README.md](./README.md)

## doctor / auth status 诚实字段（#389）

`can_refresh: true` **仅表示** credentials 里同时有 `refresh_token` + `client_id`，**不代表** token endpoint 已探活成功。

| 字段 | 含义 |
|------|------|
| `refresh_token_present` | 本地是否存有 refresh_token |
| `can_refresh` | 本地是否具备换票条件（refresh_token + client_id） |
| `can_refresh_note` | `not_probed` / `probed_ok` / `probed_fail` — 是否已实际请求 token endpoint |
| `refresh_attempted` / `refresh_ok` | 本次是否发起过换票；若发起则 `refresh_ok` 表示成败 |
| `connectivity.api_ok` | doctor 对 `/platform/user` 探活是否成功 |
| `auth_refresh` | `not_needed` \| `skipped` \| `refreshed` \| `refresh_failed` |

要点：

1. **空跑 `yunxiao auth status` / 空跑 `yunxiao doctor` 不保证换票。** status 永不打 token endpoint（`can_refresh_note=not_probed`）。
2. doctor 在 access 临近过期时会静默 refresh；若静默路径为 `not_needed` 且 API 探活返回 **401**，会 **再强制 refresh 一次后重试**。
3. 需要主动探活时：`yunxiao auth refresh` 或 `yunxiao doctor --probe-refresh`。
4. `auth refresh` 失败时会给出机读 `oauth_error`（如 `invalid_grant`）；此时需重新 `yunxiao auth login --browser`。
5. Agent/CI 遇 401：先打真实 API 或 `auth refresh`，**勿单信 `can_refresh`**。

```bash
yunxiao auth refresh --profile <name>
yunxiao doctor --profile <name> --probe-refresh
yunxiao auth status --profile <name>   # 仅本地元数据；can_refresh_note=not_probed
```
