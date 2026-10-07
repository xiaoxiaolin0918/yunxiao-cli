package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yunxiao-cli/yunxiao/internal/client"
	"github.com/yunxiao-cli/yunxiao/internal/orguid"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/profile"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
	"github.com/yunxiao-cli/yunxiao/internal/zhiyi"
)

// categoryCreateLong renders the shared +risk-create / +req-create help (#128).
func categoryCreateLong(category, typeKey string) string {
	lower := strings.ToLower(category)
	return fmt.Sprintf(`Risk: write (requires --yes for real run; prefer --dry-run first)

Profile-driven shortcut for creating a %s workitem, aligned with +bug-create:
type id, priority option id and participants come from the profile instead of
hardcoded tenant values.

Type resolution: --type-id flag > profile %s > the single workitem_defaults /
workflows entry with category %q. Several candidates → error listing them (never
guess); none → error with the profile hint. %s types must be enabled per project
in the Projex settings UI first (no OpenAPI; create fails with 工作项类型未启用！).

Priority (--priority, default medium) resolves: workitem_defaults[<type>].fields.
priority.display 显示值 (e.g. 高/中) → shared bug_create_fields.priority alias map
(priority option ids are space-scoped) → medium falls back to the per-type default
value → raw option id passthrough. Unmapped known aliases (urgent/high/medium/low)
fail client-side (no API 400). Pass --priority="" to omit it entirely.

Sprint is NOT sent unless --sprint is passed (%s types often reject 迭代:
未启用此字段【迭代】). formatType defaults to MARKDOWN.

workitem_defaults[<type>] fills participants/trackers/其他 defaults unless
--no-defaults. Required-field precheck (#95): one read-only GET of the type's
fields before POST (also under --dry-run); all missing required fields are
reported at once (error.subtype missing_required_fields); --no-precheck skips.

--assignee accepts an organization member display name (resolved via
members:search, exact match; ambiguity lists candidates). --assigned-to takes a
raw user id or self (default profile.default_assigned_to or self). Use only one
of the two.

Windows / PowerShell: for Chinese title or description, prefer --title-file /
--description-file (UTF-8, BOM stripped) over inline flags.

  yunxiao workitem +%s-create --title "…" --description "…" --dry-run
  yunxiao workitem +%s-create --title-file ./t.txt --description-file ./d.md --priority 高 --yes
  yunxiao workitem +%s-create --assignee 崔健 --title "…" --description-file ./d.md --yes`, category, typeKey, category, category, category, lower, lower, lower)
}

var workitemRiskCreateCmd = &cobra.Command{
	Use:   "+risk-create",
	Short: "Shortcut: create a risk with profile field maps",
	Long:  categoryCreateLong("Risk", "risk_type_id"),
	Run: func(cmd *cobra.Command, args []string) {
		runWorkitemCategoryCreate(cmd, "Risk")
	},
}

var workitemReqCreateCmd = &cobra.Command{
	Use:   "+req-create",
	Short: "Shortcut: create a requirement with profile field maps",
	Long:  categoryCreateLong("Req", "req_type_id"),
	Run: func(cmd *cobra.Command, args []string) {
		runWorkitemCategoryCreate(cmd, "Req")
	},
}

