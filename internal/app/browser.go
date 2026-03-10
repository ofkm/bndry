package app

import (
	"fmt"
	"os/exec"
	"runtime"
)

func openBrowser(target string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", target)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	default:
		cmd = exec.Command("xdg-open", target)
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("open browser: %w", err)
	}

	return nil
}
