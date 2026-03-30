// Package config handles cloudcoop configuration.
package config

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/cloud-coop/cloudcoop/internal/apperrors"
)

// Config represents the cloudcoop configuration.
type Config struct {
	Cloud        CloudConfig        `toml:"cloud,omitempty"`
	VM           VMConfig           `toml:"vm,omitempty"`
	SSH          SSHConfig          `toml:"ssh,omitempty"`
	Setup        SetupConfig        `toml:"setup,omitempty"`
	Agents       AgentsConfig       `toml:"agents,omitempty"`
	Provisioning ProvisioningConfig `toml:"provisioning,omitempty"`
	TUI          TUIConfig          `toml:"tui,omitempty"`
}

// SetupConfig contains settings for `cloudcoop setup` provisioning.
type SetupConfig struct {
	ExtraAPIs     []string `toml:"extra_apis,omitempty"`      // Additional GCP APIs to enable beyond the base set
	ExtraIAMRoles []string `toml:"extra_iam_roles,omitempty"` // Additional IAM roles to grant beyond the base set
}

// TUIConfig contains TUI-specific settings.
type TUIConfig struct {
	RefreshIntervalSec int `toml:"refresh_interval_sec,omitzero"` // Auto-refresh interval during operations (default: 5)
}

// AgentsConfig contains settings for agent sessions.
type AgentsConfig struct {
	DefaultCommand string                `toml:"default_command,omitempty"` // Default command for new agents (e.g., "claude --dangerously-skip-permissions")
	PreCommands    []string              `toml:"pre_commands,omitempty"`    // Commands to run before agent in every session
	Repos          map[string]RepoConfig `toml:"repos,omitempty"`           // Per-repo overrides keyed by slug
}

// RepoConfig holds per-repo agent overrides.
type RepoConfig struct {
	Command     string   `toml:"command,omitempty"`      // Agent command override for this repo
	PreCommands []string `toml:"pre_commands,omitempty"` // Pre-commands specific to this repo
}

// ResolveCommand returns the agent command for a given repo slug.
// Priority: repo-specific → default → empty string.
func (a *AgentsConfig) ResolveCommand(slug string) string {
	if a.Repos != nil {
		if rc, ok := a.Repos[slug]; ok && rc.Command != "" {
			return rc.Command
		}
	}
	return a.DefaultCommand
}

// ResolvePreCommands returns the concatenated pre-commands for a given slug.
// Global pre-commands come first, then repo-specific.
func (a *AgentsConfig) ResolvePreCommands(slug string) []string {
	var cmds []string
	cmds = append(cmds, a.PreCommands...)
	if a.Repos != nil {
		if rc, ok := a.Repos[slug]; ok {
			cmds = append(cmds, rc.PreCommands...)
		}
	}
	return cmds
}

// ProvisioningConfig contains settings for VM provisioning.
type ProvisioningConfig struct {
	ScriptURL   string `toml:"script_url,omitempty"`   // URL to fetch provisioning script from
	DotfilesURL string `toml:"dotfiles_url,omitempty"` // URL to a dotfiles install script (run as sandbox user after provisioning)
}

// SSHConfig contains SSH connection settings.
type SSHConfig struct {
	Port int    `toml:"port,omitzero"`  // SSH port (default: 22)
	User string `toml:"user,omitempty"` // SSH username (default: current user)
}

// CloudConfig contains cloud provider settings.
type CloudConfig struct {
	Provider string    `toml:"provider,omitempty"` // gcp, aws, azure
	GCP      GCPConfig `toml:"gcp,omitempty"`
}

// GCPConfig contains GCP-specific settings.
type GCPConfig struct {
	Project        string `toml:"project,omitempty"`
	Zone           string `toml:"zone,omitempty"`
	ServiceAccount string `toml:"service_account,omitempty"`
}

// VMConfig contains VM settings.
type VMConfig struct {
	Name             string            `toml:"name,omitempty"`
	DiskSizeGB       int64             `toml:"disk_size_gb,omitzero"`       // Boot disk size in GB (default: 50)
	Image            string            `toml:"image,omitempty"`             // Boot disk image (default: Ubuntu 24.04 ARM)
	Spot             bool              `toml:"spot,omitempty"`              // Use spot/preemptible instances
	MaxUptimeMinutes int               `toml:"max_uptime_minutes,omitzero"` // Auto-stop after N minutes (0=disabled)
	Network          string            `toml:"network,omitempty"`           // VPC network name (default: "default")
	Subnet           string            `toml:"subnet,omitempty"`            // VPC subnet name (required for custom-mode VPCs)
	Tags             []string          `toml:"tags,omitempty"`              // Network tags for firewall rules
	MachineSizes     map[string]string `toml:"machine_sizes,omitempty"`     // Size name -> machine type mapping
}

// DefaultConfigPath returns the default user config file path.
func DefaultConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", apperrors.Wrap(err, "get home directory")
	}
	return filepath.Join(home, ".config", "cloudcoop", "cloudcoop.toml"), nil
}

