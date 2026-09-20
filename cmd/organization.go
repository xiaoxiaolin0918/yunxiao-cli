package cmd

import (
	"github.com/spf13/cobra"
	"github.com/yunxiao-cli/yunxiao/internal/client"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
)

var organizationCmd = &cobra.Command{
	Use:     "organization",
	Aliases: []string{"org"},
	Short:   "Organization and current user info",
	Long: `Organization domain.

+shortcuts:
  yunxiao organization +whoami     # current user + last org

Typed:
  yunxiao organization user get
  yunxiao organization list
  yunxiao organization members list|search
  yunxiao organization departments list|get|ancestors
  yunxiao organization roles list|get
  yunxiao organization members list|search

Risk: read`,
}

var orgUserCmd = &cobra.Command{
	Use:   "user",
	Short: "Current user resource",
}

var orgUserGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get current user and lastOrganization",
	Long:  "Risk: read\nHTTP: GET /oapi/v1/platform/user",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path := "/oapi/v1/platform/user"
		handleErr(runRead(cmd.Context(), c, "GET", path, nil, nil, map[string]any{"risk": risk.Read}, nil))
	},
}

var orgListCmd = &cobra.Command{
	Use:   "list",
	Short: "List organizations for the current user",
	Long:  "Risk: read\nHTTP: GET /oapi/v1/platform/organizations",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path := "/oapi/v1/platform/organizations"
		handleErr(runRead(cmd.Context(), c, "GET", path, nil, nil, map[string]any{"risk": risk.Read}, nil))
	},
}

var orgMembersCmd = &cobra.Command{Use: "members", Short: "Organization members"}

var orgMembersListCmd = &cobra.Command{
	Use:   "list",
	Short: "List organization members",
	Long: `Risk: read
HTTP: GET .../members

--include-aliyun-uid merges Aliyun numeric UID (accountId) from devops
ListOrganizationMembers (needs ALIBABA_CLOUD_ACCESS_KEY_ID/SECRET).
Required for Flow ManualValidate validators (validatorType: users).`,
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		page, _ := cmd.Flags().GetInt("page")
		perPage, _ := cmd.Flags().GetInt("per-page")
		includeUID, _ := cmd.Flags().GetBool("include-aliyun-uid")
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.PlatformPath(cmd.Context(), "/members")
		if err != nil {
			handleErr(err)
			return
		}
		q := client.PageQuery(page, perPage)
		var after func(out any, meta map[string]any) (any, map[string]any)
		if includeUID {
			if err := requireAliyunUIDAK(); err != nil {
				handleErr(err)
				return
			}
			after = membersAliyunUIDAfter(cmd, c, "")
		}
		handleErr(runRead(cmd.Context(), c, "GET", path, q, nil, map[string]any{"risk": risk.Read}, after))
	},
}

var orgMembersSearchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search organization members",
	Long: `Risk: read
HTTP: POST .../members:search

--include-aliyun-uid: same AK merge as members list.`,
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		query, _ := cmd.Flags().GetString("query")
		page, _ := cmd.Flags().GetInt("page")
		perPage, _ := cmd.Flags().GetInt("per-page")
		includeUID, _ := cmd.Flags().GetBool("include-aliyun-uid")
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.PlatformPath(cmd.Context(), "/members:search")
		if err != nil {
			handleErr(err)
			return
		}
		body := map[string]any{"page": page, "perPage": perPage}
		if query != "" {
			body["query"] = query
		}
		var after func(out any, meta map[string]any) (any, map[string]any)
		if includeUID {
			if err := requireAliyunUIDAK(); err != nil {
				handleErr(err)
				return
			}
			after = membersAliyunUIDAfter(cmd, c, query)
		}
		handleErr(runRead(cmd.Context(), c, "POST", path, nil, body, map[string]any{"risk": risk.Read}, after))
	},
}

var orgWhoamiShortcut = &cobra.Command{
	Use:   "+whoami",
	Short: "Shortcut: current user + last organization",
	Long:  "Risk: read",
	Run: func(cmd *cobra.Command, args []string) {
		orgUserGetCmd.Run(cmd, args)
	},
}

var orgDepartmentsCmd = &cobra.Command{Use: "departments", Short: "Organization departments"}
var orgRolesCmd = &cobra.Command{Use: "roles", Short: "Organization roles"}

var orgDepartmentsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List departments",
	Long:  "Risk: read\nHTTP: GET .../platform/.../departments\nSource: operations/organization/organization.ts getOrganizationDepartmentsFunc",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		parentID, _ := cmd.Flags().GetString("parent-id")
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.PlatformPath(cmd.Context(), "/departments")
		if err != nil {
			handleErr(err)
			return
		}
		q := map[string]string{}
		if parentID != "" {
			q["parentId"] = parentID
		}
		handleErr(runRead(cmd.Context(), c, "GET", path, q, nil, map[string]any{"risk": risk.Read}, nil))
	},
}

var orgDepartmentsGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get department info",
	Long:  "Risk: read\nHTTP: GET .../departments/{id}",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		id, _ := cmd.Flags().GetString("id")
		if err := requireFlags("id", id); err != nil {
			handleErr(err)
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.PlatformPath(cmd.Context(), "/departments/"+id)
		if err != nil {
			handleErr(err)
			return
		}
		handleErr(runRead(cmd.Context(), c, "GET", path, nil, nil, map[string]any{"risk": risk.Read}, nil))
	},
}

var orgDepartmentsAncestorsCmd = &cobra.Command{
	Use:   "ancestors",
	Short: "Get department ancestors",
	Long:  "Risk: read\nHTTP: GET .../departments/{id}/ancestors",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		id, _ := cmd.Flags().GetString("id")
		if err := requireFlags("id", id); err != nil {
			handleErr(err)
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.PlatformPath(cmd.Context(), "/departments/"+id+"/ancestors")
		if err != nil {
			handleErr(err)
			return
		}
		handleErr(runRead(cmd.Context(), c, "GET", path, nil, nil, map[string]any{"risk": risk.Read}, nil))
	},
}

var orgRolesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List organization roles",
	Long:  "Risk: read\nHTTP: GET .../platform/.../roles\nSource: listOrganizationRolesFunc",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.PlatformPath(cmd.Context(), "/roles")
		if err != nil {
			handleErr(err)
			return
		}
		handleErr(runRead(cmd.Context(), c, "GET", path, nil, nil, map[string]any{"risk": risk.Read}, nil))
	},
}

var orgRolesGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get an organization role",
	Long:  "Risk: read\nHTTP: GET .../roles/{id}",
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		id, _ := cmd.Flags().GetString("id")
		if err := requireFlags("id", id); err != nil {
			handleErr(err)
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.PlatformPath(cmd.Context(), "/roles/"+id)
		if err != nil {
			handleErr(err)
			return
		}
		handleErr(runRead(cmd.Context(), c, "GET", path, nil, nil, map[string]any{"risk": risk.Read}, nil))
	},
}

func init() {
	orgMembersListCmd.Flags().Int("page", 1, "page")
	orgMembersListCmd.Flags().Int("per-page", 100, "per page")
	orgMembersListCmd.Flags().Bool("include-aliyun-uid", false, "merge Aliyun UID via devops ListOrganizationMembers (needs AK)")
	orgMembersSearchCmd.Flags().String("query", "", "search query")
	orgMembersSearchCmd.Flags().Int("page", 1, "page")
	orgMembersSearchCmd.Flags().Int("per-page", 100, "per page")
	orgMembersSearchCmd.Flags().Bool("include-aliyun-uid", false, "merge Aliyun UID via devops ListOrganizationMembers (needs AK)")
	orgMembersCmd.AddCommand(orgMembersListCmd, orgMembersSearchCmd)
	orgUserCmd.AddCommand(orgUserGetCmd)
	orgDepartmentsListCmd.Flags().String("parent-id", "", "optional parent department id")
	orgDepartmentsGetCmd.Flags().String("id", "", "department id (required)")
	orgDepartmentsAncestorsCmd.Flags().String("id", "", "department id (required)")
	orgRolesGetCmd.Flags().String("id", "", "role id (required)")
	orgDepartmentsCmd.AddCommand(orgDepartmentsListCmd, orgDepartmentsGetCmd, orgDepartmentsAncestorsCmd)
	orgRolesCmd.AddCommand(orgRolesListCmd, orgRolesGetCmd)
	organizationCmd.AddCommand(orgUserCmd, orgListCmd, orgMembersCmd, orgDepartmentsCmd, orgRolesCmd, orgWhoamiShortcut)
}
