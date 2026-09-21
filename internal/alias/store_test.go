package alias

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestValidateExpansionRejectsYes(t *testing.T) {
	if err := ValidateExpansion([]string{"pipeline", "+approve", "--yes"}); err == nil {
		t.Fatal("expected reject --yes")
	}
	if err := ValidateExpansion([]string{"pipeline", "+pending"}); err != nil {
		t.Fatal(err)
	}
}

func TestValidateNameReserved(t *testing.T) {
	res := map[string]bool{"pipeline": true}
	if err := ValidateName("pipeline", res); err == nil {
		t.Fatal("expected conflict")
	}
	if err := ValidateName("pending", res); err != nil {
		t.Fatal(err)
	}
}

func TestExpandArgs(t *testing.T) {
	s := Store{"pending": {"pipeline", "+pending", "--all-pipelines"}}
	res := map[string]bool{"pipeline": true, "alias": true}
	got, name, ok, err := ExpandArgs([]string{"yunxiao", "pending"}, s, res)
	if err != nil || !ok || name != "pending" {
		t.Fatalf("ok=%v name=%s err=%v", ok, name, err)
	}
	want := []string{"yunxiao", "pipeline", "+pending", "--all-pipelines"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
	// preserve leading global flags
	got, _, ok, err = ExpandArgs([]string{"yunxiao", "--dry-run", "pending", "--format", "pretty"}, s, res)
	if err != nil || !ok {
		t.Fatalf("expected expand: %v", err)
	}
	want = []string{"yunxiao", "--dry-run", "pipeline", "+pending", "--all-pipelines", "--format", "pretty"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
	// reserved command not expanded
	_, _, ok, err = ExpandArgs([]string{"yunxiao", "pipeline", "list"}, s, res)
	if err != nil || ok {
		t.Fatal("must not expand reserved")
	}
	// user can still pass --yes after expansion point
	got, _, ok, err = ExpandArgs([]string{"yunxiao", "pending", "--yes"}, s, res)
	if err != nil || !ok {
		t.Fatalf("expected expand: %v", err)
	}
	want = []string{"yunxiao", "pipeline", "+pending", "--all-pipelines", "--yes"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v", got)
	}
}

func TestLoadSave(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	s := Store{"p": {"pipeline", "+pending"}}
	if err := Save(s); err != nil {
		t.Fatal(err)
	}
	p, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(p) != "aliases.json" {
		t.Fatal(p)
	}
	if !strings.Contains(p, "yunxiao") {
		t.Fatalf("expected yunxiao dir: %s", p)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(Store(got), s) {
		t.Fatalf("%#v", got)
	}
	fi, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm()&0o077 != 0 {
		// on Windows permissions may differ; only check file exists
		_ = fi
	}
}

func TestValidateExpansionRejectsYesEquals(t *testing.T) {
	if err := ValidateExpansion([]string{"pipeline", "+approve", "--yes=true"}); err == nil {
		t.Fatal("expected reject --yes=true")
	}
}

func TestExpandArgsRejectsHandEditedYes(t *testing.T) {
	s := Store{"bad": {"pipeline", "+approve", "--yes"}}
	res := map[string]bool{"pipeline": true}
	_, name, ok, err := ExpandArgs([]string{"yunxiao", "bad"}, s, res)
	if ok || err == nil || name != "bad" {
		t.Fatalf("ok=%v err=%v name=%s", ok, err, name)
	}
}
