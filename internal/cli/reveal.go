package cli

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/derekvery666/dsmctl/internal/clipboard"
	"github.com/derekvery666/dsmctl/internal/config"
	"github.com/derekvery666/dsmctl/internal/credentials"
)

func newAuthRevealPasswordCommand(opts *options) *cobra.Command {
	var toStdout bool
	var clearAfter int
	var account string
	command := &cobra.Command{
		Use:   "reveal-password",
		Short: "Copy a NAS's stored password to the clipboard (a human at a terminal only)",
		Long: "reveal-password retrieves the password dsmctl stored for a NAS and copies it to the clipboard\n" +
			"(auto-cleared after a delay). It is gated so only a human at the keyboard can run it: standard\n" +
			"input must be an interactive terminal AND the operator must type the NAS name to confirm. A pipe\n" +
			"fails the terminal check; a non-interactive caller with no operator reads end-of-input at the\n" +
			"prompt and is refused. This command is CLI-only; no MCP tool ever returns a password.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			// Gate 1: standard input must be an interactive terminal. This refuses
			// pipes and redirects (`echo x | ...`, `... < file`).
			if !stdinIsInteractive() {
				auditReveal(cmd, opts.nas, "refused", "stdin-not-a-terminal")
				return errors.New("reveal-password must be run by a human at an interactive terminal; it refuses pipes, redirects, and non-interactive callers")
			}
			cfg, err := config.NewStore(opts.configPath).Load()
			if err != nil {
				return err
			}
			name, _, err := cfg.Resolve(opts.nas)
			if err != nil {
				return err
			}

			// Gate 2: the operator must type the NAS name. A caller that passes
			// gate 1 on a pseudo-terminal but has no human (an agent, CI) reads
			// end-of-input here and is refused; a caller that pipes the answer to
			// satisfy this fails gate 1. Only a person at the keyboard passes both.
			fmt.Fprintf(cmd.ErrOrStderr(), "Reveal the password for NAS %q. Type the NAS name to confirm: ", name)
			var typed string
			if _, err := fmt.Fscanln(cmd.InOrStdin(), &typed); err != nil {
				auditReveal(cmd, name, "refused", "no-typed-confirmation")
				return errors.New("no confirmation was read from the terminal; nothing was revealed")
			}
			if typed != name {
				auditReveal(cmd, name, "refused", "confirmation-mismatch")
				return errors.New("confirmation did not match the NAS name; nothing was revealed")
			}

			secrets := credentials.NewSecureStore()
			password, err := secrets.RevealPasswordForAccount(ctx, name, account)
			if err != nil {
				if errors.Is(err, credentials.ErrNoStoredPassword) {
					auditReveal(cmd, name, "none", "no-stored-password")
					return fmt.Errorf("no password is stored for NAS %q in the OS credential store", name)
				}
				return err
			}

			if toStdout {
				fmt.Fprintln(cmd.OutOrStdout(), password)
				auditReveal(cmd, name, "stdout", "revealed")
				return nil
			}

			if clearAfter < 5 {
				clearAfter = 5
			}
			if err := clipboard.Copy(ctx, password); err != nil {
				auditReveal(cmd, name, "clipboard", "copy-failed")
				return fmt.Errorf("copy to clipboard failed: %w", err)
			}
			auditReveal(cmd, name, "clipboard", "revealed")
			fmt.Fprintf(cmd.ErrOrStderr(), "Password for NAS %q copied to the clipboard; it clears in %d seconds. Keep this command running.\n", name, clearAfter)
			select {
			case <-time.After(time.Duration(clearAfter) * time.Second):
			case <-ctx.Done():
			}
			// Use a fresh context so a cancelled ctx (Ctrl-C) still clears the clipboard.
			_ = clipboard.ClearIfUnchanged(context.Background(), password)
			fmt.Fprintln(cmd.ErrOrStderr(), "Clipboard cleared.")
			return nil
		},
	}
	command.Flags().BoolVar(&toStdout, "stdout", false, "print to the terminal instead of the clipboard")
	command.Flags().IntVar(&clearAfter, "clear-after", 30, "seconds before the clipboard is auto-cleared (minimum 5)")
	command.Flags().StringVar(&account, "account", "", "reveal a specific account in the NAS password book (default: the primary login)")
	return command
}

// auditReveal records a reveal-password outcome to stderr. It never records the
// password value — only the profile, the sink, and what happened.
func auditReveal(cmd *cobra.Command, nas, sink, outcome string) {
	fmt.Fprintf(cmd.ErrOrStderr(), "[audit] action=reveal-password nas=%q sink=%s outcome=%s\n", nas, sink, outcome)
}
