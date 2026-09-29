package profiles

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

func TestEmbeddedExamplesMatchRepoFiles(t *testing.T) {
	names := Names()
	for _, want := range []string{"play", "zhiyi"} {
		found := false
		for _, n := range names {
			if n == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("embedded examples %v missing %q", names, want)
		}
	}
	for _, name := range names {
		b, err := Example(name)
		if err != nil {
			t.Fatalf("Example(%q): %v", name, err)
		}
		disk, err := os.ReadFile(name + ExampleSuffix)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(b, disk) {
			t.Fatalf("embedded %s differs from %s%s", name, name, ExampleSuffix)
		}
		var doc map[string]any
		if err := json.Unmarshal(b, &doc); err != nil {
			t.Fatalf("%s: invalid JSON: %v", name, err)
		}
		if doc["name"] != name {
			t.Fatalf("%s: name=%v", name, doc["name"])
		}
		// Public examples must never carry a PAT.
		if tok, ok := doc["access_token"]; ok && tok != "" {
			t.Fatalf("%s: example must not contain access_token", name)
		}
	}
}

func TestExampleUnknown(t *testing.T) {
	if _, err := Example("does-not-exist"); err == nil {
		t.Fatal("expected error for unknown example")
	}
}
