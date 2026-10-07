package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yunxiao-cli/yunxiao/internal/client"
	"github.com/yunxiao-cli/yunxiao/internal/config"
	"github.com/yunxiao-cli/yunxiao/internal/output"
	"github.com/yunxiao-cli/yunxiao/internal/pipelineyaml"
	"github.com/yunxiao-cli/yunxiao/internal/profile"
	"github.com/yunxiao-cli/yunxiao/internal/risk"
	"github.com/yunxiao-cli/yunxiao/internal/zhiyi"
)

// processExit is os.Exit; tests may override to capture confirmation exit codes
// without killing the test process (cobra Execute → handleErr path).
var processExit = os.Exit

// refreshWarnOut receives post-transition refresh warnings (tests may redirect).
var refreshWarnOut io.Writer = os.Stderr

// getter is the minimal client surface used by refreshAfterTransition.
type getter interface {
	Get(ctx context.Context, path string, query map[string]string, out any) error
}

// refreshAfterTransition GETs the work item after a successful status PUT.
// On GET failure it prints the B4 warning and returns refreshOK=false; callers
// still take the success path with refresh_ok:false in the envelope.
func refreshAfterTransition(ctx context.Context, c getter, path, id string) (refreshed map[string]any, refreshOK bool) {
	refreshOK = true
	if err := c.Get(ctx, path, nil, &refreshed); err != nil {
		refreshOK = false
		fmt.Fprintf(refreshWarnOut, "warning: transition succeeded but refresh failed for %s: %v\n", id, err)
	}
	return refreshed, refreshOK
}

// resolveEffectiveConfig applies active profile org (via setenv) and resolves
// token with precedence: YUNXIAO_ACCESS_TOKEN > credentials.json > profile > config.json.
func resolveEffectiveConfig() (config.Resolved, *profile.Profile, error) {
	pf, err := applyActiveProfileOrg()
	if err != nil {
		return config.Resolved{}, nil, err
	}
	var profileToken string
	if pf != nil {
		profileToken = pf.AccessToken
	}
	r, err := config.ResolveWithProfileToken(profileToken)
	if err != nil {
		return r, pf, err
	}
	return r, pf, nil
}

func mustClient() (*client.Client, config.Resolved, error) {
	r, _, err := resolveEffectiveConfig()
	if err != nil {
		return nil, r, err
	}
	c, err := client.New(r)
	if err != nil {
		return nil, r, err
	}
	if r.TokenKind == config.TokenKindOAuth {
		c.OnRefresh = oauthRefreshHook
	}
	return c, r, nil
}

// apiErrorBody is the error envelope body for an API error: type "api", status code,
// apiErrorHint, and yaml_validation subtype + details for Flow YAML errors.
func apiErrorBody(ae *client.APIError) output.ErrorBody {
	body := output.ErrorBody{
		Type:    "api",
		Message: ae.Error(),
		Hint:    apiErrorHint(ae),
		Code:    ae.Status,
	}
	if code, issues, ok := pipelineyaml.ParseYAMLValidationError(ae.Body); ok {
		body.Subtype = "yaml_validation"
		if body.Hint == "" {
			body.Hint = "Flow YAML validation failed; see error.details.issues (path + errorMessage)"
		}
		body.Details = map[string]any{"errorCode": code, "issues": issues}
	}
	return body
}

// contextError prefixes err with what the CLI was doing ("<Context>: <err>") and adds
// Hint. handleErr still reports a wrapped *client.APIError exactly like an unwrapped one
// (type "api", status code, subtype/details) with the API hint and Hint joined by "; ";
// other causes are type "cli".
type contextError struct {
	Context string
	Hint    string
	Err     error
}

func (e *contextError) Error() string { return e.Context + ": " + e.Err.Error() }
func (e *contextError) Unwrap() error { return e.Err }

