package cli

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/ychiu1211/dsmctl/internal/config"
	"github.com/ychiu1211/dsmctl/internal/credentials"
	"github.com/ychiu1211/dsmctl/internal/provision"
)

func newProvisionCommand(opts *options) *cobra.Command {
	var adminUser, targetURL, deviceName, autoUpdate string
	var skipTLS, analytics, finishOnly bool
	var length int
	command := &cobra.Command{
		Use:   "provision <name>",
		Short: "Bring up a factory/recovery NAS: create the first administrator, store a generated password, and finish DSM setup",
		Long: "provision takes a Synology NAS in its DSM setup window to a fully configured DSM: it creates the first\n" +
			"administrator (username yours via --admin-user; password generated locally and stored in the OS credential\n" +
			"store, never printed), applies the update policy and privacy defaults, and marks the setup wizard finished.\n" +
			"Retrieve the password later with 'dsmctl auth reveal-password'. Use --finish-only to run just the post-account\n" +
			"wizard steps against a NAS whose administrator already exists (logs in with the stored password).",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			name := args[0]
			if err := config.ValidateName(name); err != nil {
				return err
			}
			setup := provision.SetupOptions{AutoUpdate: autoUpdate, Analytics: analytics}

			// --finish-only: the administrator already exists. Log in with the
			// stored password (resolved internally, never printed) and run only
			// the remaining wizard steps. This is also how a provision that was
			// interrupted after account creation is completed.
			if finishOnly {
				cfg, err := config.NewStore(opts.configPath).Load()
				if err != nil {
					return err
				}
				profile, ok := cfg.NAS[name]
				if !ok {
					return fmt.Errorf("NAS profile %q is not configured", name)
				}
				target, err := provisionTarget(profile.URL, profile.InsecureSkipTLSVerify)
				if err != nil {
					return err
				}
				password, err := credentials.NewSecureStore().Password(ctx, name, profile)
				if err != nil {
					return fmt.Errorf("need the stored administrator password to finish setup: %w", err)
				}
				if err := provision.Login(ctx, target, profile.Username, password); err != nil {
					return fmt.Errorf("log in as %q to finish setup: %w", profile.Username, err)
				}
				if err := provision.CompleteSetup(ctx, target, setup); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Finished the DSM setup wizard for NAS %q.\n", name)
				return nil
			}

			if strings.TrimSpace(adminUser) == "" {
				return errors.New("--admin-user is required; the administrator username is yours to choose")
			}
			if !strings.HasPrefix(strings.ToLower(targetURL), "https://") {
				return errors.New("--url must be an https URL, for example https://10.17.37.51:5001")
			}
			target, err := provisionTarget(targetURL, skipTLS)
			if err != nil {
				return err
			}
			// Establish the setup session (log in as the built-in admin, which is
			// empty-password during first-run setup) so the account-creation
			// compound is authorized the same way the browser wizard is.
			if err := provision.EstablishSetupSession(ctx, target); err != nil {
				return fmt.Errorf("could not start the DSM setup session; is the NAS in its first-run setup window? %w", err)
			}

			// The generated password lives only in this process's memory and the OS
			// keyring; it is never printed, logged, or returned to a caller.
			password, err := credentials.GeneratePassword(length)
			if err != nil {
				return err
			}
			scramble, err := credentials.GeneratePassword(length)
			if err != nil {
				return err
			}
			req := provision.AdminRequest{Username: adminUser, Password: password, DeviceName: deviceName, ShareLocation: analytics}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Creating administrator %q on %s ...\n", adminUser, targetURL)
			if err := provision.CreateFirstAdmin(ctx, target, req); err != nil {
				return fmt.Errorf("no administrator was created (you can retry): %w", err)
			}
			// Log in as the new administrator: verifies it works and leaves the
			// session in the client jar for the wizard-finish steps below.
			if err := provision.Login(ctx, target, adminUser, password); err != nil {
				return fmt.Errorf("administrator %q was created but the verification login failed: %w", adminUser, err)
			}
			fmt.Fprintf(out, "Administrator %q created and verified.\n", adminUser)

			// Store the only copy of the password before anything else can fail.
			if err := credentials.NewSecureStore().SavePassword(ctx, name, password); err != nil {
				return fmt.Errorf("administrator created, but saving its password to the credential store failed; rotate it: %w", err)
			}
			fmt.Fprintf(out, "Stored the generated password in the OS credential store (service dsmctl, profile %q).\n", name)

			if err := provision.CompleteSetup(ctx, target, setup); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Note: could not finish every setup step (%v); the administrator is created and usable.\n", err)
			} else {
				fmt.Fprintln(out, "Applied update policy and privacy defaults, and finished the setup wizard.")
			}
			if err := provision.Harden(ctx, target, req, scramble); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Note: post-setup hardening was incomplete (%v); the administrator is created and usable.\n", err)
			}

			store := config.NewStore(opts.configPath)
			cfg, err := store.Load()
			if err != nil {
				return err
			}
			profile := cfg.NAS[name]
			profile.URL = targetURL
			profile.Username = adminUser
			profile.InsecureSkipTLSVerify = skipTLS
			cfg.NAS[name] = profile
			if cfg.DefaultNAS == "" {
				cfg.DefaultNAS = name
			}
			if err := store.Save(cfg); err != nil {
				return err
			}

			fmt.Fprintf(out, "\nProvisioned NAS %q as administrator %q.\n", name, adminUser)
			fmt.Fprintf(out, "Retrieve the password (a human, at a terminal) with:\n    dsmctl auth reveal-password --nas %s\n", name)
			return nil
		},
	}
	command.Flags().StringVar(&adminUser, "admin-user", "", "administrator username to create (required unless --finish-only; your choice, never generated)")
	command.Flags().StringVar(&targetURL, "url", "", "DSM https URL of the NAS in its setup window, e.g. https://10.17.37.51:5001 (required unless --finish-only)")
	command.Flags().StringVar(&deviceName, "device-name", "", "DSM server name (hostname) to set")
	command.Flags().StringVar(&autoUpdate, "auto-update", "security", "DSM update policy: security (auto-install security hotfixes), all, or notify")
	command.Flags().BoolVar(&skipTLS, "insecure-skip-tls-verify", false, "accept the NAS's fresh self-signed certificate (for an explicitly isolated lab NAS)")
	command.Flags().BoolVar(&analytics, "analytics", false, "opt in to Synology device analytics / Active Insight (default off)")
	command.Flags().BoolVar(&finishOnly, "finish-only", false, "skip account creation; only run the post-account wizard steps, logging in with the stored password")
	command.Flags().IntVar(&length, "length", credentials.DefaultGeneratedPasswordLength, "generated password length")
	return command
}

// provisionTarget builds an HTTP client with a cookie jar (to carry the DSM
// setup/login session) and an optional self-signed-certificate bypass.
func provisionTarget(baseURL string, skipTLS bool) (provision.Target, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return provision.Target{}, err
	}
	client := &http.Client{Timeout: 90 * time.Second, Jar: jar}
	if skipTLS {
		client.Transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	}
	return provision.Target{BaseURL: baseURL, HTTPClient: client}, nil
}
