package profile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yunxiao-cli/yunxiao/internal/config"
)

const (
	EnvProfile = "YUNXIAO_PROFILE"
	DirName    = "profiles"
)

// BugCreateFields holds alias→id maps and field keys used when creating bugs.
type BugCreateFields struct {
	Priority          map[string]string `json:"priority,omitempty"`
	SeriousLevel      map[string]string `json:"serious_level,omitempty"`
	Module            string            `json:"module,omitempty"`
	Environment       string            `json:"environment,omitempty"`
	ExpCompletionTime string            `json:"ExpCompletionTime,omitempty"`
}

// WorkitemWorkflow is a discovered status graph for one workitem type_id.
// Profiles are project-scoped (space_id); Workflows is keyed by type_id.
type WorkitemWorkflow struct {
	TypeID          string              `json:"type_id,omitempty"`  // optional mirror of map key
	Name            string              `json:"name,omitempty"`     // type display name e.g. 产品类需求
	Category        string              `json:"category,omitempty"` // Req|Bug|Task
	WorkflowID      string              `json:"workflow_id,omitempty"`
	WorkflowName    string              `json:"workflow_name,omitempty"`
	DefaultStatusID string              `json:"default_status_id,omitempty"`
	Statuses        map[string]string   `json:"statuses,omitempty"` // displayName or alias → id
	Edges           map[string][]string `json:"edges,omitempty"`    // status id → []to ids
}

// WorkitemDefaultField is one field default captured from the type fields API.
type WorkitemDefaultField struct {
	Value     any    `json:"value"`
	Display   any    `json:"display,omitempty"` // option label, or []string for multi
	FieldName string `json:"field_name,omitempty"`
}

// WorkitemTypeDefaults holds create defaults / required field ids for one type_id.
type WorkitemTypeDefaults struct {
	Name           string                          `json:"name,omitempty"`
	Category       string                          `json:"category,omitempty"` // Req|Bug|Task|Risk|Topic|…
	Fields         map[string]WorkitemDefaultField `json:"fields,omitempty"`   // field id or system key → default
	CreateRequired []string                        `json:"create_required,omitempty"`
}

// Profile holds tenant-specific Projex constants (e.g. Zhiyi ZYPT space).
// One profile is project-scoped via space_id; per-type graphs live in Workflows.
type Profile struct {
	Name                  string                          `json:"name"`
	OrganizationID        string                          `json:"organization_id,omitempty"`
	AccessToken           string                          `json:"access_token,omitempty"` // optional PAT; see config token precedence
	SpaceID               string                          `json:"space_id,omitempty"`
	BugTypeID             string                          `json:"bug_type_id,omitempty"`
	BugStatuses           map[string]string               `json:"bug_statuses,omitempty"`
	BugEdges              map[string][]string             `json:"bug_edges,omitempty"`
	BugFields             map[string]string               `json:"bug_fields,omitempty"`
	BugCreateFields       BugCreateFields                 `json:"bug_create_fields,omitempty"`
	BugTransitionRequired map[string][]string             `json:"bug_transition_required,omitempty"`
	Workflows             map[string]WorkitemWorkflow     `json:"workflows,omitempty"`         // keyed by type_id
	WorkitemDefaults      map[string]WorkitemTypeDefaults `json:"workitem_defaults,omitempty"` // keyed by type_id
	Repositories          map[string]int64                `json:"repositories,omitempty"`
	AllowedEnvironments   []string                        `json:"allowed_environments,omitempty"`
	AllowedModules        []string                        `json:"allowed_modules,omitempty"`
	DefaultAssignedTo     string                          `json:"default_assigned_to,omitempty"`
	SerialPrefix          string                          `json:"serial_prefix,omitempty"`
}

// Dir returns ~/.config/yunxiao/profiles (same base as config.Dir).
func Dir() (string, error) {
	base, err := config.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, DirName), nil
}

// Path returns the JSON path for a named profile.
func Path(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("empty profile name")
	}
	d, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, name+".json"), nil
}

// Load reads ~/.config/yunxiao/profiles/<name>.json.
func Load(name string) (*Profile, error) {
	p, err := Path(name)
	if err != nil {
		return nil, err
	}
	return LoadFile(p)
}

