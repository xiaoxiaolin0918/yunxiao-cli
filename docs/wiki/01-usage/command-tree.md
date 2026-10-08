# CLI 命令树（自动对照）

本文件由 `scripts/check_command_docs.py` 对照 `cmd/` 里 `AddCommand` 生成/校验。
新增顶层或下列父命令的子命令时：更新本表，并确保 `docs/wiki/` 或 `AGENTS.md` 某处提到该命令名。

## 顶层

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

## `alias` 子命令

| 命令 | Go 变量 |
|------|---------|
| `alias set` | `aliasSetCmd` |
| `alias list` | `aliasListCmd` |
| `alias delete` | `aliasDeleteCmd` |

## `auth` 子命令

| 命令 | Go 变量 |
|------|---------|
| `auth login` | `authLoginCmd` |
| `auth status` | `authStatusCmd` |
| `auth logout` | `authLogoutCmd` |
| `auth probe-oauth` | `authProbeCmd` |
| `auth refresh` | `authRefreshCmd` |

## `codeup` 子命令

| 命令 | Go 变量 |
|------|---------|
| `codeup repos` | `codeupReposCmd` |
| `codeup branches` | `codeupBranchesCmd` |
| `codeup files` | `codeupFilesCmd` |
| `codeup commits` | `codeupCommitsCmd` |
| `codeup compare` | `codeupCompareCmd` |
| `codeup mrs` | `codeupMrsCmd` |
| `codeup protected-branches` | `codeupProtectedBranchesCmd` |
| `codeup tags` | `codeupTagsCmd` |

## `pipeline` 子命令

| 命令 | Go 变量 |
|------|---------|
| `pipeline list` | `pipelineListCmd` |
| `pipeline get` | `pipelineGetCmd` |
| `pipeline create` | `pipelineCreateCmd` |
| `pipeline update` | `pipelineUpdateCmd` |
| `pipeline diff` | `pipelineDiffCmd` |
| `pipeline run` | `pipelineRunCmd` |
| `pipeline job` | `pipelineJobCmd` |
| `pipeline service-connections` | `pipelineSCCmd` |
| `pipeline host-groups` | `pipelineHGCmd` |
| `pipeline flow-variable-groups` | `pipelineFlowVGCmd` |
| `pipeline resource-members` | `pipelineRMCmd` |
| `pipeline runner-groups` | `pipelineRunnerGroupsCmd` |
| `pipeline vm-deploy` | `pipelineVMDeployCmd` |

## `skills` 子命令

| 命令 | Go 变量 |
|------|---------|
| `skills list` | `skillsListCmd` |
| `skills read` | `skillsReadCmd` |
| `skills path` | `skillsPathCmd` |
| `skills install` | `skillsInstallCmd` |

## `workitem` 子命令

| 命令 | Go 变量 |
|------|---------|
| `workitem efforts` | `workitemEffortsCmd` |
| `workitem estimated-efforts` | `workitemEstimatedCmd` |
| `workitem bug-transition` | `workitemBugTransitionCmd` |
| `workitem bug-create` | `workitemBugCreateCmd` |
| `workitem explore-workflow` | `workitemExploreWorkflowCmd` |
| `workitem search` | `workitemSearchCmd` |
| `workitem get` | `workitemGetCmd` |
| `workitem comments` | `workitemCommentsCmd` |
| `workitem comment` | `workitemCommentCmd` |
| `workitem create` | `workitemCreateCmd` |
| `workitem update` | `workitemUpdateCmd` |
| `workitem delete` | `workitemDeleteCmd` |
| `workitem relations` | `workitemRelationsCmd` |
| `workitem attachments` | `workitemAttachmentsCmd` |
| `workitem fields` | `workitemFieldsCmd` |
| `workitem workflow` | `workitemWorkflowCmd` |
| `workitem statuses` | `workitemStatusesCmd` |
| `workitem activities` | `workitemActivitiesCmd` |
| `workitem types` | `workitemTypesCmd` |
| `workitem transition` | `workitemTransitionCmd` |

生成命令：`python scripts/check_command_docs.py --write-tree`
