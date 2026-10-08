# CLI 顶层命令树（自动对照）

本文件由 `scripts/check_command_docs.py` 对照 `cmd/` 里 `rootCmd.AddCommand` 生成/校验。
新增顶层命令时：更新本表，并确保 `docs/wiki/` 或 `AGENTS.md` 某处提到该命令名。

| 命令 | Go 变量 |
|------|---------|
| `onboard` | `onboardCmd` |
| `programs` | `programsCmd` |
| `auth` | `authCmd` |
| `config` | `configCmd` |
| `profile` | `profileCmd` |
| `doctor` | `doctorCmd` |
| `browse` | `browseCmd` |
| `alias` | `aliasCmd` |
| `whoami` | `whoamiCmd` |
| `api` | `apiCmd` |
| `schema` | `schemaCmd` |
| `skills` | `skillsCmd` |
| `organization` | `organizationCmd` |
| `project` | `projectCmd` |
| `sprint` | `sprintCmd` |
| `versions` | `versionsCmd` |
| `workitem` | `workitemCmd` |
| `codeup` | `codeupCmd` |
| `pipeline` | `pipelineCmd` |
| `packages` | `packagesCmd` |
| `testhub` | `testhubCmd` |
| `appstack` | `appstackCmd` |
| `status` | `statusCmd` |
| `update` | `updateCmd` |

生成命令：`python scripts/check_command_docs.py --write-tree`