// ProjectConfigDir is the directory name for per-project configuration.
const ProjectConfigDir = ".cloudcoop"

// ProjectConfigPath returns the project config file path relative to the given directory.
// The project config lives at <dir>/.cloudcoop/config.toml.
func ProjectConfigPath(dir string) string {
	return filepath.Join(dir, ProjectConfigDir, "config.toml")
}

// InstanceConfigPath returns the instance config file path relative to the given directory.
// The instance config lives at <dir>/.cloudcoop/local.toml and is gitignored.
func InstanceConfigPath(dir string) string {
	return filepath.Join(dir, ProjectConfigDir, "local.toml")
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

// LoadMerged loads config from all layers (lowest to highest priority):
//  1. Global: ~/.config/cloudcoop/cloudcoop.toml
//  2. Legacy: ./cloudcoop.toml
//  3. Repo: .cloudcoop/config.toml (git-tracked, team settings)
//  4. Instance: .cloudcoop/local.toml (gitignored, per-developer)
//  5. Apply defaults
func LoadMerged() (*Config, error) {
	// Start with global config (or empty if not found)
	globalPath, err := DefaultConfigPath()
	if err != nil {
		return nil, err
	}

	cfg, err := loadFileRaw(globalPath)
	if err != nil {
		// Global config not found is OK - use empty config
		cfg = &Config{}
	}

	// Also check legacy local config (./cloudcoop.toml) for backwards compat
	if legacyCfg, err := loadFileRaw("cloudcoop.toml"); err == nil {
		mergeConfig(cfg, legacyCfg)
	}

	// Overlay repo config if it exists
	projectPath := ProjectConfigPath(".")
	if projectCfg, err := loadFileRaw(projectPath); err == nil {
		mergeConfig(cfg, projectCfg)
	}

	// Overlay instance config if it exists (highest priority)
	instancePath := InstanceConfigPath(".")
	if instanceCfg, err := loadFileRaw(instancePath); err == nil {
		mergeConfig(cfg, instanceCfg)
	}

	applyDefaults(cfg)
	return cfg, nil
}

// SaveInstance writes instance-specific fields to .cloudcoop/local.toml.
// It creates the .cloudcoop/ directory if needed and ensures local.toml
// is listed in .cloudcoop/.gitignore.
func SaveInstance(dir string, project, zone, serviceAccount, vmName, network, subnet string) error {
	cfg := &Config{
		Cloud: CloudConfig{
			GCP: GCPConfig{
				Project:        project,
				Zone:           zone,
				ServiceAccount: serviceAccount,
			},
		},
		VM: VMConfig{
			Name:    vmName,
			Network: network,
			Subnet:  subnet,
		},
	}

	path := InstanceConfigPath(dir)
	if err := cfg.Save(path); err != nil {
		return err
	}
	return ensureGitignore(dir)
}

// SaveProject is deprecated: use SaveInstance instead.
// It delegates to SaveInstance which writes to .cloudcoop/local.toml.
func SaveProject(dir string, project, zone, serviceAccount, vmName, network, subnet string) error {
	return SaveInstance(dir, project, zone, serviceAccount, vmName, network, subnet)
}

// ensureGitignore adds "local.toml" to .cloudcoop/.gitignore if not already present.
func ensureGitignore(dir string) error {
	gitignorePath := filepath.Join(dir, ProjectConfigDir, ".gitignore")

	// Read existing content if any
	existing, err := os.ReadFile(gitignorePath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return apperrors.Wrap(err, "read .cloudcoop/.gitignore")
	}

	const entry = "local.toml"

	// Check if already present
	if existing != nil {
		scanner := bufio.NewScanner(bytes.NewReader(existing))
		for scanner.Scan() {
			if strings.TrimSpace(scanner.Text()) == entry {
				return nil // already present
			}
		}
	}

	// Append the entry
	f, err := os.OpenFile(gitignorePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return apperrors.Wrap(err, "open .cloudcoop/.gitignore")
	}
	defer f.Close()

	// Add newline before entry if file has content and doesn't end with newline
	if len(existing) > 0 && existing[len(existing)-1] != '\n' {
		if _, err := f.WriteString("\n"); err != nil {
			return apperrors.Wrap(err, "write .cloudcoop/.gitignore")
		}
	}
	if _, err := f.WriteString(entry + "\n"); err != nil {
		return apperrors.Wrap(err, "write .cloudcoop/.gitignore")
	}

	return nil
}

// loadFileRaw loads a config file without applying defaults.
func loadFileRaw(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, apperrors.Wrap(err, "parse config file")
	}
	return &cfg, nil
}

