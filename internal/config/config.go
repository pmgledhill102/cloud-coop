// Package config handles cloudcoop configuration.
package config

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"

	"github.com/cloud-coop/cloudcoop/internal/apperrors"
)

// Config represents the cloudcoop configuration.
type Config struct {
	Cloud CloudConfig `toml:"cloud"`
	VM    VMConfig    `toml:"vm"`
}

// CloudConfig contains cloud provider settings.
type CloudConfig struct {
	Provider string    `toml:"provider"` // gcp, aws, azure
	GCP      GCPConfig `toml:"gcp"`
}

// GCPConfig contains GCP-specific settings.
type GCPConfig struct {
	Project string `toml:"project"`
	Zone    string `toml:"zone"`
}

// VMConfig contains VM settings.
type VMConfig struct {
	Name string `toml:"name"`
}

// DefaultConfigPath returns the default user config file path.
func DefaultConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", apperrors.Wrap(err, "get home directory")
	}
	return filepath.Join(home, ".config", "cloudcoop", "cloudcoop.toml"), nil
}

// Load loads configuration from the default locations.
// It checks ./cloudcoop.toml first, then ~/.config/cloudcoop/cloudcoop.toml.
func Load() (*Config, error) {
	// Try local config first
	if cfg, err := LoadFile("cloudcoop.toml"); err == nil {
		return cfg, nil
	}

	// Fall back to user config
	path, err := DefaultConfigPath()
	if err != nil {
		return nil, err
	}

	return LoadFile(path)
}

// LoadFile loads configuration from a specific file.
func LoadFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, apperrors.Wrapf(err, "config file not found: %s", path)
		}
		return nil, apperrors.Wrap(err, "read config file")
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, apperrors.Wrap(err, "parse config file")
	}

	// Set defaults
	if cfg.Cloud.Provider == "" {
		cfg.Cloud.Provider = "gcp"
	}

	return &cfg, nil
}

// Validate checks that required configuration is present.
func (c *Config) Validate() error {
	switch c.Cloud.Provider {
	case "gcp":
		if c.Cloud.GCP.Project == "" {
			return errors.New("cloud.gcp.project is required")
		}
		if c.Cloud.GCP.Zone == "" {
			return errors.New("cloud.gcp.zone is required")
		}
	case "aws", "azure":
		return errors.New("provider not yet implemented: " + c.Cloud.Provider)
	default:
		return errors.New("unknown cloud provider: " + c.Cloud.Provider)
	}

	if c.VM.Name == "" {
		return errors.New("vm.name is required")
	}

	return nil
}
