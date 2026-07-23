package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/derekvery666/dsmctl/internal/application"
	"github.com/derekvery666/dsmctl/internal/buildinfo"
	"github.com/derekvery666/dsmctl/internal/config"
	"github.com/derekvery666/dsmctl/internal/credentials"
	"github.com/derekvery666/dsmctl/internal/mcpserver"
	"github.com/derekvery666/dsmctl/internal/observability"
	"github.com/derekvery666/dsmctl/internal/runtime"
)

func main() {
	flag.Usage = func() {
		output := flag.CommandLine.Output()
		fmt.Fprint(output, mcpUsagePreamble(os.Args[0]))
		flag.PrintDefaults()
	}
	configPath := flag.String("config", config.DefaultPath(), "configuration file path")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Fprintf(os.Stdout, "dsmctl-mcp %s\n", buildinfo.Version)
		return
	}

	cfg, err := config.NewStore(*configPath).Load()
	if err != nil {
		fatal(err)
	}
	secrets := credentials.NewSecureStore()
	// Prefer the stored web-login session, exactly like the CLI. The password
	// resolver stays as the automation fallback (environment variable, or a
	// password stored by an older release).
	managerOptions := []runtime.Option{
		runtime.WithDeviceStore(secrets),
		runtime.WithSessionStore(secrets),
	}
	// Opt-in diagnostic logging via DSMCTL_LOG_LEVEL. It goes to stderr only —
	// stdout carries the JSON-RPC frames — and redacts secret parameters.
	if level, ok := observability.ParseLevel(os.Getenv("DSMCTL_LOG_LEVEL")); ok {
		managerOptions = append(managerOptions, runtime.WithLogger(observability.New(os.Stderr, level)))
	}
	manager := runtime.NewManager(cfg, secrets, managerOptions...)
	service := application.NewService(cfg, manager,
		application.WithCredentialStore(secrets),
		application.WithDiscoveryStore(application.DiscoveryStorePath(*configPath)),
	)
	server := mcpserver.New(service, buildinfo.Version)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	runErr := server.Run(ctx, &mcp.StdioTransport{})
	closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	closeErr := service.Close(closeCtx)
	if runErr != nil {
		fatal(runErr)
	}
	if closeErr != nil {
		fmt.Fprintln(os.Stderr, "dsmctl-mcp: close sessions:", closeErr)
	}
}

func mcpUsagePreamble(executable string) string {
	return fmt.Sprintf(`Run dsmctl's MCP server over stdio for an MCP client.

This process is not an interactive shell: stdout is reserved for JSON-RPC.
Configure NAS profiles with "dsmctl nas add", authenticate them with
"dsmctl auth login --nas <name>", and launch this binary from the MCP client.
Agents should begin with list_nas and get_auth_status, pass nas explicitly,
use get_* tools for reads, and use only matching plan_* then approved apply_*
tools for mutations.

Usage:
  %s [flags]

Flags:
`, executable)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "dsmctl-mcp:", err)
	os.Exit(1)
}
