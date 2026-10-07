package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yunxiao-cli/yunxiao/internal/client"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
)

var codeupProtectedBranchesCmd = &cobra.Command{
	Use:     "protected-branches",
	Aliases: []string{"protect"},
	Short:   "Protected branches (Codeup)",
	Long: `Codeup protected branches.

  yunxiao codeup protected-branches list --repo <id|alias>
  yunxiao codeup protected-branches get --repo <id|alias> --id <ruleId>
  yunxiao codeup protected-branches create --repo <id|alias> --branch master \
    [--allow-push-roles 40,30] [--allow-merge-roles 40,30] [--body '{…}'] --dry-run
  yunxiao codeup protected-branches delete --repo <id|alias> --id <ruleId> --yes

--repo accepts numeric id or profile.repositories alias.
Risk: list/get=read; create/delete=high-risk-write.`,
}

var codeupProtectedListCmd = &cobra.Command{
	Use:   "list",
	Short: "List protected branches",
	Long:  "Risk: read\nHTTP: GET .../repositories/{repo}/protectedBranches\nSource: ListProtectedBranches",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		repo, _ := cmd.Flags().GetString("repo")
		if err := requireFlags("repo", repo); err != nil {
			handleErr(err)
			return
		}
		repositoryID, err := resolveCodeupRepo(repo)
		if err != nil {
			handleErr(err)
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		repoID := client.EncodeRepoID(repositoryID)
		path, err := c.CodeupPath(cmd.Context(), "/repositories/"+repoID+"/protectedBranches")
		if err != nil {
			handleErr(err)
			return
		}
		handleErr(runRead(cmd.Context(), c, "GET", path, nil, nil, map[string]any{"risk": risk.Read}, func(out any, meta map[string]any) (any, map[string]any) {
			meta["repository_id"] = repositoryID
			return out, meta
		}))
	},
}

var codeupProtectedGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get a protected branch rule",
	Long:  "Risk: read\nHTTP: GET .../protectedBranches/{id}\nSource: GetProtectedBranch",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		repo, _ := cmd.Flags().GetString("repo")
		id, _ := cmd.Flags().GetString("id")
		if err := requireFlags("repo", repo, "id", id); err != nil {
			handleErr(err)
			return
		}
		repositoryID, err := resolveCodeupRepo(repo)
		if err != nil {
			handleErr(err)
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		repoID := client.EncodeRepoID(repositoryID)
		path, err := c.CodeupPath(cmd.Context(), "/repositories/"+repoID+"/protectedBranches/"+id)
		if err != nil {
			handleErr(err)
			return
		}
		handleErr(runRead(cmd.Context(), c, "GET", path, nil, nil, map[string]any{"risk": risk.Read}, func(out any, meta map[string]any) (any, map[string]any) {
			meta["repository_id"] = repositoryID
			return out, meta
		}))
	},
}

var codeupProtectedCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a protected branch rule (high-risk-write)",
	Long: `Risk: high-risk-write
HTTP: POST .../repositories/{repo}/protectedBranches
Source: CreateProtectedBranch

Minimal body: {"branch":"master"} plus optional allowPushRoles / allowMergeRoles.
Advanced settings: pass full JSON via --body (merged over flags; --body.branch wins if set).`,
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		repo, _ := cmd.Flags().GetString("repo")
		branch, _ := cmd.Flags().GetString("branch")
		pushRoles, _ := cmd.Flags().GetString("allow-push-roles")
		mergeRoles, _ := cmd.Flags().GetString("allow-merge-roles")
		bodyJSON, _ := cmd.Flags().GetString("body")
		if err := requireFlags("repo", repo); err != nil {
			handleErr(err)
			return
		}
		extra, err := parseJSONMap(bodyJSON)
		if err != nil {
			handleErr(err)
			return
		}
		if strings.TrimSpace(branch) == "" && (extra == nil || extra["branch"] == nil || fmt.Sprint(extra["branch"]) == "") {
			handleErr(fmt.Errorf("missing required flag --branch (or body.branch)"))
			return
		}
		repositoryID, err := resolveCodeupRepo(repo)
		if err != nil {
			handleErr(err)
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		body := map[string]any{}
		if strings.TrimSpace(branch) != "" {
			body["branch"] = branch
		}
		if roles, err := parseIntCSV(pushRoles); err != nil {
			handleErr(fmt.Errorf("--allow-push-roles: %w", err))
			return
		} else if len(roles) > 0 {
			body["allowPushRoles"] = roles
		}
		if roles, err := parseIntCSV(mergeRoles); err != nil {
			handleErr(fmt.Errorf("--allow-merge-roles: %w", err))
			return
		} else if len(roles) > 0 {
			body["allowMergeRoles"] = roles
		}
		for k, v := range extra {
			body[k] = v
		}
		repoID := client.EncodeRepoID(repositoryID)
		path, err := c.CodeupPath(cmd.Context(), "/repositories/"+repoID+"/protectedBranches")
		if err != nil {
			handleErr(err)
			return
		}
		handleErr(runJSONMutating(cmd.Context(), c, "codeup protected-branches create", risk.HighRiskWrite, "POST", path, nil, body, func(out any, meta map[string]any) (any, map[string]any) {
			meta["repository_id"] = repositoryID
			return out, meta
		}))
	},
}

var codeupProtectedDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a protected branch rule (high-risk-write)",
	Long:  "Risk: high-risk-write\nHTTP: DELETE .../protectedBranches/{id}\nSource: DeleteProtectedBranch",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		repo, _ := cmd.Flags().GetString("repo")
		id, _ := cmd.Flags().GetString("id")
		if err := requireFlags("repo", repo, "id", id); err != nil {
			handleErr(err)
			return
		}
		repositoryID, err := resolveCodeupRepo(repo)
		if err != nil {
			handleErr(err)
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		repoID := client.EncodeRepoID(repositoryID)
		path, err := c.CodeupPath(cmd.Context(), "/repositories/"+repoID+"/protectedBranches/"+id)
		if err != nil {
			handleErr(err)
			return
		}
		handleErr(runJSONMutating(cmd.Context(), c, "codeup protected-branches delete", risk.HighRiskWrite, "DELETE", path, nil, nil, func(out any, meta map[string]any) (any, map[string]any) {
			meta["repository_id"] = repositoryID
			return out, meta
		}))
	},
}

func parseIntCSV(s string) ([]int, error) {
	parts := splitCSV(s)
	if len(parts) == 0 {
		return nil, nil
	}
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			return nil, fmt.Errorf("invalid int %q", p)
		}
		out = append(out, n)
	}
	return out, nil
}

func init() {
	codeupProtectedListCmd.Flags().String("repo", "", "repository id, profile alias, org/repo path, or bare name (required)")
	codeupProtectedGetCmd.Flags().String("repo", "", "repository id, profile alias, org/repo path, or bare name (required)")
	codeupProtectedGetCmd.Flags().String("id", "", "protect rule id (required)")
	codeupProtectedCreateCmd.Flags().String("repo", "", "repository id, profile alias, org/repo path, or bare name (required)")
	codeupProtectedCreateCmd.Flags().String("branch", "", "branch name to protect (required unless body.branch)")
	codeupProtectedCreateCmd.Flags().String("allow-push-roles", "", "comma-separated role ids, e.g. 40,30")
	codeupProtectedCreateCmd.Flags().String("allow-merge-roles", "", "comma-separated role ids, e.g. 40,30")
	codeupProtectedCreateCmd.Flags().String("body", "", "optional full JSON body (merged over flags)")
	codeupProtectedDeleteCmd.Flags().String("repo", "", "repository id, profile alias, org/repo path, or bare name (required)")
	codeupProtectedDeleteCmd.Flags().String("id", "", "protect rule id (required)")
	codeupProtectedBranchesCmd.AddCommand(codeupProtectedListCmd, codeupProtectedGetCmd, codeupProtectedCreateCmd, codeupProtectedDeleteCmd)
	codeupCmd.AddCommand(codeupProtectedBranchesCmd)
}
