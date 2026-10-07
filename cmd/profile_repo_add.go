package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yunxiao-cli/yunxiao/internal/client"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/profile"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
	"github.com/yunxiao-cli/yunxiao/internal/zhiyi"
)

// profileRepoAddCmd registers a Codeup repository alias under profile.repositories (#125).
// <repo> accepts every --repo form: numeric id, org[/group]/repo path, registered alias,
// or bare repo name (auto-discovered when unique in the organization). The stored value
// is always the resolved numeric id.
var profileRepoAddCmd = &cobra.Command{
	Use:   "repo-add <alias> <repo>",
	Short: "Register a Codeup repository alias in profile.repositories",
	Long: `Risk: write
Writes repositories[<alias>] = <numeric repo id> in ~/.config/yunxiao/profiles/<name>.json.

<repo> accepts the same forms as codeup --repo (#125):
  - numeric repositoryId (stored as-is, no network call)
  - org/repo or org/group/repo path — resolved via one read-only GET .../repositories/{repo}
  - registered alias (copies its numeric id)
  - bare repo name — resolved by one read-only repos search when unique in the organization

The resolving GETs also run under --dry-run (preview only, profile file untouched).
An existing alias is only overwritten with --force.

Examples:
  yunxiao profile repo-add zhiyi_doc sanzhi/zhiyi/zhiyi_doc --profile zhiyi
  yunxiao profile repo-add doc zhiyi_doc
  yunxiao profile repo-add doc 4951320 --force`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		alias := strings.TrimSpace(args[0])
		repoRef := strings.TrimSpace(args[1])
		force, _ := cmd.Flags().GetBool("force")
		if alias == "" || strings.ContainsAny(alias, " \t,") {
			handleErr(fmt.Errorf("invalid alias %q: expected a non-empty name without spaces or commas", args[0]))
			return
		}
		pf, err := requireProfile()
		if err != nil {
			handleErr(err)
			return
		}
		id, source, label, err := resolveRepoAddTarget(cmd, pf, repoRef)
		if err != nil {
			handleErr(err)
			return
		}
		idNum, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			handleErr(fmt.Errorf("resolved repository id %q is not numeric", id))
			return
		}
		previous, existed := pf.Repositories[alias]
		if existed && previous != idNum && !force {
			handleErr(fmt.Errorf("alias %q already maps to repository %d (pass --force to replace)", alias, previous))
			return
		}
		preview := map[string]any{
			"profile":       pf.Name,
			"alias":         alias,
			"repo":          repoRef,
			"repository_id": idNum,
			"force":         force,
		}
		if existed {
			preview["previous_repository_id"] = previous
		}
		if globalDryRun {
			handleErr(output.DryRunResult(string(risk.Write), preview))
			return
		}
		if pf.Repositories == nil {
			pf.Repositories = map[string]int64{}
		}
		pf.Repositories[alias] = idNum
		written, err := pf.Save()
		if err != nil {
			handleErr(err)
			return
		}
		data := map[string]any{
			"alias":         alias,
			"repository_id": idNum,
			"profile":       pf.Name,
			"unchanged":     existed && previous == idNum,
			"hint":          fmt.Sprintf("--repo %s now resolves to repository %d (e.g. yunxiao codeup branches list --repo %s)", alias, idNum, alias),
		}
		if existed && previous != idNum {
			data["replaced_repository_id"] = previous
		}
		meta := map[string]any{
			"risk":         risk.Write,
			"profile_path": written,
			"resolved_from": source,
		}
		if label != "" {
			meta["resolved_repo"] = label
		}
		handleErr(output.Success(data, meta))
	},
}

// resolveRepoAddTarget resolves a repo-add <repo> argument to its numeric id.
// Returns the id, a resolved_from source tag (numeric|path|alias|discovery) and a
// display label (pathWithNamespace when known). Only the path form needs a GET here;
// bare names / aliases go through resolveCodeupRepoContext (discovery, #125).
func resolveRepoAddTarget(cmd *cobra.Command, pf *profile.Profile, repoRef string) (id, source, label string, err error) {
	switch {
	case zhiyi.IsNumericRepositoryID(repoRef):
		return strings.TrimSpace(repoRef), "numeric", "", nil
	case strings.Contains(repoRef, "/") || strings.Contains(strings.ToLower(repoRef), "%2f"):
		c, _, cerr := mustClient()
		if cerr != nil {
			return "", "", "", cerr
		}
		path, perr := c.CodeupPath(cmd.Context(), "/repositories/"+client.EncodeRepoID(repoRef))
		if perr != nil {
			return "", "", "", perr
		}
		var obj map[string]any
		if gerr := c.Get(cmd.Context(), path, nil, &obj); gerr != nil {
			return "", "", "", fmt.Errorf("resolve repository %q: %w", repoRef, gerr)
		}
		got := repositoryIDFromRepoObject(obj)
		if got == "" {
			return "", "", "", fmt.Errorf("repository %q response carries no numeric id (fields: use `yunxiao codeup repos get --repo %s` to inspect)", repoRef, repoRef)
		}
		if l, ok := obj["pathWithNamespace"].(string); ok && l != "" {
			label = l
		}
		return got, "path", label, nil
	default:
		if pf != nil && pf.Repositories != nil {
			if existing, ok := pf.Repositories[repoRef]; ok {
				return fmt.Sprintf("%d", existing), "alias", "", nil
			}
		}
		resolved, rerr := resolveCodeupRepoContext(cmd.Context(), repoRef)
		if rerr != nil {
			return "", "", "", rerr
		}
		return resolved, "discovery", "", nil
	}
}

func init() {
	profileRepoAddCmd.Flags().Bool("force", false, "replace an existing alias mapping")
	profileCmd.AddCommand(profileRepoAddCmd)
}
