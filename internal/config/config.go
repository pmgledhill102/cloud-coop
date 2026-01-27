// Package config handles cloudcoop configuration.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/BurntSushi/toml"

	"github.com/cloud-coop/cloudcoop/internal/apperrors"
)

// Config represents the cloudcoop configuration.
type Config struct {
	Cloud        CloudConfig        `toml:"cloud"`
	VM           VMConfig           `toml:"vm"`
	SSH          SSHConfig          `toml:"ssh"`
	Agents       AgentsConfig       `toml:"agents"`
	Provisioning ProvisioningConfig `toml:"provisioning"`
}

// AgentsConfig contains settings for agent sessions.
type AgentsConfig struct {
	DefaultCommand string `toml:"default_command"` // Default command for new agents (e.g., "claude --dangerously-skip-permissions")
}

// ProvisioningConfig contains settings for VM provisioning.
type ProvisioningConfig struct {
	ScriptURL string `toml:"script_url"` // URL to fetch provisioning script from
}

// SSHConfig contains SSH connection settings.
type SSHConfig struct {
	Port int    `toml:"port"` // SSH port (default: 22)
	User string `toml:"user"` // SSH username (default: current user)
}

// CloudConfig contains cloud provider settings.
type CloudConfig struct {
	Provider string    `toml:"provider"` // gcp, aws, azure
	GCP      GCPConfig `toml:"gcp"`
}

// GCPConfig contains GCP-specific settings.
type GCPConfig struct {
	Project        string `toml:"project"`
	Zone           string `toml:"zone"`
	ServiceAccount string `toml:"service_account"`
}

// VMConfig contains VM settings.
type VMConfig struct {
	Name         string            `toml:"name"`
	DiskSizeGB   int64             `toml:"disk_size_gb"`  // Boot disk size in GB (default: 50)
	Image        string            `toml:"image"`         // Boot disk image (default: Ubuntu 24.04 ARM)
	Spot         bool              `toml:"spot"`          // Use spot/preemptible instances
	Network      string            `toml:"network"`       // VPC network name (default: "default")
	Tags         []string          `toml:"tags"`          // Network tags for firewall rules
	MachineSizes map[string]string `toml:"machine_sizes"` // Size name -> machine type mapping
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
	if cfg.SSH.Port == 0 {
		cfg.SSH.Port = 22
	}
	if cfg.VM.DiskSizeGB == 0 {
		cfg.VM.DiskSizeGB = 50
	}
	if cfg.VM.Image == "" {
		cfg.VM.Image = "projects/ubuntu-os-cloud/global/images/family/ubuntu-2504-arm64"
	}
	if cfg.VM.Network == "" {
		cfg.VM.Network = "default"
	}
	if cfg.VM.MachineSizes == nil {
		cfg.VM.MachineSizes = map[string]string{
			"small":  "c4a-highcpu-4",
			"medium": "c4a-highcpu-8",
			"large":  "c4a-highcpu-16",
			"xlarge": "c4a-highcpu-32",
		}
	}
	if cfg.Provisioning.ScriptURL == "" {
		cfg.Provisioning.ScriptURL = "https://raw.githubusercontent.com/pmgledhill102/cloudcoop/main/scripts/provision-vm.sh"
	}

	return &cfg, nil
}

// Exists checks if a configuration file exists at the default location.
func Exists() bool {
	path, err := DefaultConfigPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

// Save writes the configuration to the specified path.
// It creates parent directories if needed and sets secure permissions.
func (c *Config) Save(path string) error {
	// Create parent directories with secure permissions
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return apperrors.Wrap(err, "create config directory")
	}

	// Encode to TOML
	var buf bytes.Buffer
	encoder := toml.NewEncoder(&buf)
	if err := encoder.Encode(c); err != nil {
		return apperrors.Wrap(err, "encode config")
	}

	// Write with secure permissions (owner read/write only)
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		return apperrors.Wrap(err, "write config file")
	}

	return nil
}

// SetValue sets a configuration value by dot-notation key (e.g., "cloud.gcp.project").
// Returns an error if the key is not recognized.
func (c *Config) SetValue(key, value string) error {
	switch key {
	case "cloud.provider":
		c.Cloud.Provider = value
	case "cloud.gcp.project":
		c.Cloud.GCP.Project = value
	case "cloud.gcp.zone":
		c.Cloud.GCP.Zone = value
	case "cloud.gcp.service_account":
		c.Cloud.GCP.ServiceAccount = value
	case "vm.name":
		c.VM.Name = value
	case "ssh.port":
		port, err := strconv.Atoi(value)
		if err != nil {
			return errors.New("ssh.port must be a number")
		}
		if port < 1 || port > 65535 {
			return errors.New("ssh.port must be between 1 and 65535")
		}
		c.SSH.Port = port
	case "ssh.user":
		c.SSH.User = value
	case "agents.default_command":
		c.Agents.DefaultCommand = value
	case "provisioning.script_url":
		c.Provisioning.ScriptURL = value
	default:
		return fmt.Errorf("unknown config key: %s", key)
	}
	return nil
}

// GetValue returns a configuration value by dot-notation key.
func (c *Config) GetValue(key string) (string, error) {
	switch key {
	case "cloud.provider":
		return c.Cloud.Provider, nil
	case "cloud.gcp.project":
		return c.Cloud.GCP.Project, nil
	case "cloud.gcp.zone":
		return c.Cloud.GCP.Zone, nil
	case "cloud.gcp.service_account":
		return c.Cloud.GCP.ServiceAccount, nil
	case "vm.name":
		return c.VM.Name, nil
	case "ssh.port":
		return strconv.Itoa(c.SSH.Port), nil
	case "ssh.user":
		return c.SSH.User, nil
	case "agents.default_command":
		return c.Agents.DefaultCommand, nil
	case "provisioning.script_url":
		return c.Provisioning.ScriptURL, nil
	default:
		return "", fmt.Errorf("unknown config key: %s", key)
	}
}

// AllKeys returns all valid configuration keys in dot notation.
func AllKeys() []string {
	return []string{
		"cloud.provider",
		"cloud.gcp.project",
		"cloud.gcp.zone",
		"cloud.gcp.service_account",
		"vm.name",
		"ssh.port",
		"ssh.user",
		"agents.default_command",
		"provisioning.script_url",
	}
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
		if c.Cloud.GCP.ServiceAccount == "" {
			return errors.New("cloud.gcp.service_account is required; see docs/SETUP-FLOW.md for setup instructions")
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
