package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const configEnvironmentVariable = "DSMCTL_CONFIG"

type Store struct {
	path string
}

func DefaultPath() string {
	if path := os.Getenv(configEnvironmentVariable); path != "" {
		return path
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "dsmctl.json"
	}
	return filepath.Join(dir, "dsmctl", "config.json")
}

func NewStore(path string) *Store {
	if path == "" {
		path = DefaultPath()
	}
	return &Store{path: path}
}

func (s *Store) Path() string {
	return s.path
}

func (s *Store) Load() (*Config, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return New(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", s.path, err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("decode config %s: %w", s.path, err)
	}
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config %s: %w", s.path, err)
	}
	return &cfg, nil
}

func (s *Store) Save(cfg *Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	data = append(data, '\n')

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	temp, err := os.CreateTemp(dir, ".config-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary config: %w", err)
	}
	tempName := temp.Name()
	defer os.Remove(tempName)

	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return fmt.Errorf("set config permissions: %w", err)
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return fmt.Errorf("write config: %w", err)
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return fmt.Errorf("sync config: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close config: %w", err)
	}
	if err := os.Rename(tempName, s.path); err != nil {
		// Windows does not replace an existing destination with os.Rename.
		// Keep the temporary file complete until the old config is removed.
		if _, statErr := os.Stat(s.path); statErr != nil {
			return fmt.Errorf("replace config: %w", err)
		}
		if removeErr := os.Remove(s.path); removeErr != nil {
			return fmt.Errorf("replace config: %w", removeErr)
		}
		if renameErr := os.Rename(tempName, s.path); renameErr != nil {
			return fmt.Errorf("replace config after removing old file: %w", renameErr)
		}
	}
	return nil
}