// LoadFile reads a profile from an arbitrary path (tests / examples).
func LoadFile(path string) (*Profile, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("profile not found at %s (hint: yunxiao profile install-example zhiyi)", path)
		}
		return nil, err
	}
	var pf Profile
	if err := json.Unmarshal(b, &pf); err != nil {
		return nil, fmt.Errorf("invalid profile JSON %s: %w", path, err)
	}
	if pf.Name == "" {
		base := filepath.Base(path)
		pf.Name = strings.TrimSuffix(base, filepath.Ext(base))
	}
	if pf.BugStatuses == nil {
		pf.BugStatuses = map[string]string{}
	}
	if pf.BugEdges == nil {
		pf.BugEdges = map[string][]string{}
	}
	if pf.BugFields == nil {
		pf.BugFields = map[string]string{}
	}
	if pf.BugTransitionRequired == nil {
		pf.BugTransitionRequired = map[string][]string{}
	}
	if pf.Workflows == nil {
		pf.Workflows = map[string]WorkitemWorkflow{}
	}
	if pf.WorkitemDefaults == nil {
		pf.WorkitemDefaults = map[string]WorkitemTypeDefaults{}
	}
	if pf.Repositories == nil {
		pf.Repositories = map[string]int64{}
	}
	if pf.BugCreateFields.Priority == nil {
		pf.BugCreateFields.Priority = map[string]string{}
	}
	if pf.BugCreateFields.SeriousLevel == nil {
		pf.BugCreateFields.SeriousLevel = map[string]string{}
	}
	return &pf, nil
}

// ListNames returns profile basenames (without .json) under the profiles dir.
func ListNames() ([]string, string, error) {
	d, err := Dir()
	if err != nil {
		return nil, "", err
	}
	entries, err := os.ReadDir(d)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, d, nil
		}
		return nil, d, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".json") {
			names = append(names, strings.TrimSuffix(name, ".json"))
		}
	}
	return names, d, nil
}

