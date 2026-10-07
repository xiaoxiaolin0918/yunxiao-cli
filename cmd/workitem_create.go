package cmd

import (
	"github.com/spf13/cobra"
	"github.com/yunxiao-cli/yunxiao/internal/client"
	"github.com/yunxiao-cli/yunxiao/internal/profile"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
	"github.com/yunxiao-cli/yunxiao/internal/zhiyi"
)

var workitemCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a work item",
	Long: `Risk: write
HTTP: POST .../workitems

Success prints a brief work item (id/serialNumber/status.displayName/subject);
pass --full for the raw object. When create returns null serialNumber/status,
CLI re-GETs the item so the success payload stays usable (#62).

When an active profile has workitem_defaults for --type-id, create fills
priority / trackers / 测试负责人 / 验收负责人 (and other defaults) unless
already set by flags / --custom-fields. Pass --no-defaults to skip.

If the API returns 未启用此字段【迭代】, omit --sprint for this workitem type
(Topic/Risk and some custom types do not enable 迭代).

Required-field precheck (0.16.33+, #95): before POST (also under --dry-run) the CLI
sends one read-only GET .../workitemTypes/{typeId}/fields (same as workitem fields) and
compares required fields with the final body (flags, --description-file /
--custom-fields-file, profile workitem_defaults). All missing fields are reported at
once (exit 1, error.subtype missing_required_fields, error.details.missing[] with
field_id / name / pass_via / options) and nothing is POSTed. Skipped: optional fields,
showWhenCreate=false, server-managed (status, creator, …) and fields with a server
defaultValue (listed in meta.precheck.skipped_default). Root fields (subject, sprint,
labels, …) count only when passed via their flag, custom fields only via
--custom-fields(-file). Success / dry-run carry meta.precheck (or request.precheck)
{status: ok, source: fields, required_checked}.
401 on the fields GET fails the command. Otherwise, if the config cannot be read (HTTP
error, network, unexpected payload, no answer within 10s; at most 1 retry, backoff
<= 1s) status=skipped, or if it is empty status=empty: the create proceeds,
meta.precheck (dry-run: request.precheck) carries reason, hint and warning, and the
warning is also printed as one "warning: ..." line on stderr. If the POST then fails,
error.hint adds "precheck skipped: <reason>". If the profile has
workitem_defaults[type].create_required, those ids are checked instead
(source=profile_fallback, profile_missing[]), warn-only.
--no-precheck skips the GET (old behavior; use offline). Values you pass are never
changed; other server validation errors pass through unchanged.

Windows / PowerShell: for Chinese subject, description, or custom-fields JSON,
prefer --subject-file / --description-file / --custom-fields-file (UTF-8, BOM
stripped) over inline flags. Use only one of each pair (--custom-fields vs
--custom-fields-file, etc.).`,
	Example: `  yunxiao workitem create --space-id <space-id> --type-id <type-id> --subject "…" --assigned-to self --dry-run
  yunxiao workitem types list --space-id <space-id> --category Risk   # find --type-id`,
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		spaceID, _ := cmd.Flags().GetString("space-id")
		typeID, _ := cmd.Flags().GetString("type-id")
		subjectFlag, _ := cmd.Flags().GetString("subject")
		subjectFile, _ := cmd.Flags().GetString("subject-file")
		assignedTo, _ := cmd.Flags().GetString("assigned-to")
		descriptionFlag, _ := cmd.Flags().GetString("description")
		descriptionFile, _ := cmd.Flags().GetString("description-file")
		formatType, _ := cmd.Flags().GetString("format-type")
		parentID, _ := cmd.Flags().GetString("parent-id")
		sprint, _ := cmd.Flags().GetString("sprint")
		labels, _ := cmd.Flags().GetString("labels")
		subject, err := readFlagOrFile(subjectFlag, subjectFile, "subject", true)
		if err != nil {
			handleErr(err)
			return
		}
		description, err := readFlagOrFile(descriptionFlag, descriptionFile, "description", false)
		if err != nil {
			handleErr(err)
			return
		}
		cfJSON, _ := cmd.Flags().GetString("custom-fields")
		cfFile, _ := cmd.Flags().GetString("custom-fields-file")
		cf, err := readJSONMapFlagOrFile(cfJSON, cfFile, "custom-fields")
		if err != nil {
			handleErr(err)
			return
		}
		if err := requireFlags("space-id", spaceID, "type-id", typeID, "assigned-to", assignedTo); err != nil {
			handleErr(err)
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		assignedTo, err = resolveSelfID(cmd.Context(), c, assignedTo)
		if err != nil {
			handleErr(err)
			return
		}
		body := map[string]any{
			"spaceId":        spaceID,
			"workitemTypeId": typeID,
			"subject":        subject,
			"assignedTo":     assignedTo,
		}
		if description != "" {
			body["description"] = description
		}
		if cf != nil {
			body["customFieldValues"] = cf
		}
		if formatType != "" {
			body["formatType"] = formatType
		}
		if parentID != "" {
			body["parentId"] = parentID
		}
		if sprint != "" {
			body["sprint"] = sprint
		}
		if labels != "" {
			body["labels"] = splitCSV(labels)
		}
		participants, _ := cmd.Flags().GetString("participants")
		trackers, _ := cmd.Flags().GetString("trackers")
		verifier, _ := cmd.Flags().GetString("verifier")
		versions, _ := cmd.Flags().GetString("versions")
		if participants != "" {
			body["participants"] = splitCSV(participants)
		}
		if trackers != "" {
			body["trackers"] = splitCSV(trackers)
		}
		if verifier != "" {
			v, err := resolveSelfID(cmd.Context(), c, verifier)
			if err != nil {
				handleErr(err)
				return
			}
			body["verifier"] = v
		}
		if versions != "" {
			body["versions"] = splitCSV(versions)
		}
		noDefaults, _ := cmd.Flags().GetBool("no-defaults")
		if !noDefaults {
			if pf, err := applyActiveProfileOrg(); err != nil {
				handleErr(err)
				return
			} else if pf != nil {
				profile.ApplyWorkitemDefaults(body, typeID, pf)
			}
		}
		path, err := c.ProjexPath(cmd.Context(), "/workitems")
		if err != nil {
			handleErr(err)
			return
		}
		full, _ := cmd.Flags().GetBool("full")
		// #95: one-shot required-field precheck against the type's field config.
		var precheck map[string]any
		if noPrecheck, _ := cmd.Flags().GetBool("no-precheck"); !noPrecheck {
			// Fallback ids when the field config is unusable (read even with --no-defaults;
			// a broken profile was already reported above when defaults are on).
			var profileRequired []string
			if pf, perr := applyActiveProfileOrg(); perr == nil && pf != nil {
				profileRequired = pf.WorkitemDefaults[typeID].CreateRequired
			}
			precheck, err = precheckWorkitemCreate(cmd.Context(), c, spaceID, typeID, body, profileRequired)
			if err != nil {
				handleErr(err)
				return
			}
			printPrecheckWarning(precheck)
		}
		var preview any = c.Preview("POST", path, nil, body)
		if precheck != nil {
			preview = requestPreviewWithPrecheck{RequestPreview: preview.(client.RequestPreview), Precheck: precheck}
		}
		err = runJSONMutatingPreview(cmd.Context(), c, "workitem create", risk.Write, "POST", path, nil, body, preview, func(out any, meta map[string]any) (any, map[string]any) {
			if precheck != nil {
				meta["precheck"] = precheck
			}
			item := asStringMap(out)
			item = zhiyi.EnsureWorkItemCreateFields(item, func(id string) (map[string]any, error) {
				return fetchWorkItemMap(cmd.Context(), c, id)
			})
			zhiyi.EnrichWorkItemMeta(meta, item, profileSpaceID(), spaceID)
			if full {
				return item, meta
			}
			return zhiyi.BriefWorkItem(item), meta
		})
		handleCreateErr(err, precheck)
	},
}
