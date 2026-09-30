package profiles

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
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

// #104: npm/profiles (prepack output, also kept in-tree for drift checks) must
// match repo-root profiles/*.example.json so publish never ships stale examples.
func TestNpmProfilesMatchRepoProfiles(t *testing.T) {
	npmDir := filepath.Join("..", "npm", "profiles")
	entries, err := os.ReadDir(npmDir)
	if err != nil {
		t.Fatalf("npm/profiles missing or unreadable (%v); run node npm/scripts/sync-profiles.js and commit the result", err)
	}
	var npmNames []string
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) == "" {
			continue
		}
		if len(e.Name()) < len(ExampleSuffix) || e.Name()[len(e.Name())-len(ExampleSuffix):] != ExampleSuffix {
			continue
		}
		npmNames = append(npmNames, e.Name()[:len(e.Name())-len(ExampleSuffix)])
	}
	repoNames := Names()
	if len(npmNames) != len(repoNames) {
		t.Fatalf("npm/profiles names %v != embedded/repo names %v", npmNames, repoNames)
	}
	for _, name := range repoNames {
		repo, err := os.ReadFile(name + ExampleSuffix)
		if err != nil {
			t.Fatal(err)
		}
		npm, err := os.ReadFile(filepath.Join(npmDir, name+ExampleSuffix))
		if err != nil {
			t.Fatalf("npm/profiles/%s%s: %v", name, ExampleSuffix, err)
		}
		if !bytes.Equal(repo, npm) {
			t.Fatalf("drift: profiles/%s%s != npm/profiles/%s%s — run node npm/scripts/sync-profiles.js", name, ExampleSuffix, name, ExampleSuffix)
		}
	}
}

func TestExampleUnknown(t *testing.T) {
	if _, err := Example("does-not-exist"); err == nil {
		t.Fatal("expected error for unknown example")
	}
}