// InstallExample copies srcExamplePath to profiles/<name>.json.
func InstallExample(name, srcExamplePath string, force bool) (string, error) {
	dst, err := Path(name)
	if err != nil {
		return "", err
	}
	if !force {
		if _, err := os.Stat(dst); err == nil {
			return "", fmt.Errorf("profile already exists at %s (use --force to overwrite)", dst)
		}
	}
	b, err := os.ReadFile(srcExamplePath)
	if err != nil {
		return "", fmt.Errorf("read example %s: %w", srcExamplePath, err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return "", err
	}
	if err := os.WriteFile(dst, b, 0o600); err != nil {
		return "", err
	}
	return dst, nil
}

// ResolveName returns explicit, else YUNXIAO_PROFILE env.
func ResolveName(explicit string) string {
	if strings.TrimSpace(explicit) != "" {
		return strings.TrimSpace(explicit)
	}
	return strings.TrimSpace(os.Getenv(EnvProfile))
}

// HintMissing is the standard error when a command needs a profile.
func HintMissing() error {
	return fmt.Errorf("no profile set: use --profile zhiyi or YUNXIAO_PROFILE=zhiyi; install with: yunxiao profile install-example zhiyi")
}

// FieldID returns bug_fields[key] or empty.
func (p *Profile) FieldID(key string) string {
	if p == nil || p.BugFields == nil {
		return ""
	}
	return p.BugFields[key]
}

// ModuleFieldID prefers bug_create_fields.module, else bug_fields.module.
func (p *Profile) ModuleFieldID() string {
	if p == nil {
		return ""
	}
	if p.BugCreateFields.Module != "" {
		return p.BugCreateFields.Module
	}
	return p.FieldID("module")
}

// EnvironmentFieldID prefers bug_create_fields.environment, else bug_fields.environment.
func (p *Profile) EnvironmentFieldID() string {
	if p == nil {
		return ""
	}
	if p.BugCreateFields.Environment != "" {
		return p.BugCreateFields.Environment
	}
	return p.FieldID("environment")
}

// ExpCompletionTimeKey prefers bug_create_fields, else bug_fields.
// Empty means the profile does not configure this create field (omit from payload).
func (p *Profile) ExpCompletionTimeKey() string {
	if p == nil {
		return ""
	}
	if p.BugCreateFields.ExpCompletionTime != "" {
		return p.BugCreateFields.ExpCompletionTime
	}
	return p.FieldID("ExpCompletionTime")
}

// Known priority aliases accepted by --priority when mapped in bug_create_fields.priority.
// Option ids are space-specific; aliases without a map entry must not be sent to the API.
var knownPriorityAliases = map[string]struct{}{
	"urgent": {}, "high": {}, "medium": {}, "low": {},
}

// Known serious-level aliases for --serious-level (includes synonyms used in docs/help).
var knownSeriousAliases = map[string]struct{}{
	"fatal": {}, "serious": {}, "severe": {}, "normal": {}, "slight": {}, "minor": {},
}

func isKnownPriorityAlias(v string) bool {
	_, ok := knownPriorityAliases[strings.ToLower(strings.TrimSpace(v))]
	return ok
}

func isKnownSeriousAlias(v string) bool {
	_, ok := knownSeriousAliases[strings.ToLower(strings.TrimSpace(v))]
	return ok
}

// ResolvePriorityID maps alias → option id.
// Profile maps win; raw option ids pass through; known aliases with no map entry error
// (avoids API 400 "字段【优先级】所填值无效").
func (p *Profile) ResolvePriorityID(aliasOrID string) (string, error) {
	aliasOrID = strings.TrimSpace(aliasOrID)
	if aliasOrID == "" {
		return "", fmt.Errorf("priority is empty")
	}
	key := strings.ToLower(aliasOrID)
	if p != nil && p.BugCreateFields.Priority != nil {
		if id, ok := p.BugCreateFields.Priority[aliasOrID]; ok && id != "" {
			return id, nil
		}
		if id, ok := p.BugCreateFields.Priority[key]; ok && id != "" {
			return id, nil
		}
	}
	if isKnownPriorityAlias(aliasOrID) {
		return "", fmt.Errorf("priority alias %q is not mapped to an option id; set profile bug_create_fields.priority[%q] (or pass the option id). Hint: yunxiao profile doctor", aliasOrID, key)
	}
	return aliasOrID, nil
}

// ResolveSeriousLevelID maps alias → option id (same rules as ResolvePriorityID).
func (p *Profile) ResolveSeriousLevelID(aliasOrID string) (string, error) {
	aliasOrID = strings.TrimSpace(aliasOrID)
	if aliasOrID == "" {
		return "", fmt.Errorf("serious-level is empty")
	}
	key := strings.ToLower(aliasOrID)
	if p != nil && p.BugCreateFields.SeriousLevel != nil {
		if id, ok := p.BugCreateFields.SeriousLevel[aliasOrID]; ok && id != "" {
			return id, nil
		}
		if id, ok := p.BugCreateFields.SeriousLevel[key]; ok && id != "" {
			return id, nil
		}
	}
	if isKnownSeriousAlias(aliasOrID) {
		return "", fmt.Errorf("serious-level alias %q is not mapped to an option id; set profile bug_create_fields.serious_level[%q] (or pass the option id). Hint: yunxiao profile doctor", aliasOrID, key)
	}
	return aliasOrID, nil
}

// AllowedContains reports whether value is in list. Empty list → true (backward compatible).
func AllowedContains(list []string, value string) bool {
	if len(list) == 0 {
		return true
	}
	for _, v := range list {
		if v == value {
			return true
		}
	}
	return false
}

// StatusGraph returns bug_edges, or derives them from bug_statuses aliases when empty.
func (p *Profile) StatusGraph() map[string][]string {
	if p == nil {
		return nil
	}
	if len(p.BugEdges) > 0 {
		return p.BugEdges
	}
	return DeriveBugEdges(p.BugStatuses)
}

// AllStatusIDs is the set of known bug status IDs from bug_statuses.
func (p *Profile) AllStatusIDs() map[string]bool {
	out := map[string]bool{}
	if p == nil {
		return out
	}
	for _, id := range p.BugStatuses {
		out[id] = true
	}
	return out
}

// defaultAliasEdges mirrors domain.ts BUG_EDGES (alias → aliases).
var defaultAliasEdges = map[string][]string{
	"confirm":      {"processing"},
	"processing":   {"deploy-test"},
	"deploy-test":  {"testing"},
	"testing":      {"fixed", "deploy-prod"},
	"fixed":        {"regression"},
	"regression":   {"closed-fixed"},
	"deploy-prod":  {"acceptance"},
	"acceptance":   {"closed-fixed"},
	"closed-fixed": {"reopen"},
	"reopen":       {"processing"},
}

// DeriveBugEdges builds status-id adjacency from alias edges + status map.
func DeriveBugEdges(statuses map[string]string) map[string][]string {
	out := map[string][]string{}
	for fromAlias, tos := range defaultAliasEdges {
		fromID, ok := statuses[fromAlias]
		if !ok || fromID == "" {
			continue
		}
		var next []string
		for _, toAlias := range tos {
			toID, ok := statuses[toAlias]
			if !ok || toID == "" {
				continue
			}
			next = append(next, toID)
		}
		if len(next) > 0 {
			out[fromID] = next
		}
	}
	return out
}

// Save writes the profile back to ~/.config/yunxiao/profiles/<name>.json (0600).
func (p *Profile) Save() (string, error) {
	if p == nil || strings.TrimSpace(p.Name) == "" {
		return "", fmt.Errorf("profile name required to save")
	}
	path, err := Path(p.Name)
	if err != nil {
		return "", err
	}
	if err := SaveFile(path, p); err != nil {
		return "", err
	}
	return path, nil
}

// SaveFile marshals profile JSON with indent to path (0600).
func SaveFile(path string, p *Profile) error {
	if p == nil {
		return fmt.Errorf("nil profile")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o600)
}

// MergeBugWorkflow merges discovered bug_edges and unambiguous bug_statuses aliases.
// Edges replace bug_edges entirely when non-empty. Status aliases are merged key-by-key
// (new aliases added/updated; existing keys not in snippet are kept).
func (p *Profile) MergeBugWorkflow(statuses map[string]string, edges map[string][]string) {
	if p == nil {
		return
	}
	if len(edges) > 0 {
		p.BugEdges = edges
	}
	if len(statuses) == 0 {
		return
	}
	if p.BugStatuses == nil {
		p.BugStatuses = map[string]string{}
	}
	for k, v := range statuses {
		if k == "" || v == "" {
			continue
		}
		p.BugStatuses[k] = v
	}
}

// MergeWorkflow upserts wf into Workflows[typeID]. Edges replace when non-empty;
// status name/alias maps merge key-by-key. Empty TypeID on wf is set to typeID.
func (p *Profile) MergeWorkflow(typeID string, wf WorkitemWorkflow) {
	if p == nil {
		return
	}
	typeID = strings.TrimSpace(typeID)
	if typeID == "" {
		return
	}
	if p.Workflows == nil {
		p.Workflows = map[string]WorkitemWorkflow{}
	}
	cur := p.Workflows[typeID]
	if wf.TypeID != "" {
		cur.TypeID = wf.TypeID
	} else if cur.TypeID == "" {
		cur.TypeID = typeID
	}
	if wf.Name != "" {
		cur.Name = wf.Name
	}
	if wf.Category != "" {
		cur.Category = wf.Category
	}
	if wf.WorkflowID != "" {
		cur.WorkflowID = wf.WorkflowID
	}
	if wf.WorkflowName != "" {
		cur.WorkflowName = wf.WorkflowName
	}
	if wf.DefaultStatusID != "" {
		cur.DefaultStatusID = wf.DefaultStatusID
	}
	if len(wf.Edges) > 0 {
		cur.Edges = wf.Edges
	}
	if len(wf.Statuses) > 0 {
		if cur.Statuses == nil {
			cur.Statuses = map[string]string{}
		}
		for k, v := range wf.Statuses {
			if k == "" || v == "" {
				continue
			}
			cur.Statuses[k] = v
		}
	}
	p.Workflows[typeID] = cur
}

// MergeWorkitemDefaults upserts defs into WorkitemDefaults[typeID].
// Fields replace when non-empty; create_required replaces when non-empty;
// name/category update when non-empty.
func (p *Profile) MergeWorkitemDefaults(typeID string, defs WorkitemTypeDefaults) {
	if p == nil {
		return
	}
	typeID = strings.TrimSpace(typeID)
	if typeID == "" {
		return
	}
	if p.WorkitemDefaults == nil {
		p.WorkitemDefaults = map[string]WorkitemTypeDefaults{}
	}
	cur := p.WorkitemDefaults[typeID]
	if defs.Name != "" {
		cur.Name = defs.Name
	}
	if defs.Category != "" {
		cur.Category = defs.Category
	}
	if len(defs.Fields) > 0 {
		cur.Fields = defs.Fields
	}
	if len(defs.CreateRequired) > 0 {
		cur.CreateRequired = defs.CreateRequired
	}
	p.WorkitemDefaults[typeID] = cur
}
