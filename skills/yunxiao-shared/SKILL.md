---
name: yunxiao-shared
version: 1.1.2
description: "Use for yunxiao CLI setup/auth: auth login/status/logout, config, doctor, whoami, self-update (yunxiao update), JSON output contract (ok==true), list meta.has_more/total/page, meta.url, refresh_ok, --dry-run, high-risk --yes confirmation (exit 10), or handling error envelopes."
metadata:
  requires:
    bins: ["yunxiao"]
  cliHelp: "yunxiao --help"
---

# yunxiao 共享规则

所有 `yunxiao-*` skill 共享的底座：认证、输出契约与高风险操作。

## 通用准则

1. **调用前先确认用法**：执行前读对应 skill 或跑 `yunxiao <domain> --help` / `yunxiao schema <id>`，别猜 flag 盲调。
2. **优先 +shortcut**：有 `+` 快捷命令时优先于 typed API；typed 不够时再用 `yunxiao api METHOD /path`。
3. **默认 JSON 输出**：判断成功用 `ok == true`（或进程退出码 0），**不要**用 `code == 0`。

- Topic/Risk 创建不要带 `--sprint`；Topic ↔ Req 等普通关联优先使用 `ASSOCIATED`（依赖使用 `DEPEND_ON`）。

## 认证

```bash
export YUNXIAO_ACCESS_TOKEN="<PAT>"          # 推荐（CI / 会话）
# 或
yunxiao auth login --token "<PAT>"           # 写入 ~/.config/yunxiao/config.json (0600)
yunxiao auth status
yunxiao whoami
yunxiao doctor
```

可选：`YUNXIAO_ORGANIZATION_ID`、`YUNXIAO_API_BASE_URL`（默认 `https://openapi-rdc.aliyuncs.com`）、`YUNXIAO_EDITION=central|region`。也可在 profile JSON 写可选 `access_token`（优先级：env > profile > config.json）。

PAT 控制台（首选）：https://account-devops.aliyun.com/settings/personalAccessToken
帮助：https://help.aliyun.com/zh/yunxiao/user-guide/personal-access-token

推荐模块权限：组织/成员读；Projex/Codeup/Flow 读+写（试用可只读）；Packages/Testhub/AppStack 按需。令牌名建议 `yunxiao-cli`。无飞书式一键 OAuth（`CreateOAuthToken` 仍内测）。

**禁止**把完整 token 打到终端或回复里；只用 `token_masked` / `auth status`。

## 输出契约

成功（stdout，exit 0）：

```json
{ "ok": true, "data": { }, "meta": { } }
```

错误（stderr，exit ≠ 0）：

```json
{ "ok": false, "error": { "type": "...", "message": "...", "hint": "..." } }
```

`--jq '<expr>'` 过滤 JSON；`--format pretty` 缩进。

### 列表分页 meta（CLI 0.15.x）

List 响应的 `meta` 在有分页头时可能包含：

| 字段 | 含义 |
|------|------|
| `has_more` | 是否还有下一页 |
| `total` / `page` | 总数 / 当前页（有则出现） |
| `pagination` | 嵌套完整 `x-page`/`x-per-page`/`x-total`/`x-next-page` 等 |

**Agents 不得把第一页当成全集**：当 `has_more == true` 时必须继续翻页（或使用已接线的 `--all`）；当这些字段**缺席**时，也**不等于**已完整——只能说明本响应没有分页头。部分 list（如 `pipeline list --all`、`codeup mrs list --all`）会用 `client.ListAll` 自动跟页（上限 50 页）。

### 常见 `meta.url`

workitem get/create/update/transition、MR get/create、pipeline get/run 等常在 `meta.url`（或列表项 `url`）给出控制台可点击链接。有则优先展示给用户。

### `refresh_ok`（流转成功后）

`+transition` / `+bug-transition` 成功信封可能含 `refresh_ok`：流转 PUT 已成功；若随后 GET 刷新失败则为 `false`，并在 **stderr** 打 warning。**不要把 `refresh_ok:false` 当成流转失败**（`ok` 仍为 `true`）。


## 本地 Profile 初始化（+onboard）

同事/Agent 首次接入优先：

```bash
yunxiao +onboard                          # TTY：列项目并编号选择
yunxiao +onboard --space-id <id> --profile <name>   # 非交互
yunxiao +onboard --space-id <id> --dry-run
```

写入位置：`~/.config/yunxiao/profiles/<name>.json`（仅 `name` + `space_id` 的通用模板）。**不要**把智衣/沙箱租户 profile 提交进本公开仓库；也不要把 `profile install-example zhiyi|play` 当作同事默认路径。缺 token 时提示控制台：https://account-devops.aliyun.com/settings/personalAccessToken 并打印模块权限清单

## 风险与确认

| Risk | 行为 |
|------|------|
| `read` | 可直接执行 |
| `write` | 先确认用户意图；支持时先 `--dry-run` |
| `high-risk-write` | **必须**用户显式同意后再加 `--yes`；缺省时 exit **10** |

高风险缺 `--yes` 时 stderr 形如：

```json
{
  "ok": false,
  "error": {
    "type": "confirmation",
    "subtype": "confirmation_required",
    "message": "...",
    "hint": "add --yes to confirm",
    "risk": "high-risk-write",
    "action": "codeup mrs create"
  }
}
```

流程：识别 exit 10 → **向用户展示 action/risk/关键参数并等待同意** → 同意后再把 `--yes` 追加到原 argv 重试。**禁止**静默加 `--yes`。

`--dry-run` 只预览请求（URL/body），不触发确认门禁、不发写请求。

## 自更新

```bash
yunxiao update --check          # 仅检查；有新版本则 exit 2
yunxiao update                  # TTY：确认后替换本地二进制
yunxiao update --yes            # 非交互直接更新（脚本/CI）
```

其他命令偶尔在 **stderr** 打印中文提示（最多每 24h 一次网络检查）：`发现新版本 yunxiao：x → y。运行：yunxiao update`。跳过：`--format json`（默认）、`update`/`self-update`/`completion`。关闭：`YUNXIAO_UPDATE_CHECK=0`。**禁止**静默加 `--yes`。

## Reference

- 输出与确认细节 → [`references/output-and-risk.md`](references/output-and-risk.md)


## Browse console

- `yunxiao browse pipeline|workitem|mr|repo|url` �� open console; `--print-only` for CI/Agent.
- Migration from gh/yx: `docs/wiki/00-process/gh-yx-migration.md`.

## Alias

- `yunxiao alias set <name> <cmd> [args...]` — must not embed `--yes`/`-y`
- File: `~/.config/yunxiao/aliases.json`