func handleErr(err error) {
	if err == nil {
		return
	}
	if g, ok := err.(risk.GateResult); ok {
		_ = output.Fail(output.ErrorBody{
			Type:    "confirmation",
			Subtype: "confirmation_required",
			Message: g.Error(),
			Hint:    g.Hint,
			Risk:    string(g.Risk),
			Action:  g.Action,
		}, risk.ExitConfirmationRequired)
		processExit(risk.ExitConfirmationRequired)
	}
	if ee, ok := err.(output.ExitError); ok {
		processExit(ee.Code)
	}
	if ce, ok := err.(*contextError); ok {
		body := output.ErrorBody{Type: "cli", Message: ce.Error(), Hint: ce.Hint}
		var ae *client.APIError
		if errors.As(ce.Err, &ae) {
			// Same body as an unwrapped APIError (type/code/hint/subtype/details),
			// with the context-prefixed message and the context hint appended.
			body = apiErrorBody(ae)
			body.Message = ce.Error()
			switch {
			case ce.Hint == "":
			case body.Hint == "":
				body.Hint = ce.Hint
			default:
				body.Hint += "; " + ce.Hint
			}
		}
		_ = output.Fail(body, 1)
		processExit(1)
		return
	}
	if ae, ok := err.(*client.APIError); ok {
		_ = output.Fail(apiErrorBody(ae), 1)
		processExit(1)
	}
	if de, ok := err.(*detailedError); ok {
		_ = output.Fail(output.ErrorBody{
			Type:    "cli",
			Subtype: de.Subtype,
			Message: de.Message,
			Hint:    de.Hint,
			Details: de.Details,
		}, 1)
		processExit(1)
		return
	}
	_ = output.Fail(output.ErrorBody{
		Type:    "cli",
		Message: err.Error(),
	}, 1)
	processExit(1)
}

// detailedError is a client-side failure with a machine-readable subtype, hint and
// details (e.g. workitem create precheck, #95); handleErr reports it as type "cli".
type detailedError struct {
	Subtype string
	Message string
	Hint    string
	Details map[string]any
}

func (e *detailedError) Error() string { return e.Message }

func flagOrg(explicit string) {
	if explicit != "" {
		_ = os.Setenv(config.EnvOrganizationID, explicit)
	}
}

func requireFlags(pairs ...string) error {
	for i := 0; i+1 < len(pairs); i += 2 {
		if pairs[i+1] == "" {
			return fmt.Errorf("missing required flag --%s", pairs[i])
		}
	}
	return nil
}

// readFlagOrFile returns text from --name or --name-file (UTF-8, BOM stripped).
// Mutual exclusion is always enforced. When required, at least one must be set
// and the resulting content must be non-empty after TrimSpace.
func readFlagOrFile(inline, file, flagName string, required bool) (string, error) {
	inlineFlag := "--" + flagName
	fileFlag := "--" + flagName + "-file"
	if inline != "" && file != "" {
		return "", fmt.Errorf("use only one of %s or %s", inlineFlag, fileFlag)
	}
	var text string
	var usedFlag string
	if file != "" {
		path, err := resolveContentFilePath(file, flagName+"-file")
		if err != nil {
			return "", err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		text = string(stripUTF8BOM(b))
		usedFlag = fileFlag
	} else if inline == "" {
		if required {
			return "", fmt.Errorf("missing %s or %s", inlineFlag, fileFlag)
		}
		return "", nil
	} else {
		text = inline
		usedFlag = inlineFlag
	}
	if required && strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("empty %s", usedFlag)
	}
	return text, nil
}

func readContentInput(content, contentFile string) (string, error) {
	return readFlagOrFile(content, contentFile, "content", true)
}

// readJSONMapFlagOrFile reads an optional JSON object from --name or --name-file
// (UTF-8, BOM stripped). Empty returns (nil, nil).
func readJSONMapFlagOrFile(inline, file, flagName string) (map[string]any, error) {
	text, err := readFlagOrFile(inline, file, flagName, false)
	if err != nil {
		return nil, err
	}
	return parseJSONMap(text)
}

