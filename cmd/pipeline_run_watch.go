package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/pipelinegate"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
	"github.com/yunxiao-cli/yunxiao/internal/zhiyi"
)

var pipelineRunWatchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Poll a pipeline run until terminal or manual-gate pause",
	Long: `Risk: read

Poll GET .../pipelines/{id}/runs/{runId} until the run reaches a terminal state
or pauses on an unhandled manual gate.

Exit codes:
  0  success
  1  fail / error / unknown terminal
  2  canceled
  3  gate_paused (WAITING/RUNNING with pending pass|refuse jobs)
  4  timeout

Flags:
  --pipeline-id   required
  --run-id        required
  --interval      poll interval (default 5s)
  --timeout       overall timeout (default 30m)

Progress lines go to stderr; final Success envelope is on stdout.`,
	Run: runPipelineRunWatch,
}

func runPipelineRunWatch(cmd *cobra.Command, _ []string) {
	flagOrg(globalOrg)
	pid, _ := cmd.Flags().GetString("pipeline-id")
	rid, _ := cmd.Flags().GetString("run-id")
	interval, _ := cmd.Flags().GetDuration("interval")
	timeout, _ := cmd.Flags().GetDuration("timeout")
	if err := requireFlags("pipeline-id", pid, "run-id", rid); err != nil {
		handleErr(err)
		return
	}
	if interval <= 0 {
		interval = 5 * time.Second
	}
	if timeout <= 0 {
		timeout = 30 * time.Minute
	}

	c, _, err := mustClient()
	if err != nil {
		handleErr(err)
		return
	}
	path, err := c.FlowPath(cmd.Context(), "/pipelines/"+pid+"/runs/"+rid)
	if err != nil {
		handleErr(err)
		return
	}
	if globalDryRun {
		handleErr(output.DryRunResult(string(risk.Read), c.Preview("GET", path, nil, nil)))
		return
	}

	deadline := time.Now().Add(timeout)
	var lastStatus string
	var lastPending []pipelinegate.PendingJob
	var lastDetail map[string]any

	for {
		detail, err := fetchRunDetail(cmd.Context(), c, pid, rid)
		if err != nil {
			handleErr(err)
			return
		}
		lastDetail = detail
		lastStatus = firstNonEmpty(
			stringifyAny(detail["status"]),
			stringifyAny(detail["resultStatus"]),
			stringifyAny(detail["pipelineStatus"]),
		)
		if data := asStringMap(detail["data"]); data != nil && lastStatus == "" {
			lastStatus = firstNonEmpty(
				stringifyAny(data["status"]),
				stringifyAny(data["resultStatus"]),
			)
		}
		lastPending = pipelinegate.ExtractPendingJobs(detail, pid)
		outcome, code := pipelinegate.ClassifyWatchOutcome(lastStatus, lastPending)
		fmt.Fprintf(os.Stderr, "pipeline run watch: status=%s pending=%d outcome=%s\n", lastStatus, len(lastPending), outcome)
		if code >= 0 {
			meta := map[string]any{
				"risk":    risk.Read,
				"outcome": outcome,
				"status":  lastStatus,
			}
			zhiyi.EnrichPipelineRunMeta(meta, lastDetail, pid)
			data := map[string]any{
				"pipelineId": pid,
				"runId":      rid,
				"status":     lastStatus,
				"outcome":    outcome,
				"pending":    lastPending,
				"run":        zhiyi.AttachPipelineRunURLs(lastDetail, pid),
			}
			_ = output.Success(data, meta)
			if code != 0 {
				processExit(code)
			}
			return
		}
		if time.Now().After(deadline) {
			meta := map[string]any{
				"risk":    risk.Read,
				"outcome": "timeout",
				"status":  lastStatus,
			}
			zhiyi.EnrichPipelineRunMeta(meta, lastDetail, pid)
			data := map[string]any{
				"pipelineId": pid,
				"runId":      rid,
				"status":     lastStatus,
				"outcome":    "timeout",
				"pending":    lastPending,
				"run":        zhiyi.AttachPipelineRunURLs(lastDetail, pid),
			}
			_ = output.Success(data, meta)
			processExit(4)
			return
		}
		select {
		case <-cmd.Context().Done():
			handleErr(cmd.Context().Err())
			return
		case <-time.After(interval):
		}
	}
}
