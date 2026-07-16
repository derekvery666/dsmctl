package runtime

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/ychiu1211/dsmctl/internal/config"
	"github.com/ychiu1211/dsmctl/internal/credentials"
	"github.com/ychiu1211/dsmctl/internal/synology"
)

type SystemInfoClient interface {
	SystemInfo(ctx context.Context) (synology.SystemInfo, error)
}

type Manager struct {
	config      *config.Config
	credentials credentials.Resolver

	mu      sync.Mutex
	clients map[string]*synology.Client
}

func NewManager(cfg *config.Config, resolver credentials.Resolver) *Manager {
	return &Manager{
		config:      cfg,
		credentials: resolver,
		clients:     make(map[string]*synology.Client),
	}
}

// Client resolves a NAS profile and lazily creates one reusable authenticated
// client per profile. Separate profiles can therefore hold independent DSM
// sessions at the same time.
func (m *Manager) Client(ctx context.Context, requested string) (string, SystemInfoClient, error) {
	name, profile, err := m.config.Resolve(requested)
	if err != nil {
		return "", nil, err
	}

	m.mu.Lock()
	if client, ok := m.clients[name]; ok {
		m.mu.Unlock()
		return name, client, nil
	}
	m.mu.Unlock()

	password, err := m.credentials.Password(ctx, name, profile)
	if err != nil {
		return "", nil, err
	}
	client, err := synology.NewClient(synology.Options{
		BaseURL:    profile.URL,
		Username:   profile.Username,
		Password:   password,
		HTTPClient: httpClient(profile),
	})
	if err != nil {
		return "", nil, fmt.Errorf("create client for NAS %q: %w", name, err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if existing, ok := m.clients[name]; ok {
		return name, existing, nil
	}
	m.clients[name] = client
	return name, client, nil
}

func (m *Manager) Close(ctx context.Context) error {
	m.mu.Lock()
	clients := m.clients
	m.clients = make(map[string]*synology.Client)
	m.mu.Unlock()

	var closeErrors []error
	for name, client := range clients {
		if err := client.Close(ctx); err != nil {
			closeErrors = append(closeErrors, fmt.Errorf("NAS %q: %w", name, err))
		}
	}
	return errors.Join(closeErrors...)
}

func httpClient(profile config.Profile) *http.Client {
	timeout := 30 * time.Second
	if profile.TimeoutSeconds > 0 {
		timeout = time.Duration(profile.TimeoutSeconds) * time.Second
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: profile.InsecureSkipTLSVerify, // Explicit per-profile opt-in for self-signed test NAS devices.
	}
	return &http.Client{Transport: transport, Timeout: timeout}
}
