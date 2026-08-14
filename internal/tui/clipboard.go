package tui

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const maxClipboardBytes = 8 << 20

var clipboardTimeout = 2 * time.Second

var errClipboardTooLarge = errors.New("clipboard content exceeds limit")

// readClipboard returns the current system clipboard contents as a best-effort
// string. It shells out to the platform clipboard tool and returns "" when none
// is available or the read fails, so callers can treat it as optional input.
func readClipboard() string {
	return readClipboardFrom(clipboardCandidates(runtime.GOOS))
}

func clipboardCandidates(goos string) [][]string {
	var candidates [][]string
	switch goos {
	case "darwin":
		candidates = [][]string{{"pbpaste"}}
	case "windows":
		candidates = [][]string{
			{"powershell.exe", "-NoProfile", "-NonInteractive", "-Command", "Get-Clipboard -Raw"},
			{"pwsh.exe", "-NoProfile", "-NonInteractive", "-Command", "Get-Clipboard -Raw"},
		}
	default:
		candidates = [][]string{
			{"wl-paste", "--no-newline"},
			{"xclip", "-selection", "clipboard", "-o"},
			{"xsel", "--clipboard", "--output"},
			// WSL can access the Windows clipboard through interop even when no
			// Wayland/X11 clipboard helper is installed.
			{"powershell.exe", "-NoProfile", "-NonInteractive", "-Command", "Get-Clipboard -Raw"},
		}
	}
	return candidates
}

func readClipboardFrom(candidates [][]string) string {
	for _, args := range candidates {
		path, err := exec.LookPath(args[0])
		if err != nil {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), clipboardTimeout)
		output := &limitedWriter{remaining: maxClipboardBytes}
		command := exec.CommandContext(ctx, path, args[1:]...)
		command.Stdout = output
		command.Stderr = io.Discard
		err = command.Run()
		cancel()
		if err != nil {
			continue
		}
		return strings.TrimRight(output.builder.String(), "\r\n")
	}
	return ""
}

type limitedWriter struct {
	builder   strings.Builder
	remaining int
}

func (w *limitedWriter) Write(data []byte) (int, error) {
	if len(data) > w.remaining {
		written := 0
		if w.remaining > 0 {
			written, _ = w.builder.Write(data[:w.remaining])
			w.remaining = 0
		}
		return written, errClipboardTooLarge
	}
	w.remaining -= len(data)
	return w.builder.Write(data)
}
