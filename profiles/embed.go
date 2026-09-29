// Package profiles embeds the example tenant profiles (*.example.json) into the
// yunxiao binary so `yunxiao profile install-example` works from npm / GitHub
// Release installs that ship only bin/yunxiao (+ skills/) without profiles/ (#92).
package profiles

import (
	"embed"
	"io/fs"
	"sort"
	"strings"
)

// ExampleSuffix is the filename suffix of shipped example profiles.
const ExampleSuffix = ".example.json"

//go:embed *.example.json
var examples embed.FS

// Example returns the embedded profiles/<name>.example.json content.
func Example(name string) ([]byte, error) {
	return examples.ReadFile(name + ExampleSuffix)
}

// Names lists embedded example names (without ExampleSuffix), sorted.
func Names() []string {
	entries, err := fs.ReadDir(examples, ".")
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ExampleSuffix) {
			continue
		}
		names = append(names, strings.TrimSuffix(e.Name(), ExampleSuffix))
	}
	sort.Strings(names)
	return names
}
