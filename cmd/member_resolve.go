package cmd

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"github.com/spf13/cobra"
	"github.com/yunxiao-cli/yunxiao/internal/client"
	"github.com/yunxiao-cli/yunxiao/internal/orguid"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
)

// memberHit is one organization member matched by display name (#367).
type memberHit struct {
	ID    string
	Name  string
	Email string
}

// orgMemberResolveCmd resolves a display name to userId/name/email via members:search.
// Unique exact match → success; none → error; several → list candidates and exit ≠ 0
// unless --pick first.
var orgMemberResolveCmd = &cobra.Command{
	Use:   "+member-resolve",
	Short: "Resolve organization member by display name",
	Long: `Risk: read
HTTP: POST .../members:search

Resolve a Chinese (or other) display name to userId / name / email for assignedTo.
Exact match (trim, case-insensitive) only — never fuzzy-guesses.

Exit:
  0  unique match (or --pick first with ≥1 exact match)
  1  no match, or several matches without --pick first

Prefer this over clientside filtering of members list. Raw API search remains:
  yunxiao organization members search --query <q>

Workitem --assigned-to accepts the same display names (non-id values) via the
same resolver (#367).`,
	Example: `  yunxiao organization +member-resolve --name 崔健
  yunxiao organization +member-resolve --name 崔健 --pick first
  yunxiao workitem create ... --assigned-to 崔健 --dry-run`,
	Run: func(cmd *cobra.Command, args []string) {
		flagOrg(globalOrg)
		name, _ := cmd.Flags().GetString("name")
		pick, _ := cmd.Flags().GetString("pick")
		pickFirst := strings.EqualFold(strings.TrimSpace(pick), "first")
		if strings.TrimSpace(pick) != "" && !pickFirst {
			handleErr(fmt.Errorf("--pick only supports \"first\" (got %q)", pick))
			return
		}
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
		body := map[string]any{"query": strings.TrimSpace(name), "page": 1, "perPage": 50}
		if globalDryRun {
			handleErr(output.DryRunResult(string(risk.Read), c.Preview("POST", path, nil, body)))
			return
		}
		hit, err := resolveMemberByName(cmd.Context(), c, name, pickFirst)
		if err != nil {
			handleErr(err)
			return
		}
		data := map[string]any{
			"userId": hit.ID,
			"name":   hit.Name,
			"email":  hit.Email,
		}
		handleErr(output.Success(data, map[string]any{
			"risk":        risk.Read,
			"resolved_by": "members:search",
			"pick":        pickFirst,
		}))
	},
}

// resolveMemberByName finds exact display-name matches via POST members:search.
// pickFirst: when several exact matches, return the first (stable API order).
func resolveMemberByName(ctx context.Context, c *client.Client, name string, pickFirst bool) (memberHit, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return memberHit{}, fmt.Errorf("--name is empty")
	}
	matches, err := searchMembersExactName(ctx, c, name)
	if err != nil {
		return memberHit{}, err
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return memberHit{}, fmt.Errorf("no organization member named %q (via members:search); pass a userId instead", name)
	default:
		if pickFirst {
			return matches[0], nil
		}
		parts := make([]string, 0, len(matches))
		for _, m := range matches {
			p := fmt.Sprintf("%s (%s)", m.ID, m.Name)
			if m.Email != "" {
				p = fmt.Sprintf("%s (%s, %s)", m.ID, m.Name, m.Email)
			}
			parts = append(parts, p)
		}
		return memberHit{}, fmt.Errorf("name %q is ambiguous: %s — re-run with --pick first or pass a userId", name, strings.Join(parts, ", "))
	}
}

// searchMembersExactName POSTs members:search and keeps exact name matches only.
func searchMembersExactName(ctx context.Context, c *client.Client, name string) ([]memberHit, error) {
	name = strings.TrimSpace(name)
	path, err := c.PlatformPath(ctx, "/members:search")
	if err != nil {
		return nil, err
	}
	var out any
	if err := c.Post(ctx, path, map[string]any{"query": name, "page": 1, "perPage": 50}, &out); err != nil {
		return nil, fmt.Errorf("members:search for %q: %w", name, err)
	}
	return filterExactMemberHits(orguid.MembersFromAPI(out), name), nil
}

// filterExactMemberHits keeps members whose name equals want (trim, EqualFold).
func filterExactMemberHits(items []any, want string) []memberHit {
	want = strings.TrimSpace(want)
	var matches []memberHit
	seen := map[string]bool{}
	for _, it := range items {
		m, ok := it.(map[string]any)
		if !ok {
			continue
		}
		n, _ := m["name"].(string)
		if !strings.EqualFold(strings.TrimSpace(n), want) {
			continue
		}
		id := memberIDString(m)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		email := memberEmailString(m)
		matches = append(matches, memberHit{ID: id, Name: strings.TrimSpace(n), Email: email})
	}
	return matches
}

func memberEmailString(m map[string]any) string {
	for _, k := range []string{"email", "Email"} {
		switch v := m[k].(type) {
		case string:
			if s := strings.TrimSpace(v); s != "" {
				return s
			}
		}
	}
	return ""
}

// looksLikeUserID reports whether v is safe to treat as a raw user id (ASCII
// letters/digits/hyphen/underscore only). Display names with CJK or spaces
// return false so --assigned-to 崔健 goes through members:search (#367).
func looksLikeUserID(v string) bool {
	v = strings.TrimSpace(v)
	if v == "" {
		return false
	}
	for _, r := range v {
		if unicode.IsLetter(r) && r > unicode.MaxASCII {
			return false
		}
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			continue
		}
		return false
	}
	return true
}

// resolveAssignedToRef resolves --assigned-to:
//
//	self → current user id
//	ASCII id-shaped value → passthrough (backward compatible)
//	otherwise → exact display-name match via members:search (#367)
func resolveAssignedToRef(ctx context.Context, c *client.Client, v string) (string, error) {
	v = strings.TrimSpace(v)
	if v == "" || v == "self" {
		return resolveSelfID(ctx, c, v)
	}
	if looksLikeUserID(v) {
		return v, nil
	}
	hit, err := resolveMemberByName(ctx, c, v, false)
	if err != nil {
		return "", fmt.Errorf("resolve --assigned-to %q: %w", v, err)
	}
	return hit.ID, nil
}

func init() {
	orgMemberResolveCmd.Flags().String("name", "", "member display name (required; exact match)")
	orgMemberResolveCmd.Flags().String("pick", "", `when several exact matches: "first" takes the first candidate`)
	_ = orgMemberResolveCmd.MarkFlagRequired("name")
	organizationCmd.AddCommand(orgMemberResolveCmd)
}