// stripUTF8BOM removes a leading UTF-8 BOM (EF BB BF) so Windows editors
// that save "UTF-8 with BOM" do not send a garbage prefix to OpenAPI.
func stripUTF8BOM(b []byte) []byte {
	if len(b) >= 3 && b[0] == 0xEF && b[1] == 0xBB && b[2] == 0xBF {
		return b[3:]
	}
	return b
}

// resolveContentFilePath accepts cwd-relative paths or absolute paths (incl. Windows-style
// drive paths like C:\foo when running on Windows). Relative paths may not escape via "..".
// flagName is the CLI flag without leading dashes (e.g. "subject-file", "content-file").
func resolveContentFilePath(p, flagName string) (string, error) {
	if flagName == "" {
		flagName = "content-file"
	}
	flag := "--" + flagName
	p = strings.TrimSpace(p)
	if p == "" {
		return "", fmt.Errorf("empty %s", flag)
	}
	// Normalize Windows drive paths on non-Windows (treat as absolute if they look like one).
	if looksAbsolutePath(p) {
		return p, nil
	}
	cleaned := filepath.Clean(p)
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe file path: %s relative path must stay under cwd (got %q)", flag, p)
	}
	return cleaned, nil
}

// looksAbsolutePath reports whether p should be treated as an absolute path:
// platform-absolute (filepath.IsAbs), volume-less rooted ("/x" — absolute on
// POSIX, drive-rooted on Windows where it resolves against the current drive's
// root rather than cwd), a Windows drive path (C:/ or C:\), or UNC (\\x, //x).
// The Windows-style forms are recognized on every platform so path validation
// behaves the same regardless of where the CLI runs.
func looksAbsolutePath(p string) bool {
	if filepath.IsAbs(p) {
		return true
	}
	// Rooted without a volume: "/x" is absolute on POSIX and drive-rooted on
	// Windows, so treat it as absolute on both. A leading "\" is only a path
	// separator on Windows (on POSIX it is a legal filename character).
	if strings.HasPrefix(p, "/") || (os.PathSeparator == '\\' && strings.HasPrefix(p, `\`)) {
		return true
	}
	// Windows-style drive path (C:/ or C:\) or UNC
	if len(p) >= 3 {
		drive := p[0]
		if ((drive >= 'A' && drive <= 'Z') || (drive >= 'a' && drive <= 'z')) && p[1] == ':' {
			if p[2] == '\\' || p[2] == '/' {
				return true
			}
		}
	}
	if strings.HasPrefix(p, "\\\\") || strings.HasPrefix(p, "//") {
		return true
	}
	return false
}

// loadJSONBodyFromFlags reads JSON from --data and/or --data-file.
// Rules:
// - both empty → (nil, nil)
// - both set → error mutual exclusion
// - dataFile set → resolveContentFilePath + os.ReadFile + json.Unmarshal
// - data starts with "@" → treat rest as file path (same resolve rules), else Unmarshal string
func loadJSONBodyFromFlags(data, dataFile string) (any, error) {
	data = strings.TrimSpace(data)
	dataFile = strings.TrimSpace(dataFile)
	if data == "" && dataFile == "" {
		return nil, nil
	}
	if data != "" && dataFile != "" {
		return nil, fmt.Errorf("use only one of --data or --data-file")
	}
	var raw []byte
	if dataFile != "" {
		path, err := resolveContentFilePath(dataFile, "data-file")
		if err != nil {
			return nil, err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		raw = b
	} else if strings.HasPrefix(data, "@") {
		path, err := resolveContentFilePath(strings.TrimPrefix(data, "@"), "data")
		if err != nil {
			return nil, err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		raw = b
	} else {
		raw = []byte(data)
	}
	var body any
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	return body, nil
}

func apiErrorHint(ae *client.APIError) string {
	if ae == nil {
		return ""
	}
	body := ae.Body
	msg := ae.Error()
	combined := body + " " + msg
	if ae.Status == 403 && (strings.Contains(ae.URL, "/pipelines/") || strings.Contains(combined, "pipeline")) {
		return "insufficient role on this pipeline (permissions are per-pipeline); check with: yunxiao pipeline resource-members list --resource-type pipeline --resource-id <id>; ask an owner to grant access in the Flow web UI"
	}
	if strings.Contains(combined, "未启用此字段【迭代】") {
		return "omit --sprint for this workitem type (field 迭代 is not enabled)"
	}
	if strings.Contains(combined, "未启用此字段") {
		return "one or more fields are not enabled on this type; for +bug-create try --minimal or remove module/environment/ExpCompletionTime from profile bug_create_fields; run: yunxiao profile doctor"
	}
	if strings.Contains(combined, "非高级版组织，不支持此功能") {
		return "programs (project sets) require an Advanced-edition Yunxiao organization; this org cannot use `programs search` — use `yunxiao project list` for projects instead, or upgrade the org plan"
	}
	if hint := zhiyi.RequiredFieldHint(combined); hint != "" {
		return hint
	}
	return ""
}

func runMutating(action string, level risk.Level, dryRun, yes bool, preview any, execFn func() error) error {
	if dryRun {
		return output.DryRunResult(string(level), preview)
	}
	if level == risk.HighRiskWrite {
		if err := risk.CheckHighRisk(action, yes); err != nil {
			return err
		}
	}
	return execFn()
}

// runRead is the common read path: dry-run preview, or Do + Success with pagination meta.
// baseMeta may be nil; risk=read is set when missing. after may mutate out/meta (e.g. URL enrich).
func runRead(ctx context.Context, c *client.Client, method, path string, query map[string]string, body any, baseMeta map[string]any, after func(out any, meta map[string]any) (any, map[string]any)) error {
	if baseMeta == nil {
		baseMeta = map[string]any{}
	}
	if _, ok := baseMeta["risk"]; !ok {
		baseMeta["risk"] = risk.Read
	}
	if globalDryRun {
		return output.DryRunResult(string(risk.Read), c.Preview(method, path, query, body))
	}
	var out any
	hdr, err := c.Do(ctx, method, path, query, body, &out)
	if err != nil {
		return err
	}
	meta := client.MetaWithPagination(baseMeta, hdr)
	if after != nil {
		out, meta = after(out, meta)
	}
	return output.Success(out, meta)
}

// runReadAll follows list pages via client.ListAll (cap DefaultListAllMaxPages).
// Used by list commands that expose --all. after may enrich the concatenated slice.
func runReadAll(ctx context.Context, c *client.Client, method, path string, query map[string]string, perPage int, baseMeta map[string]any, after func(out any, meta map[string]any) (any, map[string]any)) error {
	if baseMeta == nil {
		baseMeta = map[string]any{}
	}
	if _, ok := baseMeta["risk"]; !ok {
		baseMeta["risk"] = risk.Read
	}
	baseQ := map[string]string{}
	for k, v := range query {
		if k == "page" || k == "perPage" {
			continue
		}
		baseQ[k] = v
	}
	if globalDryRun {
		q := client.PageQuery(1, perPage)
		for k, v := range baseQ {
			q[k] = v
		}
		return output.DryRunResult(string(risk.Read), c.Preview(method, path, q, nil))
	}
	fetch := func(ctx context.Context, q map[string]string) (any, http.Header, error) {
		var out any
		hdr, err := c.Do(ctx, method, path, q, nil, &out)
		return out, hdr, err
	}
	res, err := client.ListAll(ctx, 1, perPage, client.DefaultListAllMaxPages, baseQ, fetch)
	if err != nil {
		return err
	}
	meta := map[string]any{}
	for k, v := range baseMeta {
		meta[k] = v
	}
	for k, v := range res.Meta {
		meta[k] = v
	}
	out := any(res.Items)
	if after != nil {
		out, meta = after(out, meta)
	}
	return output.Success(out, meta)
}

// runJSONMutating is the common write path for JSON body methods via runMutating + Do.
// Keeps HighRiskWrite/--yes and dry-run identical to runMutating. after may enrich meta.
func runJSONMutating(ctx context.Context, c *client.Client, action string, level risk.Level, method, path string, query map[string]string, body any, after func(out any, meta map[string]any) (any, map[string]any)) error {
	return runJSONMutatingPreview(ctx, c, action, level, method, path, query, body, c.Preview(method, path, query, body), after)
}

// runJSONMutatingPreview is runJSONMutating with a caller-built dry-run preview;
// callers pass a request-preview builder (e.g. mrs comments create, workitem create).
func runJSONMutatingPreview(ctx context.Context, c *client.Client, action string, level risk.Level, method, path string, query map[string]string, body, preview any, after func(out any, meta map[string]any) (any, map[string]any)) error {
	return runMutating(action, level, globalDryRun, globalYes, preview, func() error {
		var out any
		_, err := c.Do(ctx, method, path, query, body, &out)
		if err != nil {
			return err
		}
		meta := map[string]any{"risk": level}
		if after != nil {
			out, meta = after(out, meta)
		}
		return output.Success(out, meta)
	})
}

// parseJSONMap parses a JSON object string into map[string]any.
func parseJSONMap(s string) (map[string]any, error) {
	if s == "" {
		return nil, nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil, fmt.Errorf("invalid JSON object: %w", err)
	}
	return m, nil
}

// assertRelativePath rejects absolute paths and parent traversal (same rule as
// --content-file, incl. Windows drive/UNC/rooted paths — a leading "/" or "\"
// escapes cwd on Windows via the current drive's root).
func assertRelativePath(p string) error {
	if p == "" {
		return fmt.Errorf("empty path")
	}
	if looksAbsolutePath(p) || strings.HasPrefix(filepath.Clean(p), "..") {
		return fmt.Errorf("unsafe file path: must be a relative path under cwd (got %q)", p)
	}
	return nil
}

// readRelativeFile reads a relative path under cwd; returns bytes and basename.
func readRelativeFile(p string) ([]byte, string, error) {
	if err := assertRelativePath(p); err != nil {
		return nil, "", err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, "", err
	}
	return b, filepath.Base(p), nil
}

// readContentOrFile returns text from --content or relative --file (mutually exclusive).
func readContentOrFile(content, file string) (string, error) {
	if content != "" && file != "" {
		return "", fmt.Errorf("use only one of --content or --file")
	}
	if file != "" {
		b, _, err := readRelativeFile(file)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	if content == "" {
		return "", fmt.Errorf("missing --content or --file")
	}
	return content, nil
}

func activeProfileName() string {
	return profile.ResolveName(globalProfile)
}

// applyActiveProfileOrg loads the active profile (if any) and sets org env when empty.
// Returns the loaded profile or nil when no profile name is set. Missing file is an error only when name is set.
func applyActiveProfileOrg() (*profile.Profile, error) {
	name := activeProfileName()
	if name == "" {
		return nil, nil
	}
	p, err := profile.Load(name)
	if err != nil {
		return nil, err
	}
	if p.OrganizationID != "" && strings.TrimSpace(os.Getenv(config.EnvOrganizationID)) == "" {
		_ = os.Setenv(config.EnvOrganizationID, p.OrganizationID)
	}
	return p, nil
}

// requireProfile loads the active profile or returns HintMissing.
func requireProfile() (*profile.Profile, error) {
	name := activeProfileName()
	if name == "" {
		return nil, profile.HintMissing()
	}
	p, err := profile.Load(name)
	if err != nil {
		return nil, err
	}
	if p.OrganizationID != "" && strings.TrimSpace(os.Getenv(config.EnvOrganizationID)) == "" {
		_ = os.Setenv(config.EnvOrganizationID, p.OrganizationID)
	}
	return p, nil
}

// workitemIDFromFlagOrArg returns --id or first positional arg.
func workitemIDFromFlagOrArg(cmd *cobra.Command, args []string) (string, error) {
	id, _ := cmd.Flags().GetString("id")
	id = strings.TrimSpace(id)
	if id == "" && len(args) > 0 {
		id = strings.TrimSpace(args[0])
	}
	if id == "" {
		return "", fmt.Errorf("missing work item id: pass --id or positional arg")
	}
	return id, nil
}

// resolveCodeupRepo resolves --repo as numeric id, profile.repositories alias, or org/repo path.
func resolveCodeupRepo(repoFlag string) (string, error) {
	pf, err := applyActiveProfileOrg()
	if err != nil {
		return "", err
	}
	var repos map[string]int64
	if pf != nil {
		repos = pf.Repositories
	}
	return zhiyi.ResolveRepositoryID(repoFlag, repos)
}

// asStringMap coerces decoded JSON objects to map[string]any.
func asStringMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return nil
}

// profileSpaceID returns active profile space_id when a profile is loaded.
func profileSpaceID() string {
	pf, err := applyActiveProfileOrg()
	if err != nil || pf == nil {
		return ""
	}
	return pf.SpaceID
}

// coalesceSpaceID returns flag space id, else profile space_id.
// When both empty, returns a clear CLI usage error (caller must not hit the API).
func coalesceSpaceID(flagVal, profileVal string) (string, error) {
	flagVal = strings.TrimSpace(flagVal)
	if flagVal != "" {
		return flagVal, nil
	}
	profileVal = strings.TrimSpace(profileVal)
	if profileVal != "" {
		return profileVal, nil
	}
	return "", fmt.Errorf("missing --space-id (and active profile has no space_id); pass --space-id <projectId> or run `yunxiao +onboard` / set profile space_id")
}

// resolveSpaceIDFlag resolves --space-id with profile.space_id fallback.
func resolveSpaceIDFlag(flagVal string) (string, error) {
	return coalesceSpaceID(flagVal, profileSpaceID())
}

// afterSortByTime sorts list payloads by update/modified time (default newest-first) then calls next.
// sortFlag is the --sort value (asc|desc); empty defaults to desc. Invalid values return an error.
func afterSortByTime(sortFlag string, next func(out any, meta map[string]any) (any, map[string]any)) (func(out any, meta map[string]any) (any, map[string]any), error) {
	return afterSortByTimePref(sortFlag, client.PreferUpdateTime, next)
}

// afterSortByCreateTime is like afterSortByTime but ranks by create time first (comment lists).
func afterSortByCreateTime(sortFlag string, next func(out any, meta map[string]any) (any, map[string]any)) (func(out any, meta map[string]any) (any, map[string]any), error) {
	return afterSortByTimePref(sortFlag, client.PreferCreateTime, next)
}

func afterSortByTimePref(sortFlag string, pref client.TimeKeyPreference, next func(out any, meta map[string]any) (any, map[string]any)) (func(out any, meta map[string]any) (any, map[string]any), error) {
	desc, err := client.ParseSortDescending(sortFlag)
	if err != nil {
		return nil, err
	}
	return func(out any, meta map[string]any) (any, map[string]any) {
		out = client.SortListByTimePref(out, desc, pref)
		if meta == nil {
			meta = map[string]any{}
		}
		if desc {
			meta["sort"] = "desc"
		} else {
			meta["sort"] = "asc"
		}
		if next != nil {
			return next(out, meta)
		}
		return out, meta
	}, nil
}

// addSortFlag registers --sort with default newest-first (desc).
// Client-side sort applies to the current response page when the command is paginated.
func addSortFlag(c *cobra.Command) {
	c.Flags().String("sort", "desc", "asc|desc (default newest first; page-local when paginated)")
}
