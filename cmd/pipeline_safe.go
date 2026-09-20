package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/yunxiao-cli/yunxiao/internal/client"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/pipelineyaml"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
	"github.com/yunxiao-cli/yunxiao/internal/zhiyi"
)

var pipelineDiffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Diff current pipeline flow YAML against a local file",
	Long: `Risk: read
HTTP: GET .../pipelines/{id} then compare pipelineConfig.flow to --file/--content.

Outputs a stage/job/step summary (+added ~modified -removed). Deleting stages/jobs
or changing deploy/script-like units sets high_risk=true.

  yunxiao pipeline diff --id <id> --file new.yaml`,
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		id, _ := cmd.Flags().GetString("id")
		contentFlag, _ := cmd.Flags().GetString("content")
		file, _ := cmd.Flags().GetString("file")
		if err := requireFlags("id", id); err != nil {
			handleErr(err)
			return
		}
		newYAML, err := readContentOrFile(contentFlag, file)
		if err != nil {
			handleErr(err)
			return
		}
		c, _, err := mustClient()
		if err != nil {
			handleErr(err)
			return
		}
		cur, err := fetchPipelineFlowYAML(cmd.Context(), c, id)
		if err != nil {
			handleErr(err)
			return
		}
		diff, err := pipelineyaml.DiffYAML(cur, newYAML)
		if err != nil {
			handleErr(err)
			return
		}
		data := map[string]any{
			"pipeline_id": id,
			"diff":        diff,
		}
		meta := map[string]any{"risk": risk.Read}
		if u := zhiyi.PipelineURL(id); u != "" {
			meta["url"] = u
		}
		handleErr(output.Success(data, meta))
	},
}

func fetchPipelineFlowYAML(ctx context.Context, c *client.Client, id string) (string, error) {
	path, err := c.FlowPath(ctx, "/pipelines/"+id)
	if err != nil {
		return "", err
	}
	var out map[string]any
	if err := c.Get(ctx, path, nil, &out); err != nil {
		return "", err
	}
	return pipelineyaml.ExtractFlowYAML(out)
}

func writePipelineYAMLFile(path, yamlText string) error {
	if path == "" {
		return fmt.Errorf("empty yaml output path")
	}
	if err := assertRelativePath(path); err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, []byte(yamlText), 0o644)
}

func init() {
	pipelineDiffCmd.Flags().String("id", "", "pipeline id (required)")
	pipelineDiffCmd.Flags().String("content", "", "new pipeline YAML content")
	pipelineDiffCmd.Flags().String("file", "", "relative path to new YAML file")
}
