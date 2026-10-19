package cmd

import (
	"github.com/spf13/cobra"
	"github.com/yunxiao-cli/yunxiao/internal/client"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
)

var packagesCmd = &cobra.Command{
	Use:   "packages",
	Short: "Artifact package repositories",
	Long: `Packages domain (Packages / artifact repos).

Typed:
  yunxiao packages repos list [--repo-types ...] [--repo-categories ...]
  yunxiao packages artifacts list|get|delete|upload

Upload (Packages protocol API + Basic auth, not openapi-rdc JSON):
  GENERIC: --repo-id --repo-type GENERIC --path --version --file --username --password
  MAVEN:   --repo-id --repo-type MAVEN --group-id --artifact-id --version --file --username --password
Dual credentials: OpenAPI PAT/profile (mustClient + ResolveOrgID) AND protocol Basic
(YUNXIAO_PACKAGES_* or flags). Dry-run still requires Basic creds and an existing --file.
OpenAPI binary upload announced 2025-02 but docs unpublished; CLI uses repo-guide protocol.
Not wrapped (native clients): NPM=npm publish, DOCKER=docker push, NUGET=nuget/dotnet, PYPI=twine.

Risk: list/get=read; upload=write; delete=high-risk-write`,
}

var packagesReposCmd = &cobra.Command{Use: "repos", Short: "Package repositories"}

var packagesReposListCmd = &cobra.Command{
	Use:   "list",
	Short: "List package repositories",
	Long:  "Risk: read\nHTTP: GET .../packages/.../repositories",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		repoTypes, _ := cmd.Flags().GetString("repo-types")
		repoCategories, _ := cmd.Flags().GetString("repo-categories")
		page, _ := cmd.Flags().GetInt("page")
		perPage, _ := cmd.Flags().GetInt("per-page")
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.PackagesPath(cmd.Context(), "/repositories")
		if err != nil {
			handleErr(err)
			return
		}
		q := client.PageQuery(page, perPage)
		if repoTypes != "" {
			q["repoTypes"] = repoTypes
		}
		if repoCategories != "" {
			q["repoCategories"] = repoCategories
		}
		handleErr(runRead(cmd.Context(), c, "GET", path, q, nil, map[string]any{"risk": risk.Read}, nil))
	},
}

var packagesArtifactsCmd = &cobra.Command{Use: "artifacts", Short: "Artifacts in a package repository"}

var packagesArtifactsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List artifacts in a package repository",
	Long:  "Risk: read\nHTTP: GET .../packages/.../repositories/{repoId}/artifacts",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		repoID, _ := cmd.Flags().GetString("repo-id")
		repoType, _ := cmd.Flags().GetString("repo-type")
		search, _ := cmd.Flags().GetString("search")
		page, _ := cmd.Flags().GetInt("page")
		perPage, _ := cmd.Flags().GetInt("per-page")
		orderBy, _ := cmd.Flags().GetString("order-by")
		sort, _ := cmd.Flags().GetString("sort")
		if err := requireFlags("repo-id", repoID, "repo-type", repoType); err != nil {
			handleErr(err)
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.PackagesPath(cmd.Context(), "/repositories/"+repoID+"/artifacts")
		if err != nil {
			handleErr(err)
			return
		}
		q := client.PageQuery(page, perPage)
		q["repoType"] = repoType
		q["orderBy"] = orderBy
		q["sort"] = sort
		if search != "" {
			q["search"] = search
		}
		handleErr(runRead(cmd.Context(), c, "GET", path, q, nil, map[string]any{"risk": risk.Read}, nil))
	},
}

var packagesArtifactsGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get a single artifact",
	Long:  "Risk: read\nHTTP: GET .../repositories/{repoId}/artifacts/{id}",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		repoID, _ := cmd.Flags().GetString("repo-id")
		repoType, _ := cmd.Flags().GetString("repo-type")
		id, _ := cmd.Flags().GetString("id")
		if err := requireFlags("repo-id", repoID, "repo-type", repoType, "id", id); err != nil {
			handleErr(err)
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.PackagesPath(cmd.Context(), "/repositories/"+repoID+"/artifacts/"+id)
		if err != nil {
			handleErr(err)
			return
		}
		q := map[string]string{"repoType": repoType}
		handleErr(runRead(cmd.Context(), c, "GET", path, q, nil, map[string]any{"risk": risk.Read}, nil))
	},
}

var packagesArtifactsDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete an artifact (or a version) — high-risk-write",
	Long: `Risk: high-risk-write

HTTP: DELETE .../artifacts/{id}[/{version-id}]?repoType=...
Omit --version-id to delete the whole artifact module.`,
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		repoID, _ := cmd.Flags().GetString("repo-id")
		repoType, _ := cmd.Flags().GetString("repo-type")
		id, _ := cmd.Flags().GetString("id")
		versionID, _ := cmd.Flags().GetString("version-id")
		if err := requireFlags("repo-id", repoID, "repo-type", repoType, "id", id); err != nil {
			handleErr(err)
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		suffix := "/repositories/" + repoID + "/artifacts/" + id
		if versionID != "" {
			suffix += "/" + versionID
		}
		path, err := c.PackagesPath(cmd.Context(), suffix)
		if err != nil {
			handleErr(err)
			return
		}
		q := map[string]string{"repoType": repoType}
		handleErr(runJSONMutating(cmd.Context(), c, "packages artifacts delete", risk.HighRiskWrite, "DELETE", path, q, nil, nil))
	},
}

func init() {
	packagesReposListCmd.Flags().String("repo-types", "", "repo types filter")
	packagesReposListCmd.Flags().String("repo-categories", "", "repo categories filter")
	packagesReposListCmd.Flags().Int("page", 1, "page")
	packagesReposListCmd.Flags().Int("per-page", 20, "per page")
	packagesArtifactsListCmd.Flags().String("repo-id", "", "package repository id (required)")
	packagesArtifactsListCmd.Flags().String("repo-type", "", "repo type e.g. GENERIC/MAVEN/NPM (required)")
	packagesArtifactsListCmd.Flags().String("search", "", "search keyword")
	packagesArtifactsListCmd.Flags().String("order-by", "latestUpdate", "orderBy")
	packagesArtifactsListCmd.Flags().String("sort", "desc", "asc|desc")
	packagesArtifactsListCmd.Flags().Int("page", 1, "page")
	packagesArtifactsListCmd.Flags().Int("per-page", 20, "per page")
	packagesArtifactsGetCmd.Flags().String("repo-id", "", "package repository id (required)")
	packagesArtifactsGetCmd.Flags().String("repo-type", "", "repo type (required)")
	packagesArtifactsGetCmd.Flags().String("id", "", "artifact id (required)")
	packagesArtifactsDeleteCmd.Flags().String("repo-id", "", "package repository id (required)")
	packagesArtifactsDeleteCmd.Flags().String("repo-type", "", "repo type (required)")
	packagesArtifactsDeleteCmd.Flags().String("id", "", "artifact id (required)")
	packagesArtifactsDeleteCmd.Flags().String("version-id", "", "optional version id (delete one version)")
	packagesArtifactsCmd.AddCommand(packagesArtifactsListCmd, packagesArtifactsGetCmd, packagesArtifactsDeleteCmd, packagesArtifactsUploadCmd)
	packagesReposCmd.AddCommand(packagesReposListCmd)
	packagesCmd.AddCommand(packagesReposCmd, packagesArtifactsCmd)
}
