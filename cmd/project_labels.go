package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yunxiao-cli/yunxiao/internal/client"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
)

var projectLabelsCmd = &cobra.Command{
	Use:   "labels",
	Short: "Projex project labels (list / create)",
}

var projectLabelsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List labels in a project/space",
	Long: `Risk: read
HTTP: GET .../projects/{spaceId}/labels

Returns label id / name / color (OpenAPI ListLabels). --space-id defaults to
profile.space_id when set (#141).

  yunxiao project labels list --space-id <id>
  yunxiao project labels list --profile zhiyi`,
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		spaceID, _ := cmd.Flags().GetString("space-id")
		spaceID, err := resolveSpaceIDFlag(spaceID)
		if err != nil {
			handleErr(err)
			return
		}
		page, _ := cmd.Flags().GetInt("page")
		perPage, _ := cmd.Flags().GetInt("per-page")
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.ProjexPath(cmd.Context(), "/projects/"+spaceID+"/labels")
		if err != nil {
			handleErr(err)
			return
		}
		q := client.PageQuery(page, perPage)
		handleErr(runRead(cmd.Context(), c, "GET", path, q, nil, map[string]any{
			"risk":    risk.Read,
			"spaceId": spaceID,
		}, nil))
	},
}

var projectLabelsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a label in a project/space",
	Long: `Risk: write (requires --yes for real run; prefer --dry-run first)
HTTP: POST .../projects/{spaceId}/labels

Body: name (required), color (default #A773E0). Returns the new label id (#141).

  yunxiao project labels create --name "快速通道" --dry-run
  yunxiao project labels create --name "快速通道" --color "#4676E5" --yes`,
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		spaceID, _ := cmd.Flags().GetString("space-id")
		spaceID, err := resolveSpaceIDFlag(spaceID)
		if err != nil {
			handleErr(err)
			return
		}
		name, _ := cmd.Flags().GetString("name")
		color, _ := cmd.Flags().GetString("color")
		name = strings.TrimSpace(name)
		color = strings.TrimSpace(color)
		if name == "" {
			handleErr(fmt.Errorf("missing required flag --name"))
			return
		}
		if color == "" {
			color = "#A773E0"
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		path, err := c.ProjexPath(cmd.Context(), "/projects/"+spaceID+"/labels")
		if err != nil {
			handleErr(err)
			return
		}
		body := map[string]any{"name": name, "color": color}
		handleErr(runJSONMutating(cmd.Context(), c, "project labels create", risk.Write, "POST", path, nil, body, nil))
	},
}

func init() {
	projectLabelsListCmd.Flags().String("space-id", "", "project/space id (default: profile.space_id; required)")
	projectLabelsListCmd.Flags().Int("page", 1, "page")
	projectLabelsListCmd.Flags().Int("per-page", 100, "per page")
	projectLabelsCreateCmd.Flags().String("space-id", "", "project/space id (default: profile.space_id; required)")
	projectLabelsCreateCmd.Flags().String("name", "", "label name (required)")
	projectLabelsCreateCmd.Flags().String("color", "#A773E0", "label color (e.g. #A773E0)")
	projectLabelsCmd.AddCommand(projectLabelsListCmd, projectLabelsCreateCmd)
	projectCmd.AddCommand(projectLabelsCmd)
}
