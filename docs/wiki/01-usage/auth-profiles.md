# 多身份：per-profile credentials（#380）

目标：**不同 profile = 不同云效身份**。`yunxiao auth login --profile <name>` 把凭证绑到该 profile；之后 `--profile` / `YUNXIAO_PROFILE` / `profile use` 切换时自动用对应身份。

## Token 优先级

1. `YUNXIAO_ACCESS_TOKEN` 环境变量（强制覆盖，适合 CI 单身份）
2. `~/.config/yunxiao/credentials.json` 的 **`profiles[<name>]`**（当前 active profile 有名字时）
3. 同文件的 **`active`**（未选 profile 时的遗留全局槽）
4. profile JSON 里可选的 `access_token` 字段
5. `~/.config/yunxiao/config.json` 的 `access_token`（更旧遗留）

**有 profile 名时不会回落到全局 `active`**，避免「login 到 A 却被上次全局 login 盖掉」。

## 常用命令

```bash
# 绑到 profile（PAT）
yunxiao auth login --token "<PAT>" --profile work
yunxiao auth login --token "<PAT>" --profile personal

# 或浏览器 OAuth
yunxiao auth login --browser --profile work

# 切身份看状态（永不打印原始 token）
yunxiao auth status --profile work
yunxiao auth status --profile personal
# 输出含 token_source / credential_profile / token_masked

yunxiao doctor --profile work   # token 检查同样带 credential_profile

# 只清某一个 profile 槽（不动其它 profile / 全局 active）
yunxiao auth logout --profile work
```

无 `--profile` / `YUNXIAO_PROFILE` 时：`auth login` 写入全局 `active`；`auth logout` 清全局 `active` + 遗留 `config.json` token。

> `auth logout` **不会**用 `profile use` 写入的 config 默认名当分槽目标，避免误清。要清默认 profile 的槽，请显式 `--profile <name>` 或设 `YUNXIAO_PROFILE`。

## 与 env 的关系

- 设了 `YUNXIAO_ACCESS_TOKEN` 时，**所有 profile 都用这一份**（CI 单身份场景）。
- 本机多身份：不要设该 env，靠 `--profile` 切换即可。

## 示例勿写真实 token

文档 / skill / 回复里只用占位符（`<PAT>`）或 `token_masked`；禁止把完整 token 写进仓库或复活包。

## 相关

- `yunxiao auth --help` / `auth login --help`
- skill：`skills/yunxiao-shared/SKILL.md`（认证节）
- 使用索引：[README.md](./README.md)
