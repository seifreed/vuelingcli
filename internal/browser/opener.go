package browser

import (
	"fmt"
	"os/exec"
	"runtime"
)

// OpenURL opens a URL in the host system's default browser.
func OpenURL(link string) error {
	return openURL(runtime.GOOS, link)
}

func openURL(goos, link string) error {
	cmd, err := openCommand(goos, link)
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

func openCommand(goos, link string) (*exec.Cmd, error) {
	var cmd *exec.Cmd
	switch goos {
	case "darwin":
		cmd = exec.Command("open", link)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", link)
	case "linux":
		cmd = exec.Command("xdg-open", link)
	default:
		return nil, fmt.Errorf("opening links is not supported on %s", goos)
	}
	return cmd, nil
}
