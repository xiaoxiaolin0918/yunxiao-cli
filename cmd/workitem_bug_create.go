package cmd

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/profile"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
	"github.com/yunxiao-cli/yunxiao/internal/zhiyi"
)

// expectedCompletionDateRe validates --expected-completion YYYY-MM-DD.
var expectedCompletionDateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

var workitemBugCreateCmd = &cobra.Command{
	Use:   "+bug-create",
	Short: "Shortcut: create a bug with profile field maps",
	Long: `Risk: write (requires --yes for real run; prefer --dry-run first)

Needs active profile with space_id + bug_type_id + bug_create_fields (priority/seriousLevel alias→option id maps). Unmapped aliases fail client-side (no API 400).

Optional create fields (module / environment / ExpCompletionTime) are sent only when
configured on the profile. Omit them in play/sandbox profiles, or pass --minimal to
force subject/description/priority/seriousLevel/sprint/assignedTo only.

  # Zhiyi (full fields)
  yunxiao workitem +bug-create --profile zhiyi \
    --title "标题" --description "描述" --expected-completion 2026-09-20 \
    --sprint <id> --dry-run

  # Sandbox / non-Zhiyi (play)
  yunxiao workitem +bug-create --profile play \
    --title "标题" --description "描述" --sprint <id> --dry-run

  yunxiao workitem +bug-create --minimal --title "…" --description "…" --sprint <id> --yes

If --sprint is omitted, searches recent Bug sprints and errors with a suggestion (does not create).

Defaults: --environment 测试环境, --module MES, --priority high, --serious-level normal,
--assigned-to from profile.default_assigned_to (or self),
--verifier from flag → profile.default_verifier → workitem_defaults[bug_type_id].verifier
(warn on stderr if still unset — SOP expects a verifier at create time).

--serious-level aliases: fatal / severe (synonym: serious) / normal / slight (synonym: minor).
Help lists the profile map keys; serious↔severe and minor↔slight are accepted interchangeably.

When profile.workitem_defaults has an entry for bug_type_id, create also pulls
priority/trackers/verifier/测试负责人/验收负责人 (etc.) unless already set by BuildCreateBugArgs
or flags. Pass --no-defaults to skip. --minimal still skips optional module/env/exp.`,
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		pf, err := requireProfile()
		if err != nil {
			handleErr(err)
			return
		}
		if pf.SpaceID == "" || pf.BugTypeID == "" {
			handleErr(fmt.Errorf("profile %q missing space_id or bug_type_id", pf.Name))
			return
		}

		title, _ := cmd.Flags().GetString("title")
		description, _ := cmd.Flags().GetString("description")
		environment, _ := cmd.Flags().GetString("environment")
		module, _ := cmd.Flags().GetString("module")
		priority, _ := cmd.Flags().GetString("priority")
		seriousLevel, _ := cmd.Flags().GetString("serious-level")
		expectedCompletion, _ := cmd.Flags().GetString("expected-completion")
		sprint, _ := cmd.Flags().GetString("sprint")
		assignedTo, _ := cmd.Flags().GetString("assigned-to")
		minimal, _ := cmd.Flags().GetBool("minimal")

		if err := requireFlags("title", title, "description", description); err != nil {
			handleErr(err)
			return
		}

		wantModule := !minimal && pf.ModuleFieldID() != ""
		wantEnv := !minimal && pf.EnvironmentFieldID() != ""
		wantExp := !minimal && pf.ExpCompletionTimeKey() != ""

		if wantExp {
			if strings.TrimSpace(expectedCompletion) == "" {
				handleErr(fmt.Errorf("missing required flag --expected-completion (profile configures ExpCompletionTime; use --minimal to skip)"))
				return
			}
			if !expectedCompletionDateRe.MatchString(strings.TrimSpace(expectedCompletion)) {
				handleErr(fmt.Errorf("--expected-completion must be YYYY-MM-DD"))
				return
			}
		} else if strings.TrimSpace(expectedCompletion) != "" && !minimal {
			// User passed a date but profile does not configure the field — ignore quietly
			// unless they expected it to be sent; still validate format if provided.
			if !expectedCompletionDateRe.MatchString(strings.TrimSpace(expectedCompletion)) {
				handleErr(fmt.Errorf("--expected-completion must be YYYY-MM-DD"))
				return
			}
		}

		if wantEnv {
			if !profile.AllowedContains(pf.AllowedEnvironments, environment) {
				handleErr(fmt.Errorf("--environment 仅支持 %s", strings.Join(pf.AllowedEnvironments, "/")))
				return
			}
		}
		if wantModule {
			if !profile.AllowedContains(pf.AllowedModules, module) {
				handleErr(fmt.Errorf("--module 仅支持 %s", strings.Join(pf.AllowedModules, "/")))
				return
			}
		}

		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}

		if strings.TrimSpace(sprint) == "" {
			path, err := c.ProjexPath(cmd.Context(), "/workitems:search")
			if err != nil {
				handleErr(err)
				return
			}
			searchBody := map[string]any{
				"spaceId":  pf.SpaceID,
				"category": "Bug",
				"page":     1,
				"perPage":  10,
			}
			var searchOut any
			if err := c.Post(cmd.Context(), path, searchBody, &searchOut); err != nil {
				handleErr(err)
				return
			}
			suggestion := zhiyi.AggregateBugSprints(zhiyi.ExtractSearchItems(searchOut))
			handleErr(fmt.Errorf("%s", zhiyi.FormatSprintSuggestion(suggestion)))
			return
		}

		if strings.TrimSpace(assignedTo) == "" {
			assignedTo = pf.DefaultAssignedTo
		}
		if strings.TrimSpace(assignedTo) == "" {
			assignedTo = "self"
		}
		assignedTo, err = resolveSelfID(cmd.Context(), c, assignedTo)
		if err != nil {
			handleErr(err)
			return
		}
		if assignedTo == "" {
			handleErr(fmt.Errorf("could not resolve --assigned-to / default_assigned_to / self"))
			return
		}

		verifierFlag, _ := cmd.Flags().GetString("verifier")
		verifierExplicit := strings.TrimSpace(verifierFlag)
		if verifierExplicit == "" {
			verifierExplicit = strings.TrimSpace(pf.DefaultVerifier)
		}
		var verifier string
		if verifierExplicit != "" {
			verifier, err = resolveSelfID(cmd.Context(), c, verifierExplicit)
			if err != nil {
				handleErr(err)
				return
			}
		}

		body, err := zhiyi.BuildCreateBugArgs(zhiyi.CreateBugInput{
			Title:              title,
			Description:        description,
			Environment:        environment,
			Priority:           priority,
			SeriousLevel:       seriousLevel,
			Module:             module,
			ExpectedCompletion: expectedCompletion,
			Sprint:             sprint,
			AssignedTo:         assignedTo,
			Minimal:            minimal,
		}, pf)
		if err != nil {
			handleErr(err)
			return
		}
		noDefaults, _ := cmd.Flags().GetBool("no-defaults")
		if !noDefaults {
			profile.ApplyWorkitemDefaults(body, pf.BugTypeID, pf)
		}
		if verifier != "" {
			body["verifier"] = verifier
		} else if v, _ := body["verifier"].(string); strings.TrimSpace(v) != "" {
			verifier = strings.TrimSpace(v)
		} else {
			fmt.Fprintf(os.Stderr, "warning: +bug-create verifier unset (pass --verifier <userId|self> or set profile.default_verifier / workitem_defaults verifier)\n")
		}

		path, err := c.ProjexPath(cmd.Context(), "/workitems")
		if err != nil {
			handleErr(err)
			return
		}

		if globalDryRun {
			handleErr(output.DryRunResult(string(risk.Write), c.Preview("POST", path, nil, body)))
			return
		}
		if err := risk.CheckConfirmed("workitem +bug-create", risk.Write, globalYes); err != nil {
			handleErr(err)
			return
		}

		var created map[string]any
		if err := c.Post(cmd.Context(), path, body, &created); err != nil {
			handleErr(withWriteDedupeHint(err, workitemSearchHint(title)))
			return
		}
		internal := zhiyi.InternalID(created)
		result := map[string]any{
			"created": created,
		}
		if verifier != "" {
			result["verifier"] = verifier
		}
		if internal != "" {
			result["internal_id"] = internal
			getPath, err := c.ProjexPath(cmd.Context(), "/workitems/"+internal)
			if err == nil {
				var full map[string]any
				if err := c.Get(cmd.Context(), getPath, nil, &full); err == nil {
					result["serial_number"] = zhiyi.SerialNumber(full)
					if u := zhiyi.WorkItemURL(full, pf.SpaceID); u != "" {
						result["url"] = u
					}
					result["item"] = full
					if verifier == "" {
						if vv := zhiyi.VerifierID(full); vv != "" {
							result["verifier"] = vv
							verifier = vv
						}
					}
				}
			}
		}
		handleErr(output.Success(result, map[string]any{"risk": risk.Write, "profile": pf.Name, "minimal": minimal, "verifier": verifier}))
	},
}

