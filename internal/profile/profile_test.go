package profile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromTempDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	profDir := filepath.Join(dir, "yunxiao", "profiles")
	if err := os.MkdirAll(profDir, 0o700); err != nil {
		t.Fatal(err)
	}
	body := `{
  "name": "zhiyi",
  "organization_id": "67762490f72b227b2bf8327b",
  "space_id": "f24eb470873a90eb8a1ed994ba",
  "bug_type_id": "37da3a07df4d08aef2e3b393",
  "bug_statuses": {
    "confirm": "28",
    "processing": "100010",
    "deploy-test": "b2c5c60b4428974ef10483c800",
    "testing": "42d423acc3f60646a533bac489",
    "fixed": "29",
    "deferred": "34",
    "reopen": "30",
    "deploy-prod": "4b895bdc3433104d6bf73e4c17",
    "acceptance": "c0efc47304cc56508ac4314680",
    "regression": "2458e2d912449b731aa6797f70",
    "closed-fixed": "33",
    "wont-fix": "31",
    "cancelled-nofix": "03cf4f3d160e37aaa13a6c27c8",
    "cancelled-wontfix": "30bdbe7fc10889cab222915375",
    "closed-unfixed": "013a823767244591639ea5a7"
  },
  "bug_fields": {
    "plan_due_date": "80",
    "developer": "171ea64c4a9b51542c960a6d7c",
    "responsible_person": "4086ed55f8852e203dcf3d0042",
    "bug_reason": "51fdf5114acb39746e4ad08677",
    "bug_impact_scope": "6a32dfdc0288052b44f2b004b2"
  },
  "bug_transition_required": {
    "100010": ["80", "171ea64c4a9b51542c960a6d7c"],
    "b2c5c60b4428974ef10483c800": ["4086ed55f8852e203dcf3d0042", "51fdf5114acb39746e4ad08677", "6a32dfdc0288052b44f2b004b2"],
    "42d423acc3f60646a533bac489": ["80", "171ea64c4a9b51542c960a6d7c"]
  },
  "serial_prefix": "ZYPT"
}`
	if err := os.WriteFile(filepath.Join(profDir, "zhiyi.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	p, err := Load("zhiyi")
	if err != nil {
		t.Fatal(err)
	}
	if p.OrganizationID != "67762490f72b227b2bf8327b" {
		t.Fatalf("org=%s", p.OrganizationID)
	}
	if p.BugFields["developer"] != "171ea64c4a9b51542c960a6d7c" {
		t.Fatalf("developer id wrong: %s", p.BugFields["developer"])
	}
	if len(p.BugStatuses) != 15 {
		t.Fatalf("want 15 statuses, got %d", len(p.BugStatuses))
	}
	edges := p.StatusGraph()
	if len(edges["28"]) != 1 || edges["28"][0] != "100010" {
		t.Fatalf("derived edges confirm: %v", edges["28"])
	}
	names, d, err := ListNames()
	if err != nil {
		t.Fatal(err)
	}
	if d == "" || len(names) != 1 || names[0] != "zhiyi" {
		t.Fatalf("list=%v dir=%s", names, d)
	}
}

func TestDeriveBugEdgesMatchesExample(t *testing.T) {
	st := map[string]string{
		"confirm": "28", "processing": "100010",
		"deploy-test": "b2c5c60b4428974ef10483c800",
		"testing":     "42d423acc3f60646a533bac489",
		"fixed":       "29", "deploy-prod": "4b895bdc3433104d6bf73e4c17",
		"regression": "2458e2d912449b731aa6797f70", "closed-fixed": "33",
		"acceptance": "c0efc47304cc56508ac4314680", "reopen": "30",
	}
	edges := DeriveBugEdges(st)
	if got := edges["42d423acc3f60646a533bac489"]; len(got) != 2 {
		t.Fatalf("testing forks: %v", got)
	}
}

func TestInstallExample(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	src := filepath.Join(dir, "zhiyi.example.json")
	if err := os.WriteFile(src, []byte(`{"name":"zhiyi","serial_prefix":"ZYPT"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	dst, err := InstallExample("zhiyi", src, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dst); err != nil {
		t.Fatal(err)
	}
	if _, err := InstallExample("zhiyi", src, false); err == nil {
		t.Fatal("expected exists error")
	}
	if _, err := InstallExample("zhiyi", src, true); err != nil {
		t.Fatal(err)
	}
}

func TestResolveName(t *testing.T) {
	t.Setenv(EnvProfile, "from-env")
	if ResolveName("") != "from-env" {
		t.Fatal(ResolveName(""))
	}
	if ResolveName("flag") != "flag" {
		t.Fatal(ResolveName("flag"))
	}
}

func TestLoadExampleFileWave2(t *testing.T) {
	// Load shipped example relative to module (repo checkout).
	candidates := []string{
		filepath.Join("..", "..", "profiles", "zhiyi.example.json"),
	}
	var path string
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			path = c
			break
		}
	}
	if path == "" {
		t.Skip("example file not found from test cwd")
	}
	p, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := p.Repositories["iipmes_gy"]; !ok {
		t.Fatalf("repos missing iipmes_gy alias: %v", p.Repositories)
	}
	if p.OrganizationID != "<your-org-id>" || p.SpaceID != "<your-space-id>" {
		t.Fatalf("example must use placeholders, got org=%q space=%q", p.OrganizationID, p.SpaceID)
	}
	if p.BugTypeID != "<bug-type-id>" {
		t.Fatalf("bug_type_id=%q", p.BugTypeID)
	}
	if p.BugCreateFields.Priority["high"] == "" {
		t.Fatal("missing priority high")
	}
	if len(p.AllowedEnvironments) != 2 || p.DefaultAssignedTo == "" {
		t.Fatalf("allowed/default missing: %+v", p)
	}
	if p.ModuleFieldID() == "" || p.EnvironmentFieldID() == "" {
		t.Fatal("module/env field id")
	}
	if p.SerialPrefix != "ZYPT" {
		t.Fatalf("serial_prefix=%q", p.SerialPrefix)
	}
}

func TestMergeWorkflow(t *testing.T) {
	p := &Profile{Name: "play", BugTypeID: "bug-1"}
	p.MergeWorkflow("type-a", WorkitemWorkflow{
		Name:            "产品类需求",
		Category:        "Req",
		WorkflowID:      "wf-a",
		WorkflowName:    "默认需求工作流",
		DefaultStatusID: "100005",
		Statuses:        map[string]string{"待处理": "100005", "已完成": "100014"},
		Edges:           map[string][]string{"100005": {"100014"}},
	})
	wf, ok := p.Workflows["type-a"]
	if !ok {
		t.Fatal("missing workflows[type-a]")
	}
	if wf.TypeID != "type-a" || wf.Name != "产品类需求" || wf.Statuses["待处理"] != "100005" {
		t.Fatalf("%+v", wf)
	}
	if len(wf.Edges["100005"]) != 1 {
		t.Fatalf("edges=%v", wf.Edges)
	}

	// Upsert: merge statuses, replace edges, keep prior name if new empty.
	p.MergeWorkflow("type-a", WorkitemWorkflow{
		Statuses: map[string]string{"已取消": "141230"},
		Edges:    map[string][]string{"100005": {"100014", "141230"}},
	})
	wf = p.Workflows["type-a"]
	if wf.Name != "产品类需求" {
		t.Fatalf("name clobbered: %s", wf.Name)
	}
	if wf.Statuses["已取消"] != "141230" || wf.Statuses["待处理"] != "100005" {
		t.Fatalf("statuses merge: %v", wf.Statuses)
	}
	if len(wf.Edges["100005"]) != 2 {
		t.Fatalf("edges replace: %v", wf.Edges)
	}

	// Empty typeID is a no-op.
	p.MergeWorkflow("", WorkitemWorkflow{Name: "x"})
	if _, ok := p.Workflows[""]; ok {
		t.Fatal("empty key should not be inserted")
	}
}

func TestMergeBugWorkflow(t *testing.T) {
	p := &Profile{Name: "play", BugStatuses: map[string]string{"confirm": "28"}}
	p.MergeBugWorkflow(map[string]string{"processing": "100010"}, map[string][]string{"28": {"100010"}})
	if p.BugStatuses["confirm"] != "28" || p.BugStatuses["processing"] != "100010" {
		t.Fatalf("%v", p.BugStatuses)
	}
	if len(p.BugEdges["28"]) != 1 {
		t.Fatalf("%v", p.BugEdges)
	}
}

func TestMergeWorkitemDefaults(t *testing.T) {
	p := &Profile{Name: "t"}
	p.MergeWorkitemDefaults("type-a", WorkitemTypeDefaults{
		Name:     "产品类需求",
		Category: "Req",
		Fields: map[string]WorkitemDefaultField{
			"priority": {Value: "f008", Display: "中", FieldName: "优先级"},
		},
		CreateRequired: []string{"assignedTo", "priority"},
	})
	got := p.WorkitemDefaults["type-a"]
	if got.Name != "产品类需求" || got.Category != "Req" {
		t.Fatalf("meta: %+v", got)
	}
	if got.Fields["priority"].Display != "中" {
		t.Fatalf("priority display: %+v", got.Fields["priority"])
	}
	p.MergeWorkitemDefaults("type-a", WorkitemTypeDefaults{
		Fields: map[string]WorkitemDefaultField{
			"priority": {Value: "0751", Display: "高"},
		},
	})
	if p.WorkitemDefaults["type-a"].Fields["priority"].Value != "0751" {
		t.Fatalf("fields should replace: %+v", p.WorkitemDefaults["type-a"].Fields)
	}
	if len(p.WorkitemDefaults["type-a"].CreateRequired) != 2 {
		t.Fatalf("create_required kept when empty on merge: %v", p.WorkitemDefaults["type-a"].CreateRequired)
	}
	p.MergeWorkitemDefaults("", WorkitemTypeDefaults{Name: "x"})
	if _, ok := p.WorkitemDefaults[""]; ok {
		t.Fatal("empty type id should be ignored")
	}
}

func TestLoadWorkitemDefaults(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	profDir := filepath.Join(dir, "yunxiao", "profiles")
	if err := os.MkdirAll(profDir, 0o700); err != nil {
		t.Fatal(err)
	}
	body := `{
  "name": "play",
  "space_id": "abc",
  "workitem_defaults": {
    "9uy29901re573f561d69jn40": {
      "name": "产品类需求",
      "category": "Req",
      "fields": {
        "priority": {"value": "f008f88b4282790a7529a05bcf", "display": "中", "field_name": "优先级"}
      },
      "create_required": ["assignedTo", "priority"]
    }
  }
}`
	if err := os.WriteFile(filepath.Join(profDir, "play.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := Load("play")
	if err != nil {
		t.Fatal(err)
	}
	d := p.WorkitemDefaults["9uy29901re573f561d69jn40"]
	if d.Name != "产品类需求" || d.Fields["priority"].Display != "中" {
		t.Fatalf("loaded defaults: %+v", d)
	}
	path, err := p.Save()
	if err != nil {
		t.Fatal(err)
	}
	p2, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if p2.WorkitemDefaults["9uy29901re573f561d69jn40"].Fields["priority"].Value != "f008f88b4282790a7529a05bcf" {
		t.Fatalf("roundtrip: %+v", p2.WorkitemDefaults)
	}
}


func TestMergeWorkflowHintedEdges(t *testing.T) {
	p := &Profile{Name: "play"}
	p.MergeWorkflow("type-a", WorkitemWorkflow{
		Edges:       map[string][]string{"a": {"b"}},
		HintedEdges: map[string][]string{"a": {"c"}},
	})
	wf := p.Workflows["type-a"]
	if len(wf.HintedEdges["a"]) != 1 || wf.HintedEdges["a"][0] != "c" {
		t.Fatalf("hinted: %v", wf.HintedEdges)
	}
	// nil HintedEdges must not clear
	p.MergeWorkflow("type-a", WorkitemWorkflow{Edges: map[string][]string{"a": {"b", "d"}}})
	wf = p.Workflows["type-a"]
	if len(wf.HintedEdges["a"]) != 1 {
		t.Fatalf("hinted cleared unexpectedly: %v", wf.HintedEdges)
	}
	// empty non-nil clears
	p.MergeWorkflow("type-a", WorkitemWorkflow{HintedEdges: map[string][]string{}})
	wf = p.Workflows["type-a"]
	if len(wf.HintedEdges) != 0 {
		t.Fatalf("expected clear: %v", wf.HintedEdges)
	}
}

func TestMergeWorkflowEdgesUnion(t *testing.T) {
	p := &Profile{Name: "t"}
	p.MergeWorkflow("type-a", WorkitemWorkflow{
		Edges: map[string][]string{"st-a": {"st-b"}, "st-deploy": {"st-regress"}},
	})
	p.MergeWorkflow("type-a", WorkitemWorkflow{
		Edges: map[string][]string{"st-a": {"st-c"}, "st-b": {"st-d"}},
	})
	wf := p.Workflows["type-a"]
	if len(wf.Edges["st-a"]) != 2 {
		t.Fatalf("st-a should union: %v", wf.Edges["st-a"])
	}
	if len(wf.Edges["st-deploy"]) != 1 || wf.Edges["st-deploy"][0] != "st-regress" {
		t.Fatalf("manual edge dropped: %v", wf.Edges)
	}
	if len(wf.Edges["st-b"]) != 1 {
		t.Fatalf("new edge missing: %v", wf.Edges)
	}
}