// runWorkitemCategoryCreate is the shared +risk-create / +req-create runner (#128):
// resolve type id from the profile, build the create body via profile field maps,
// apply workitem_defaults, run the #95 required-field precheck, then reuse the
// workitem create POST path (brief view + meta.url enrich).
func runWorkitemCategoryCreate(cmd *cobra.Command, category string) {
	flagOrg(globalOrg)
	pf, err := requireProfile()
	if err != nil {
		handleErr(err)
		return
	}
	if strings.TrimSpace(pf.SpaceID) == "" {
		handleErr(fmt.Errorf("profile %q missing space_id", pf.Name))
		return
	}

	typeIDFlag, _ := cmd.Flags().GetString("type-id")
	typeID := strings.TrimSpace(typeIDFlag)
	typeSource := "flag"
	if typeID == "" {
		typeID, err = pf.ResolveCategoryTypeID(category)
		if err != nil {
			handleErr(err)
			return
		}
		for _, cand := range pf.CategoryTypeCandidates(category) {
			if cand.TypeID == typeID {
				typeSource = cand.Source
				break
			}
		}
	}

	titleFlag, _ := cmd.Flags().GetString("title")
	titleFile, _ := cmd.Flags().GetString("title-file")
	title, err := readFlagOrFile(titleFlag, titleFile, "title", true)
	if err != nil {
		handleErr(err)
		return
	}
	descriptionFlag, _ := cmd.Flags().GetString("description")
	descriptionFile, _ := cmd.Flags().GetString("description-file")
	description, err := readFlagOrFile(descriptionFlag, descriptionFile, "description", true)
	if err != nil {
		handleErr(err)
		return
	}

	priority, _ := cmd.Flags().GetString("priority")
	sprint, _ := cmd.Flags().GetString("sprint")
	assignedTo, _ := cmd.Flags().GetString("assigned-to")
	assignee, _ := cmd.Flags().GetString("assignee")
	if strings.TrimSpace(assignee) != "" && strings.TrimSpace(assignedTo) != "" {
		handleErr(fmt.Errorf("use only one of --assignee (display name) or --assigned-to (user id)"))
		return
	}

	c, _, err := mustClient()
	if err != nil {
		handleErr(err)
		return
	}

	if strings.TrimSpace(assignee) != "" {
		assignedTo, err = resolveAssigneeName(cmd.Context(), c, assignee)
		if err != nil {
			handleErr(err)
			return
		}
	} else {
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
	}
	if strings.TrimSpace(assignedTo) == "" {
		handleErr(fmt.Errorf("could not resolve --assigned-to / default_assigned_to / self"))
		return
	}

	var participants []string
	participantsFlag, _ := cmd.Flags().GetString("participants")
	for _, p := range splitCSV(participantsFlag) {
		if p == "self" {
			p, err = resolveSelfID(cmd.Context(), c, p)
			if err != nil {
				handleErr(err)
				return
			}
		}
		if p != "" {
			participants = append(participants, p)
		}
	}

	body, err := zhiyi.BuildCreateCategoryItemArgs(zhiyi.CreateCategoryItemInput{
		Category:     category,
		TypeID:       typeID,
		Title:        title,
		Description:  description,
		Priority:     priority,
		Sprint:       sprint,
		AssignedTo:   assignedTo,
		Participants: participants,
	}, pf)
	if err != nil {
		handleErr(err)
		return
	}
	if noDefaults, _ := cmd.Flags().GetBool("no-defaults"); !noDefaults {
		profile.ApplyWorkitemDefaults(body, typeID, pf)
	}

	path, err := c.ProjexPath(cmd.Context(), "/workitems")
	if err != nil {
		handleErr(err)
		return
	}

	action := "workitem +" + strings.ToLower(category) + "-create"
	full, _ := cmd.Flags().GetBool("full")

	// #95 precheck: same one-shot required-field GET as `workitem create` (also
	// under --dry-run); profile create_required is the warn-only fallback.
	var precheck map[string]any
	if noPrecheck, _ := cmd.Flags().GetBool("no-precheck"); !noPrecheck {
		var profileRequired []string
		if defs, ok := pf.WorkitemDefaults[typeID]; ok {
			profileRequired = defs.CreateRequired
		}
		precheck, err = precheckWorkitemCreate(cmd.Context(), c, pf.SpaceID, typeID, body, profileRequired)
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

	if globalDryRun {
		handleErr(output.DryRunResult(string(risk.Write), preview))
		return
	}
	if err := risk.CheckConfirmed(action, risk.Write, globalYes); err != nil {
		handleErr(err)
		return
	}

	err = runJSONMutatingPreview(cmd.Context(), c, action, risk.Write, "POST", path, nil, body, preview, func(out any, meta map[string]any) (any, map[string]any) {
		if precheck != nil {
			meta["precheck"] = precheck
		}
		meta["profile"] = pf.Name
		meta["category"] = category
		meta["type_id"] = typeID
		meta["type_source"] = typeSource
		item := asStringMap(out)
		item = zhiyi.EnsureWorkItemCreateFields(item, func(id string) (map[string]any, error) {
			return fetchWorkItemMap(cmd.Context(), c, id)
		})
		zhiyi.EnrichWorkItemMeta(meta, item, profileSpaceID(), pf.SpaceID)
		if full {
			return item, meta
		}
		return zhiyi.BriefWorkItem(item), meta
	})
	handleCreateErr(err, precheck)
}

// resolveAssigneeName resolves an organization member display name to a userId via
// POST /members:search (same endpoint as `organization members search`). Requires an
// exact (trimmed, case-insensitive) name match; several distinct user ids → error
// listing candidates; no match → error. Never guesses (#128).
func resolveAssigneeName(ctx context.Context, c *client.Client, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("--assignee is empty")
	}
	path, err := c.PlatformPath(ctx, "/members:search")
	if err != nil {
		return "", err
	}
	var out any
	if err := c.Post(ctx, path, map[string]any{"query": name, "page": 1, "perPage": 50}, &out); err != nil {
		return "", fmt.Errorf("resolve --assignee %q via members:search: %w", name, err)
	}
	type memberMatch struct{ id, name string }
	var matches []memberMatch
	seen := map[string]bool{}
	for _, it := range orguid.MembersFromAPI(out) {
		m, ok := it.(map[string]any)
		if !ok {
			continue
		}
		n, _ := m["name"].(string)
		if !strings.EqualFold(strings.TrimSpace(n), name) {
			continue
		}
		id := memberIDString(m)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		matches = append(matches, memberMatch{id: id, name: strings.TrimSpace(n)})
	}
	switch len(matches) {
	case 1:
		return matches[0].id, nil
	case 0:
		return "", fmt.Errorf("no organization member named %q (via members:search); pass --assigned-to <userId> instead", name)
	default:
		parts := make([]string, 0, len(matches))
		for _, m := range matches {
			parts = append(parts, fmt.Sprintf("%s (%s)", m.id, m.name))
		}
		return "", fmt.Errorf("--assignee %q is ambiguous: %s — pass --assigned-to with the exact user id", name, strings.Join(parts, ", "))
	}
}