func init() {
	workitemBugCreateCmd.Flags().String("title", "", "bug title (required)")
	workitemBugCreateCmd.Flags().String("description", "", "Markdown description (required)")
	workitemBugCreateCmd.Flags().String("environment", "测试环境", "生产环境 / 测试环境 (only if profile configures environment field)")
	workitemBugCreateCmd.Flags().String("module", "MES", "MES / OMS / PDM / 系统服务 (only if profile configures module field)")
	workitemBugCreateCmd.Flags().String("priority", "high", "urgent/high/medium/low (needs bug_create_fields.priority map) or option id")
	workitemBugCreateCmd.Flags().String("serious-level", "normal", "fatal/severe/normal/slight (synonyms: serious→severe, minor→slight; needs bug_create_fields.serious_level map) or option id")
	workitemBugCreateCmd.Flags().String("expected-completion", "", "YYYY-MM-DD (required only if profile configures ExpCompletionTime)")
	workitemBugCreateCmd.Flags().String("sprint", "", "sprint id (required; omit to get suggestion)")
	workitemBugCreateCmd.Flags().String("assigned-to", "", "assignee user id (default: profile.default_assigned_to or self)")
	workitemBugCreateCmd.Flags().String("verifier", "", "verifier user id or self (default: profile.default_verifier or workitem_defaults verifier)")
	workitemBugCreateCmd.Flags().Bool("minimal", false, "only subject/description/priority/seriousLevel/sprint/assignedTo (skip module/env/ExpCompletionTime)")
	workitemBugCreateCmd.Flags().Bool("no-defaults", false, "skip profile workitem_defaults for bug_type_id")
	workitemCmd.AddCommand(workitemBugCreateCmd)
}