// mergeConfig overlays non-zero values from src onto dst.
func mergeConfig(dst, src *Config) {
	if src.Cloud.Provider != "" {
		dst.Cloud.Provider = src.Cloud.Provider
	}
	if src.Cloud.GCP.Project != "" {
		dst.Cloud.GCP.Project = src.Cloud.GCP.Project
	}
	if src.Cloud.GCP.Zone != "" {
		dst.Cloud.GCP.Zone = src.Cloud.GCP.Zone
	}
	if src.Cloud.GCP.ServiceAccount != "" {
		dst.Cloud.GCP.ServiceAccount = src.Cloud.GCP.ServiceAccount
	}
	if src.VM.Name != "" {
		dst.VM.Name = src.VM.Name
	}
	if src.VM.DiskSizeGB != 0 {
		dst.VM.DiskSizeGB = src.VM.DiskSizeGB
	}
	if src.VM.Image != "" {
		dst.VM.Image = src.VM.Image
	}
	if src.VM.Spot {
		dst.VM.Spot = true
	}
	if src.VM.MaxUptimeMinutes != 0 {
		dst.VM.MaxUptimeMinutes = src.VM.MaxUptimeMinutes
	}
	if src.VM.Network != "" {
		dst.VM.Network = src.VM.Network
	}
	if src.VM.Subnet != "" {
		dst.VM.Subnet = src.VM.Subnet
	}
	if len(src.VM.Tags) > 0 {
		dst.VM.Tags = src.VM.Tags
	}
	if len(src.VM.MachineSizes) > 0 {
		dst.VM.MachineSizes = src.VM.MachineSizes
	}
	if len(src.Setup.ExtraAPIs) > 0 {
		dst.Setup.ExtraAPIs = src.Setup.ExtraAPIs
	}
	if len(src.Setup.ExtraIAMRoles) > 0 {
		dst.Setup.ExtraIAMRoles = src.Setup.ExtraIAMRoles
	}
	if src.SSH.Port != 0 {
		dst.SSH.Port = src.SSH.Port
	}
	if src.SSH.User != "" {
		dst.SSH.User = src.SSH.User
	}
	if src.Agents.DefaultCommand != "" {
		dst.Agents.DefaultCommand = src.Agents.DefaultCommand
	}
	if len(src.Agents.PreCommands) > 0 {
		dst.Agents.PreCommands = src.Agents.PreCommands
	}
	if len(src.Agents.Repos) > 0 {
		dst.Agents.Repos = src.Agents.Repos
	}
	if src.Provisioning.ScriptURL != "" {
		dst.Provisioning.ScriptURL = src.Provisioning.ScriptURL
	}
	if src.Provisioning.DotfilesURL != "" {
		dst.Provisioning.DotfilesURL = src.Provisioning.DotfilesURL
	}
	if src.TUI.RefreshIntervalSec != 0 {
		dst.TUI.RefreshIntervalSec = src.TUI.RefreshIntervalSec
	}
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

	applyDefaults(&cfg)
	return &cfg, nil
}

// applyDefaults sets default values for any unset fields.
func applyDefaults(cfg *Config) {
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
		cfg.VM.Image = "projects/ubuntu-os-cloud/global/images/family/ubuntu-2404-lts-arm64"
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
		cfg.Provisioning.ScriptURL = "https://raw.githubusercontent.com/pmgledhill102/cloud-coop/main/scripts/provision-vm.sh"
	}
	if cfg.TUI.RefreshIntervalSec == 0 {
		cfg.TUI.RefreshIntervalSec = 15
	}
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
// Returns an error if the key is not recognised.
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
	case "vm.max_uptime_minutes":
		minutes, err := strconv.Atoi(value)
		if err != nil {
			return errors.New("vm.max_uptime_minutes must be a number")
		}
		if minutes < 0 {
			return errors.New("vm.max_uptime_minutes must be >= 0")
		}
		c.VM.MaxUptimeMinutes = minutes
	case "vm.network":
		c.VM.Network = value
	case "vm.subnet":
		c.VM.Subnet = value
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
	case "provisioning.dotfiles_url":
		c.Provisioning.DotfilesURL = value
	case "tui.refresh_interval_sec":
		interval, err := strconv.Atoi(value)
		if err != nil {
			return errors.New("tui.refresh_interval_sec must be a number")
		}
		if interval < 1 || interval > 300 {
			return errors.New("tui.refresh_interval_sec must be between 1 and 300")
		}
		c.TUI.RefreshIntervalSec = interval
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
	case "vm.max_uptime_minutes":
		return strconv.Itoa(c.VM.MaxUptimeMinutes), nil
	case "vm.network":
		return c.VM.Network, nil
	case "vm.subnet":
		return c.VM.Subnet, nil
	case "ssh.port":
		return strconv.Itoa(c.SSH.Port), nil
	case "ssh.user":
		return c.SSH.User, nil
	case "agents.default_command":
		return c.Agents.DefaultCommand, nil
	case "provisioning.script_url":
		return c.Provisioning.ScriptURL, nil
	case "provisioning.dotfiles_url":
		return c.Provisioning.DotfilesURL, nil
	case "tui.refresh_interval_sec":
		return strconv.Itoa(c.TUI.RefreshIntervalSec), nil
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
		"vm.max_uptime_minutes",
		"vm.network",
		"vm.subnet",
		"ssh.port",
		"ssh.user",
		"agents.default_command",
		"provisioning.script_url",
		"provisioning.dotfiles_url",
		"tui.refresh_interval_sec",
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
