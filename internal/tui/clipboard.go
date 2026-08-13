package tui

import (
	"os/exec"
	"runtime"
	"strings"
)

// readClipboard returns the current system clipboard contents as a best-effort
// string. It shells out to the platform clipboard tool and returns "" when none
// is available or the read fails, so callers can treat it as optional input.
func readClipboard() string {
	var candidates [][]string
	switch runtime.GOOS {
	case "darwin":
		candidates = [][]string{{"pbpaste"}}
	case "windows":
		candidates = [][]string{{"powershell", "-NoProfile", "-Command", "Get-Clipboard"}}
	default:
		candidates = [][]string{
			{"wl-paste", "--no-newline"},
			{"xclip", "-selection", "clipboard", "-o"},
			{"xsel", "--clipboard", "--output"},
		}
	}
	for _, args := range candidates {
		path, err := exec.LookPath(args[0])
		if err != nil {
			continue
		}
		out, err := exec.Command(path, args[1:]...).Output()
		if err != nil {
			continue
		}
		return strings.TrimRight(string(out), "\r\n")
	}
	return ""
}
