package cli

import (
	"github.com/ychiu1211/dsmctl/internal/application"
	"github.com/ychiu1211/dsmctl/internal/config"
	"github.com/ychiu1211/dsmctl/internal/credentials"
	"github.com/ychiu1211/dsmctl/internal/runtime"
)

func loadService(configPath string) (*application.Service, error) {
	cfg, err := config.NewStore(configPath).Load()
	if err != nil {
		return nil, err
	}
	manager := runtime.NewManager(cfg, credentials.NewEnvironment())
	return application.NewService(cfg, manager), nil
}
