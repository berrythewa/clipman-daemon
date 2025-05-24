//go:build linux
// +build linux

package platform

import (
	"fmt"
	"os/exec"
	"strings"
)

// X11 clipboard selection types
const (
	clipboardPrimary   = "primary"
	clipboardClipboard = "clipboard"
)

// getX11Content attempts to read clipboard content using xclip
func getX11Content(selection string) (string, error) {
	cmd := exec.Command("xclip", "-selection", selection, "-o")
	output, err := cmd.Output()
	if err != nil {
		if strings.Contains(err.Error(), "exit status 1") {
			// Empty clipboard
			return "", nil
		}
		return "", fmt.Errorf("failed to read X11 clipboard: %w", err)
	}
	return string(output), nil
}

// setX11Content attempts to write clipboard content using xclip
func setX11Content(selection string, content string) error {
	cmd := exec.Command("xclip", "-selection", selection, "-i")
	cmd.Stdin = strings.NewReader(content)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to write X11 clipboard: %w", err)
	}
	return nil
}

// isX11Available checks if X11 clipboard tools are available
func isX11Available() bool {
	_, err := exec.LookPath("xclip")
	return err == nil
} 