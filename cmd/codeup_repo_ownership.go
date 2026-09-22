package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/yunxiao-cli/yunxiao/internal/client"
	"github.com/yunxiao-cli/yunxiao/internal/zhiyi"
)

// ensureNumericRepoOwnership validates numeric --repo against profile.repositories
// and organization-reachable repos (GET .../repositories/{id} under current org).
// Alias / org%2Frepo paths skip (aliases already fail when unregistered — #49).
// Mismatch returns a clear error (no --yes override; mrs update is write, not high-risk-write).
func ensureNumericRepoOwnership(ctx context.Context, c *client.Client, repoFlag, repositoryID string) error {
	if !zhiyi.IsNumericRepositoryID(repoFlag) {
		return nil
	}
	allowed := zhiyi.ProfileRepositoryIDSet(nil)
	pf, err := applyActiveProfileOrg()
	if err != nil {
		return err
	}
	if pf != nil {
		allowed = zhiyi.ProfileRepositoryIDSet(pf.Repositories)
	}
	if _, ok := allowed[repositoryID]; ok {
		return nil
	}
	// Organization-reachable: successful GET under current org path adds the id.
	if c == nil {
		return zhiyi.ValidateNumericRepoInAllowlist(repoFlag, allowed)
	}
	path, err := c.CodeupPath(ctx, "/repositories/"+client.EncodeRepoID(repositoryID))
	if err != nil {
		return err
	}
	var repoObj map[string]any
	if err := c.Get(ctx, path, nil, &repoObj); err != nil {
		var ae *client.APIError
		if errors.As(err, &ae) {
			return zhiyi.ValidateNumericRepoInAllowlist(repoFlag, allowed)
		}
		return fmt.Errorf("%w (organization repos get failed: %v)",
			zhiyi.ValidateNumericRepoInAllowlist(repoFlag, allowed), err)
	}
	// Confirm response id when present; otherwise treat GET success as reachable.
	if id := repositoryIDFromRepoObject(repoObj); id != "" && id != repositoryID {
		return zhiyi.ValidateNumericRepoInAllowlist(repoFlag, allowed)
	}
	allowed[repositoryID] = struct{}{}
	return zhiyi.ValidateNumericRepoInAllowlist(repoFlag, allowed)
}

func repositoryIDFromRepoObject(m map[string]any) string {
	if m == nil {
		return ""
	}
	switch v := m["id"].(type) {
	case string:
		return v
	case float64:
		return fmt.Sprintf("%.0f", v)
	case int64:
		return fmt.Sprintf("%d", v)
	case int:
		return fmt.Sprintf("%d", v)
	default:
		if v == nil {
			return ""
		}
		s := fmt.Sprint(v)
		if s == "" || s == "<nil>" {
			return ""
		}
		return s
	}
}
