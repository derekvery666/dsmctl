// Package clipboard copies short-lived secrets to and from the OS clipboard
// using the platform's native clipboard tool. The secret is always passed on
// the child process's standard input, never as a command-line argument, so it
// never appears in the process list.
package clipboard

import (
	"context"
	"errors"
	"os/exec"
	"runtime"
	"strings"
)

// Copy writes data to the OS clipboard.
func Copy(ctx context.Context, data string) error {
	name, args, err := copyCommand()
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = strings.NewReader(data)
	return cmd.Run()
}

// Read returns the current clipboard contents.
func Read(ctx context.Context) (string, error) {
	name, args, err := pasteCommand()
	if err != nil {
		return "", err
	}
	out, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// ClearIfUnchanged clears the clipboard, but only if it still holds wrote, so a
// value the user copied in the meantime is left untouched. If the clipboard
// cannot be read back it is cleared anyway, so a secret is never left behind.
func ClearIfUnchanged(ctx context.Context, wrote string) error {
	current, err := Read(ctx)
	if err != nil {
		return Copy(ctx, "")
	}
	if trimEOL(current) != trimEOL(wrote) {
		return nil
	}
	return Copy(ctx, "")
}

func trimEOL(s string) string { return strings.TrimRight(s, "\r\n") }

func copyCommand() (string, []string, error) {
	switch runtime.GOOS {
	case "windows":
		return "clip", nil, nil
	case "darwin":
		return "pbcopy", nil, nil
	default:
		if path, err := exec.LookPath("wl-copy"); err == nil {
			return path, nil, nil
		}
		if path, err := exec.LookPath("xclip"); err == nil {
			return path, []string{"-selection", "clipboard"}, nil
		}
		if path, err := exec.LookPath("xsel"); err == nil {
			return path, []string{"--clipboard", "--input"}, nil
		}
		return "", nil, errors.New("no clipboard tool found (install wl-clipboard, xclip, or xsel)")
	}
}

func pasteCommand() (string, []string, error) {
	switch runtime.GOOS {
	case "windows":
		return "powershell", []string{"-NoProfile", "-Command", "Get-Clipboard"}, nil
	case "darwin":
		return "pbpaste", nil, nil
	default:
		if path, err := exec.LookPath("wl-paste"); err == nil {
			return path, nil, nil
		}
		if path, err := exec.LookPath("xclip"); err == nil {
			return path, []string{"-selection", "clipboard", "-o"}, nil
		}
		if path, err := exec.LookPath("xsel"); err == nil {
			return path, []string{"--clipboard", "--output"}, nil
		}
		return "", nil, errors.New("no clipboard tool found (install wl-clipboard, xclip, or xsel)")
	}
}
