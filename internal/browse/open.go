package browse

import (
	"fmt"
	"os/exec"
	"runtime"
)

// OpenSystemBrowser opens url with the OS default handler.
func OpenSystemBrowser(rawURL string) error {
	if err := AssertHTTPURL(rawURL); err != nil {
		return err
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL)
	case "darwin":
		cmd = exec.Command("open", rawURL)
	default:
		cmd = exec.Command("xdg-open", rawURL)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open browser: %w", err)
	}
	return nil
}
