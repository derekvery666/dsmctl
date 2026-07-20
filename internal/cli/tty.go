package cli

import "os"

// stdinIsInteractive reports whether standard input is an interactive terminal
// (a character device) rather than a pipe, redirect, file, or the null device.
//
// It is the gate that lets a human at a terminal reveal a stored password while
// refusing every non-interactive caller — shell pipelines, CI, and AI coding
// agents, whose subprocess stdin is never a controlling terminal. This is the
// structural reason the model can never obtain a revealed password: even if it
// ran `dsmctl auth reveal-password`, its stdin is not a TTY and the command
// refuses before reading the credential store.
func stdinIsInteractive() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
