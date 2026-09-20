package pipelineyaml

import "testing"

func TestDiffYAML_addRemoveModify(t *testing.T) {
	oldY := `
sources: {}
stages:
  - name: build
    jobs:
      - name: compile
        steps:
          - name: go-build
            run: go build
  - name: deploy
    jobs:
      - name: ship
        steps:
          - name: kubectl
            plugin: kubectl
`
	newY := `
sources: {}
stages:
  - name: build
    jobs:
      - name: compile
        steps:
          - name: go-build
            run: go build ./cmd
  - name: test
    jobs:
      - name: unit
        steps:
          - name: go-test
            run: go test ./...
`
	d, err := DiffYAML(oldY, newY)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Removed) == 0 {
		t.Fatalf("expected removed deploy, got %#v", d)
	}
	if !d.HighRisk {
		t.Fatal("expected high risk due to removed stage")
	}
	if len(d.Added) == 0 {
		t.Fatalf("expected added test stage: %#v", d)
	}
}

func TestDiffYAML_empty(t *testing.T) {
	d, err := DiffYAML("", "stages:\n  - name: a\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Added) == 0 {
		t.Fatal(d)
	}
}

func TestDiffYAML_renameIsRemoveAdd(t *testing.T) {
	oldY := "stages:\n  - name: build\n"
	newY := "stages:\n  - name: compile\n"
	d, err := DiffYAML(oldY, newY)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Removed) == 0 || len(d.Added) == 0 {
		t.Fatalf("rename should be remove+add: %#v", d)
	}
	if !d.HighRisk {
		t.Fatal("expected high risk on remove")
	}
}
