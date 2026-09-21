package cmd

import (
	"fmt"
	"strings"

	"github.com/yunxiao-cli/yunxiao/internal/client"
)

// withWriteDedupeHint appends a ready-made search command when a create/update
// failed at the network layer (server may already have applied it) — #47.
func withWriteDedupeHint(err error, hintCmd string) error {
	if err == nil || !client.IsTransientNetworkError(err) {
		return err
	}
	hintCmd = strings.TrimSpace(hintCmd)
	if hintCmd == "" {
		return client.AnnotateWriteNetworkError(err, "POST", "")
	}
	return client.AnnotateWriteNetworkError(err, "POST", hintCmd)
}

func mrsListSearchHint(repo, title string) string {
	repo = strings.TrimSpace(repo)
	title = strings.TrimSpace(title)
	if repo == "" {
		return "yunxiao codeup mrs list --search <title>"
	}
	if title == "" {
		return fmt.Sprintf("yunxiao codeup mrs list --repo %s --search <title>", repo)
	}
	// shell-escape title lightly for display
	safe := strings.ReplaceAll(title, `"`, `'`)
	return fmt.Sprintf(`yunxiao codeup mrs list --repo %s --search "%s"`, repo, safe)
}

func workitemSearchHint(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return "yunxiao workitem search --category Bug --subject <title>"
	}
	safe := strings.ReplaceAll(title, `"`, `'`)
	return fmt.Sprintf(`yunxiao workitem search --category Bug --subject "%s"`, safe)
}
