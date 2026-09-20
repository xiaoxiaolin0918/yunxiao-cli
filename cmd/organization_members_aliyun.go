package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/yunxiao-cli/yunxiao/internal/client"
	"github.com/yunxiao-cli/yunxiao/internal/orguid"
)

// requireAliyunUIDAK fails fast when --include-aliyun-uid is set without AccessKey env.
func requireAliyunUIDAK() error {
	if _, ok := orguid.LoadAKEnv(); !ok {
		return fmt.Errorf("%s", orguid.MissingAKMessage())
	}
	return nil
}

// membersAliyunUIDAfter enriches oapi members with aliyunUid from devops ListOrganizationMembers.
// Call requireAliyunUIDAK before runRead when the flag is set. On enrich failure this calls
// handleErr (process exit); it does not return a half-success envelope.
func membersAliyunUIDAfter(cmd *cobra.Command, c *client.Client, nameQuery string) func(out any, meta map[string]any) (any, map[string]any) {
	return func(out any, meta map[string]any) (any, map[string]any) {
		ak, ok := orguid.LoadAKEnv()
		if !ok {
			handleErr(fmt.Errorf("%s", orguid.MissingAKMessage()))
			return out, meta
		}
		orgID, err := c.ResolveOrgID(cmd.Context())
		if err != nil {
			handleErr(err)
			return out, meta
		}
		cli := &orguid.DevOpsMembersClient{AK: ak}
		legacy, err := cli.ListOrganizationMembers(cmd.Context(), orgID, nameQuery)
		if err != nil {
			handleErr(fmt.Errorf("list aliyun UIDs: %w", err))
			return out, meta
		}
		items := orguid.MembersFromAPI(out)
		if items == nil {
			if m, ok := out.(map[string]any); ok {
				items = []any{m}
				enriched, emeta := orguid.AttachAliyunUID(items, legacy)
				for k, v := range emeta {
					meta[k] = v
				}
				return enriched[0], meta
			}
			meta["aliyun_uid_warning"] = "unexpected members payload shape; skipped merge"
			return out, meta
		}
		enriched, emeta := orguid.AttachAliyunUID(items, legacy)
		for k, v := range emeta {
			meta[k] = v
		}
		return orguid.ReplaceMembersInAPI(out, enriched), meta
	}
}
