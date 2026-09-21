package alias

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/yunxiao-cli/yunxiao/internal/config"
)

const fileName = "aliases.json"

// Store is the on-disk alias map: name -> argv tokens (without "yunxiao").
type Store map[string][]string

// Path returns ~/.config/yunxiao/aliases.json (same dir as config.json).
func Path() (string, error) {
	d, err := config.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, fileName), nil
}

// Load reads the alias file; missing file => empty store.
func Load() (Store, error) {
	p, err := Path()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return Store{}, nil
		}
		return nil, err
	}
	var s Store
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("parse aliases: %w", err)
	}
	if s == nil {
		s = Store{}
	}
	return s, nil
}

// Save writes the alias file with 0600.
func Save(s Store) error {
	p, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(p, b, 0o600)
}

// ValidateName rejects empty, reserved, or non-token names.
func ValidateName(name string, reserved map[string]bool) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("alias name is required")
	}
	if strings.HasPrefix(name, "-") {
		return fmt.Errorf("alias name must not start with -")
	}
	for _, r := range name {
		if unicode.IsSpace(r) {
			return fmt.Errorf("alias name must not contain spaces")
		}
	}
	if reserved[name] {
		return fmt.Errorf("alias name %q conflicts with a built-in command", name)
	}
	if name == "alias" {
		return fmt.Errorf("alias name cannot be %q", name)
	}
	return nil
}

// ValidateExpansion rejects empty expansion and embedded --yes/-y (must not bypass high-risk gate).
func ValidateExpansion(tokens []string) error {
	if len(tokens) == 0 {
		return fmt.Errorf("alias expansion is empty")
	}
	for _, t := range tokens {
		switch t {
		case "--yes", "-y":
			return fmt.Errorf("alias must not embed %s; pass it on the command line after confirmation", t)
		}
	}
	return nil
}

// ExpandArgs rewrites argv (os.Args style: [prog, ...]) if args[1] is an alias.
// Returns (newArgs, expandedName, ok).
// Global flags before the alias are preserved; the alias name is replaced by its tokens.
func ExpandArgs(args []string, s Store, reserved map[string]bool) ([]string, string, bool) {
	if len(args) < 2 || s == nil {
		return args, "", false
	}
	// Find first non-flag token after prog — that is the command / alias candidate.
	idx := -1
	for i := 1; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			if i+1 < len(args) {
				idx = i + 1
			}
			break
		}
		if strings.HasPrefix(a, "-") {
			// skip flag and optional value for --flag=val already one token;
			// for --flag value we skip next if it does not look like a command — keep simple:
			// only treat bare tokens as commands; known global flags take a value.
			if isGlobalFlagTakingValue(a) && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
			}
			continue
		}
		idx = i
		break
	}
	if idx < 0 {
		return args, "", false
	}
	name := args[idx]
	if reserved[name] {
		return args, "", false
	}
	exp, ok := s[name]
	if !ok || len(exp) == 0 {
		return args, "", false
	}
	out := append([]string{}, args[:idx]...)
	out = append(out, exp...)
	out = append(out, args[idx+1:]...)
	return out, name, true
}

func isGlobalFlagTakingValue(flag string) bool {
	switch flag {
	case "--format", "--jq", "--organization-id", "--profile":
		return true
	default:
		if strings.HasPrefix(flag, "--format=") || strings.HasPrefix(flag, "--jq=") ||
			strings.HasPrefix(flag, "--organization-id=") || strings.HasPrefix(flag, "--profile=") {
			return false
		}
		return false
	}
}
