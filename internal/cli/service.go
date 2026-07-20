package cli

import (
	"github.com/ychiu1211/dsmctl/internal/application"
	"github.com/ychiu1211/dsmctl/internal/config"
	"github.com/ychiu1211/dsmctl/internal/credentials"
	"github.com/ychiu1211/dsmctl/internal/runtime"
)

func loadService(opts *options) (*application.Service, error) {
	cfg, err := config.NewStore(opts.configPath).Load()
	if err != nil {
		return nil, err
	}
	// The runtime prefers a stored web-login session; this resolver is the
	// automatic, non-interactive fallback when no session can be resumed. It
	// checks the OS credential store first ('auth password set') and then the
	// profile's environment variable, matching the stdio MCP entry point.
	secrets := credentials.NewSecureStore()
	managerOptions := []runtime.Option{
		runtime.WithDeviceStore(secrets),
		runtime.WithSessionStore(secrets),
	}
	if logger := buildLogger(opts.logLevel); logger != nil {
		managerOptions = append(managerOptions, runtime.WithLogger(logger))
	}
	manager := runtime.NewManager(cfg, secrets, managerOptions...)
	return application.NewService(cfg, manager,
		application.WithCredentialStore(secrets),
		application.WithDiscoveryStore(application.DiscoveryStorePath(opts.configPath)),
	), nil
}