// memberIDString extracts the member userId from an OAPI member map (string or
// numeric id; hex userId per /oapi/v1/platform members).
func memberIDString(m map[string]any) string {
	switch v := m["id"].(type) {
	case string:
		return strings.TrimSpace(v)
	case float64:
		return fmt.Sprintf("%.0f", v)
	default:
		return ""
	}
}

func init() {
	for _, c := range []*cobra.Command{workitemRiskCreateCmd, workitemReqCreateCmd} {
		c.Flags().String("title", "", "title (required unless --title-file)")
		c.Flags().String("title-file", "", "UTF-8 file for title (BOM stripped; preferred on Windows for Chinese)")
		c.Flags().String("description", "", "Markdown description (required unless --description-file)")
		c.Flags().String("description-file", "", "UTF-8 Markdown file (BOM stripped; preferred on Windows for Chinese)")
		c.Flags().String("priority", "medium", "urgent/high/medium/low alias, 显示值 from workitem_defaults (e.g. 高), or option id; pass \"\" to omit")
		c.Flags().String("assignee", "", "assignee display name, resolved via organization members:search (exact match; ambiguity lists candidates)")
		c.Flags().String("assigned-to", "", "assignee user id or self (default: profile.default_assigned_to or self); use only one of --assignee/--assigned-to")
		c.Flags().String("participants", "", "comma-separated participant user ids (self allowed)")
		c.Flags().String("sprint", "", "sprint id (optional; NOT sent by default — Risk/Req types often reject 迭代)")
		c.Flags().String("type-id", "", "override the profile-resolved workitem type id")
		c.Flags().Bool("no-defaults", false, "skip profile workitem_defaults for the resolved type")
		c.Flags().Bool("no-precheck", false, "skip the required-field precheck (one read GET of the type fields) before POST")
		c.Flags().Bool("full", false, "print the raw created object instead of the brief view")
		workitemCmd.AddCommand(c)
	}
}
