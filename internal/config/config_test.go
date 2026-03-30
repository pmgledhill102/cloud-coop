package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadFile(t *testing.T) {
	// Create temp config file
	dir := t.TempDir()
	path := filepath.Join(dir, "cloudcoop.toml")

	content := `
[cloud]
provider = "gcp"

[cloud.gcp]
project = "test-project"
zone = "us-central1-a"

[vm]
name = "test-vm"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Cloud.Provider != "gcp" {
		t.Errorf("provider = %q, want %q", cfg.Cloud.Provider, "gcp")
	}
	if cfg.Cloud.GCP.Project != "test-project" {
		t.Errorf("project = %q, want %q", cfg.Cloud.GCP.Project, "test-project")
	}
	if cfg.Cloud.GCP.Zone != "us-central1-a" {
		t.Errorf("zone = %q, want %q", cfg.Cloud.GCP.Zone, "us-central1-a")
	}
	if cfg.VM.Name != "test-vm" {
		t.Errorf("vm.name = %q, want %q", cfg.VM.Name, "test-vm")
	}
}

func TestLoadFile_NotFound(t *testing.T) {
	_, err := LoadFile("/nonexistent/config.toml")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid gcp config",
			cfg: Config{
				Cloud: CloudConfig{
					Provider: "gcp",
					GCP:      GCPConfig{Project: "proj", Zone: "zone-a", ServiceAccount: "sa@proj.iam.gserviceaccount.com"},
				},
				VM: VMConfig{Name: "vm-1"},
			},
			wantErr: false,
		},
		{
			name: "missing project",
			cfg: Config{
				Cloud: CloudConfig{
					Provider: "gcp",
					GCP:      GCPConfig{Zone: "zone-a", ServiceAccount: "sa@proj.iam.gserviceaccount.com"},
				},
				VM: VMConfig{Name: "vm-1"},
			},
			wantErr: true,
		},
		{
			name: "missing zone",
			cfg: Config{
				Cloud: CloudConfig{
					Provider: "gcp",
					GCP:      GCPConfig{Project: "proj", ServiceAccount: "sa@proj.iam.gserviceaccount.com"},
				},
				VM: VMConfig{Name: "vm-1"},
			},
			wantErr: true,
		},
		{
			name: "missing service account",
			cfg: Config{
				Cloud: CloudConfig{
					Provider: "gcp",
					GCP:      GCPConfig{Project: "proj", Zone: "zone-a"},
				},
				VM: VMConfig{Name: "vm-1"},
			},
			wantErr: true,
		},
		{
			name: "missing vm name",
			cfg: Config{
				Cloud: CloudConfig{
					Provider: "gcp",
					GCP:      GCPConfig{Project: "proj", Zone: "zone-a", ServiceAccount: "sa@proj.iam.gserviceaccount.com"},
				},
			},
			wantErr: true,
		},
		{
			name: "unsupported provider",
			cfg: Config{
				Cloud: CloudConfig{Provider: "unknown"},
				VM:    VMConfig{Name: "vm-1"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDefaultProviderIsGCP(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cloudcoop.toml")

	// Config without explicit provider
	content := `
[cloud.gcp]
project = "test-project"
zone = "us-central1-a"

[vm]
name = "test-vm"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Cloud.Provider != "gcp" {
		t.Errorf("default provider = %q, want %q", cfg.Cloud.Provider, "gcp")
	}
}

func TestConfig_Save(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "cloudcoop.toml")

	cfg := &Config{
		Cloud: CloudConfig{
			Provider: "gcp",
			GCP: GCPConfig{
				Project:        "my-project",
				Zone:           "us-west1-a",
				ServiceAccount: "sa@my-project.iam.gserviceaccount.com",
			},
		},
		VM: VMConfig{
			Name: "test-vm",
		},
		SSH: SSHConfig{
			Port: 22,
			User: "testuser",
		},
	}

	// Save should create parent directories
	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("config file was not created")
	}

	// Check file permissions (should be 0600)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	perm := info.Mode().Perm()
	if perm != 0o600 {
		t.Errorf("file permissions = %o, want %o", perm, 0o600)
	}

	// Load the saved config and verify
	loaded, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}

	if loaded.Cloud.GCP.Project != "my-project" {
		t.Errorf("project = %q, want %q", loaded.Cloud.GCP.Project, "my-project")
	}
	if loaded.Cloud.GCP.Zone != "us-west1-a" {
		t.Errorf("zone = %q, want %q", loaded.Cloud.GCP.Zone, "us-west1-a")
	}
	if loaded.VM.Name != "test-vm" {
		t.Errorf("vm.name = %q, want %q", loaded.VM.Name, "test-vm")
	}
	if loaded.SSH.User != "testuser" {
		t.Errorf("ssh.user = %q, want %q", loaded.SSH.User, "testuser")
	}
}

func TestConfig_SetValue(t *testing.T) {
	tests := []struct {
		key     string
		value   string
		check   func(*Config) bool
		wantErr bool
	}{
		{
			key:     "cloud.provider",
			value:   "gcp",
			check:   func(c *Config) bool { return c.Cloud.Provider == "gcp" },
			wantErr: false,
		},
		{
			key:     "cloud.gcp.project",
			value:   "my-project",
			check:   func(c *Config) bool { return c.Cloud.GCP.Project == "my-project" },
			wantErr: false,
		},
		{
			key:     "cloud.gcp.zone",
			value:   "us-central1-a",
			check:   func(c *Config) bool { return c.Cloud.GCP.Zone == "us-central1-a" },
			wantErr: false,
		},
		{
			key:     "cloud.gcp.service_account",
			value:   "sa@my-project.iam.gserviceaccount.com",
			check:   func(c *Config) bool { return c.Cloud.GCP.ServiceAccount == "sa@my-project.iam.gserviceaccount.com" },
			wantErr: false,
		},
		{
			key:     "vm.name",
			value:   "test-vm",
			check:   func(c *Config) bool { return c.VM.Name == "test-vm" },
			wantErr: false,
		},
		{
			key:     "ssh.port",
			value:   "2222",
			check:   func(c *Config) bool { return c.SSH.Port == 2222 },
			wantErr: false,
		},
		{
			key:     "ssh.user",
			value:   "ubuntu",
			check:   func(c *Config) bool { return c.SSH.User == "ubuntu" },
			wantErr: false,
		},
		{
			key:     "agents.default_command",
			value:   "claude",
			check:   func(c *Config) bool { return c.Agents.DefaultCommand == "claude" },
			wantErr: false,
		},
		{
			key:     "vm.max_uptime_minutes",
			value:   "60",
			check:   func(c *Config) bool { return c.VM.MaxUptimeMinutes == 60 },
			wantErr: false,
		},
		{
			key:     "vm.max_uptime_minutes",
			value:   "0",
			check:   func(c *Config) bool { return c.VM.MaxUptimeMinutes == 0 },
			wantErr: false,
		},
		{
			key:     "vm.max_uptime_minutes",
			value:   "invalid",
			check:   func(c *Config) bool { return true },
			wantErr: true,
		},
		{
			key:     "vm.max_uptime_minutes",
			value:   "-1",
			check:   func(c *Config) bool { return true },
			wantErr: true,
		},
		{
			key:     "vm.network",
			value:   "my-vpc",
			check:   func(c *Config) bool { return c.VM.Network == "my-vpc" },
			wantErr: false,
		},
		{
			key:     "vm.subnet",
			value:   "my-subnet",
			check:   func(c *Config) bool { return c.VM.Subnet == "my-subnet" },
			wantErr: false,
		},
		{
			key:     "provisioning.dotfiles_url",
			value:   "https://example.com/install.sh",
			check:   func(c *Config) bool { return c.Provisioning.DotfilesURL == "https://example.com/install.sh" },
			wantErr: false,
		},
		{
			key:     "unknown.key",
			value:   "value",
			check:   func(c *Config) bool { return true },
			wantErr: true,
		},
		{
			key:     "ssh.port",
			value:   "invalid",
			check:   func(c *Config) bool { return true },
			wantErr: true,
		},
		{
			key:     "ssh.port",
			value:   "99999",
			check:   func(c *Config) bool { return true },
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.key+"="+tt.value, func(t *testing.T) {
			cfg := &Config{} // Fresh config for each test
			err := cfg.SetValue(tt.key, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetValue() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && !tt.check(cfg) {
				t.Errorf("SetValue() did not set value correctly")
			}
		})
	}
}

func TestConfig_GetValue(t *testing.T) {
	cfg := &Config{
		Cloud: CloudConfig{
			Provider: "gcp",
			GCP: GCPConfig{
				Project:        "my-project",
				Zone:           "us-central1-a",
				ServiceAccount: "sa@my-project.iam.gserviceaccount.com",
			},
		},
		VM: VMConfig{
			Name:             "test-vm",
			MaxUptimeMinutes: 60,
			Network:          "my-vpc",
			Subnet:           "my-subnet",
		},
		SSH: SSHConfig{
			Port: 2222,
			User: "ubuntu",
		},
		Agents: AgentsConfig{
			DefaultCommand: "claude",
		},
		Provisioning: ProvisioningConfig{
			DotfilesURL: "https://example.com/install.sh",
		},
	}

	tests := []struct {
		key     string
		want    string
		wantErr bool
	}{
		{"cloud.provider", "gcp", false},
		{"cloud.gcp.project", "my-project", false},
		{"cloud.gcp.zone", "us-central1-a", false},
		{"cloud.gcp.service_account", "sa@my-project.iam.gserviceaccount.com", false},
		{"vm.name", "test-vm", false},
		{"vm.max_uptime_minutes", "60", false},
		{"vm.network", "my-vpc", false},
		{"vm.subnet", "my-subnet", false},
		{"ssh.port", "2222", false},
		{"ssh.user", "ubuntu", false},
		{"agents.default_command", "claude", false},
		{"provisioning.dotfiles_url", "https://example.com/install.sh", false},
		{"unknown.key", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got, err := cfg.GetValue(tt.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetValue() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("GetValue() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAllKeys(t *testing.T) {
	keys := AllKeys()
	if len(keys) == 0 {
		t.Error("AllKeys() returned empty list")
	}

	// Check that some expected keys are present
	expected := []string{"cloud.provider", "cloud.gcp.project", "cloud.gcp.service_account", "vm.name", "ssh.port"}
	for _, exp := range expected {
		found := false
		for _, k := range keys {
			if k == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("AllKeys() missing expected key %q", exp)
		}
	}
}

func TestExists(t *testing.T) {
	// This test is limited since Exists() uses DefaultConfigPath()
	// We can only verify it returns a boolean without error
	_ = Exists() // Should not panic
}

func TestLoadFile_AgentsHooks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cloudcoop.toml")

	content := `
[cloud]
provider = "gcp"

[cloud.gcp]
project = "test-project"
zone = "us-central1-a"

[vm]
name = "test-vm"

[agents]
default_command = "claude"
pre_commands = ["export BEADS_NO_DAEMON=1", "export FOO=bar"]

[agents.repos.acme-backend]
command = "claude"
pre_commands = ["nvm use 18"]

[agents.repos.acme-frontend]
command = "aider"
pre_commands = ["nvm use 20"]
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Agents.DefaultCommand != "claude" {
		t.Errorf("default_command = %q, want %q", cfg.Agents.DefaultCommand, "claude")
	}
	if len(cfg.Agents.PreCommands) != 2 {
		t.Fatalf("pre_commands len = %d, want 2", len(cfg.Agents.PreCommands))
	}
	if cfg.Agents.PreCommands[0] != "export BEADS_NO_DAEMON=1" {
		t.Errorf("pre_commands[0] = %q, want %q", cfg.Agents.PreCommands[0], "export BEADS_NO_DAEMON=1")
	}
	if cfg.Agents.PreCommands[1] != "export FOO=bar" {
		t.Errorf("pre_commands[1] = %q, want %q", cfg.Agents.PreCommands[1], "export FOO=bar")
	}

	if len(cfg.Agents.Repos) != 2 {
		t.Fatalf("repos len = %d, want 2", len(cfg.Agents.Repos))
	}

	backend := cfg.Agents.Repos["acme-backend"]
	if backend.Command != "claude" {
		t.Errorf("acme-backend command = %q, want %q", backend.Command, "claude")
	}
	if len(backend.PreCommands) != 1 || backend.PreCommands[0] != "nvm use 18" {
		t.Errorf("acme-backend pre_commands = %v, want [nvm use 18]", backend.PreCommands)
	}

	frontend := cfg.Agents.Repos["acme-frontend"]
	if frontend.Command != "aider" {
		t.Errorf("acme-frontend command = %q, want %q", frontend.Command, "aider")
	}
	if len(frontend.PreCommands) != 1 || frontend.PreCommands[0] != "nvm use 20" {
		t.Errorf("acme-frontend pre_commands = %v, want [nvm use 20]", frontend.PreCommands)
	}
}

func TestAgentsConfig_ResolveCommand(t *testing.T) {
	tests := []struct {
		name string
		cfg  AgentsConfig
		slug string
		want string
	}{
		{
			name: "no repos map returns default",
			cfg:  AgentsConfig{DefaultCommand: "claude"},
			slug: "backend",
			want: "claude",
		},
		{
			name: "repo-specific command",
			cfg: AgentsConfig{
				DefaultCommand: "claude",
				Repos: map[string]RepoConfig{
					"backend": {Command: "aider"},
				},
			},
			slug: "backend",
			want: "aider",
		},
		{
			name: "slug not in map falls back to default",
			cfg: AgentsConfig{
				DefaultCommand: "claude",
				Repos: map[string]RepoConfig{
					"frontend": {Command: "aider"},
				},
			},
			slug: "backend",
			want: "claude",
		},
		{
			name: "empty repo command falls back to default",
			cfg: AgentsConfig{
				DefaultCommand: "claude",
				Repos: map[string]RepoConfig{
					"backend": {Command: ""},
				},
			},
			slug: "backend",
			want: "claude",
		},
		{
			name: "empty default and no repo returns empty",
			cfg:  AgentsConfig{},
			slug: "backend",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.ResolveCommand(tt.slug)
			if got != tt.want {
				t.Errorf("ResolveCommand(%q) = %q, want %q", tt.slug, got, tt.want)
			}
		})
	}
}

func TestProjectConfigPath(t *testing.T) {
	got := ProjectConfigPath("/home/user/project")
	want := "/home/user/project/.cloudcoop/config.toml"
	if got != want {
		t.Errorf("ProjectConfigPath() = %q, want %q", got, want)
	}
}

func TestInstanceConfigPath(t *testing.T) {
	got := InstanceConfigPath("/home/user/project")
	want := "/home/user/project/.cloudcoop/local.toml"
	if got != want {
		t.Errorf("InstanceConfigPath() = %q, want %q", got, want)
	}
}

func TestSaveInstance(t *testing.T) {
	dir := t.TempDir()

	err := SaveInstance(dir, "my-project", "us-central1-a", "sa@my-project.iam.gserviceaccount.com", "my-vm", "my-vpc", "my-subnet")
	if err != nil {
		t.Fatalf("SaveInstance() error = %v", err)
	}

	// Verify writes to local.toml, not config.toml
	instancePath := InstanceConfigPath(dir)
	cfg, err := LoadFile(instancePath)
	if err != nil {
		t.Fatalf("LoadFile(instance) error = %v", err)
	}

	if cfg.Cloud.GCP.Project != "my-project" {
		t.Errorf("project = %q, want %q", cfg.Cloud.GCP.Project, "my-project")
	}
	if cfg.Cloud.GCP.Zone != "us-central1-a" {
		t.Errorf("zone = %q, want %q", cfg.Cloud.GCP.Zone, "us-central1-a")
	}
	if cfg.Cloud.GCP.ServiceAccount != "sa@my-project.iam.gserviceaccount.com" {
		t.Errorf("service_account = %q, want %q", cfg.Cloud.GCP.ServiceAccount, "sa@my-project.iam.gserviceaccount.com")
	}
	if cfg.VM.Name != "my-vm" {
		t.Errorf("vm.name = %q, want %q", cfg.VM.Name, "my-vm")
	}
	if cfg.VM.Network != "my-vpc" {
		t.Errorf("vm.network = %q, want %q", cfg.VM.Network, "my-vpc")
	}
	if cfg.VM.Subnet != "my-subnet" {
		t.Errorf("vm.subnet = %q, want %q", cfg.VM.Subnet, "my-subnet")
	}

	// Verify config.toml was NOT created
	projectPath := ProjectConfigPath(dir)
	if _, err := os.Stat(projectPath); !os.IsNotExist(err) {
		t.Error("SaveInstance() should not create config.toml")
	}

	// Verify .gitignore was created with local.toml entry
	gitignorePath := filepath.Join(dir, ".cloudcoop", ".gitignore")
	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("expected .gitignore to be created: %v", err)
	}
	if !strings.Contains(string(data), "local.toml") {
		t.Errorf(".gitignore does not contain 'local.toml': %q", string(data))
	}
}

func TestSaveProject_DelegatesToSaveInstance(t *testing.T) {
	dir := t.TempDir()

	err := SaveProject(dir, "my-project", "us-central1-a", "sa@my-project.iam.gserviceaccount.com", "my-vm", "default", "")
	if err != nil {
		t.Fatalf("SaveProject() error = %v", err)
	}

	// SaveProject now delegates to SaveInstance, so verify it writes to local.toml
	instancePath := InstanceConfigPath(dir)
	cfg, err := LoadFile(instancePath)
	if err != nil {
		t.Fatalf("LoadFile(instance) error = %v", err)
	}

	if cfg.Cloud.GCP.Project != "my-project" {
		t.Errorf("project = %q, want %q", cfg.Cloud.GCP.Project, "my-project")
	}
	if cfg.VM.Name != "my-vm" {
		t.Errorf("vm.name = %q, want %q", cfg.VM.Name, "my-vm")
	}
}

func TestEnsureGitignore_Idempotent(t *testing.T) {
	dir := t.TempDir()

	// Create the .cloudcoop directory
	if err := os.MkdirAll(filepath.Join(dir, ".cloudcoop"), 0o700); err != nil {
		t.Fatal(err)
	}

	// Call twice
	if err := ensureGitignore(dir); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if err := ensureGitignore(dir); err != nil {
		t.Fatalf("second call: %v", err)
	}

	// Verify single entry
	data, err := os.ReadFile(filepath.Join(dir, ".cloudcoop", ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	count := strings.Count(string(data), "local.toml")
	if count != 1 {
		t.Errorf("expected 1 'local.toml' entry, got %d in: %q", count, string(data))
	}
}

func TestEnsureGitignore_PreservesExisting(t *testing.T) {
	dir := t.TempDir()
	gitignoreDir := filepath.Join(dir, ".cloudcoop")
	if err := os.MkdirAll(gitignoreDir, 0o700); err != nil {
		t.Fatal(err)
	}

	// Write pre-existing content
	existing := "*.bak\nsecrets.toml\n"
	if err := os.WriteFile(filepath.Join(gitignoreDir, ".gitignore"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := ensureGitignore(dir); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(gitignoreDir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)
	if !strings.Contains(content, "*.bak") {
		t.Error("pre-existing '*.bak' entry was lost")
	}
	if !strings.Contains(content, "secrets.toml") {
		t.Error("pre-existing 'secrets.toml' entry was lost")
	}
	if !strings.Contains(content, "local.toml") {
		t.Error("'local.toml' entry was not added")
	}
}

func TestMergeConfig(t *testing.T) {
	dst := &Config{
		Cloud: CloudConfig{
			Provider: "gcp",
			GCP: GCPConfig{
				Zone: "us-central1-a",
			},
		},
		VM: VMConfig{
			Name:       "default-vm",
			DiskSizeGB: 50,
		},
		SSH: SSHConfig{
			Port: 22,
		},
	}

	src := &Config{
		Cloud: CloudConfig{
			GCP: GCPConfig{
				Project:        "my-project",
				Zone:           "europe-north2-a", // Override
				ServiceAccount: "sa@my-project.iam.gserviceaccount.com",
			},
		},
		VM: VMConfig{
			Name: "my-vm", // Override
		},
	}

	mergeConfig(dst, src)

	// Overridden values
	if dst.Cloud.GCP.Project != "my-project" {
		t.Errorf("project = %q, want %q", dst.Cloud.GCP.Project, "my-project")
	}
	if dst.Cloud.GCP.Zone != "europe-north2-a" {
		t.Errorf("zone = %q, want %q", dst.Cloud.GCP.Zone, "europe-north2-a")
	}
	if dst.Cloud.GCP.ServiceAccount != "sa@my-project.iam.gserviceaccount.com" {
		t.Errorf("service_account = %q, want %q", dst.Cloud.GCP.ServiceAccount, "sa@my-project.iam.gserviceaccount.com")
	}
	if dst.VM.Name != "my-vm" {
		t.Errorf("vm.name = %q, want %q", dst.VM.Name, "my-vm")
	}

	// Preserved values (not in src)
	if dst.Cloud.Provider != "gcp" {
		t.Errorf("provider = %q, want %q", dst.Cloud.Provider, "gcp")
	}
	if dst.VM.DiskSizeGB != 50 {
		t.Errorf("disk_size_gb = %d, want 50", dst.VM.DiskSizeGB)
	}
	if dst.SSH.Port != 22 {
		t.Errorf("ssh.port = %d, want 22", dst.SSH.Port)
	}
}

func TestMergeConfig_EmptySrc(t *testing.T) {
	dst := &Config{
		Cloud: CloudConfig{
			Provider: "gcp",
			GCP: GCPConfig{
				Project: "original",
				Zone:    "us-central1-a",
			},
		},
	}

	src := &Config{}
	mergeConfig(dst, src)

	// Nothing should change
	if dst.Cloud.Provider != "gcp" {
		t.Errorf("provider = %q, want %q", dst.Cloud.Provider, "gcp")
	}
	if dst.Cloud.GCP.Project != "original" {
		t.Errorf("project = %q, want %q", dst.Cloud.GCP.Project, "original")
	}
}

func TestSaveInstance_CleanOutput(t *testing.T) {
	dir := t.TempDir()

	err := SaveInstance(dir, "my-project", "us-central1-a", "sa@my-project.iam.gserviceaccount.com", "my-vm", "default", "")
	if err != nil {
		t.Fatalf("SaveInstance() error = %v", err)
	}

	data, err := os.ReadFile(InstanceConfigPath(dir))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	content := string(data)

	// Should contain the fields we set
	if !strings.Contains(content, `project = "my-project"`) {
		t.Errorf("expected project in output:\n%s", content)
	}
	if !strings.Contains(content, `zone = "us-central1-a"`) {
		t.Errorf("expected zone in output:\n%s", content)
	}
	if !strings.Contains(content, `name = "my-vm"`) {
		t.Errorf("expected vm name in output:\n%s", content)
	}

	// Should NOT contain zero-value fields
	if strings.Contains(content, "port") {
		t.Errorf("should not contain port (zero value):\n%s", content)
	}
	if strings.Contains(content, "disk_size_gb") {
		t.Errorf("should not contain disk_size_gb (zero value):\n%s", content)
	}
	if strings.Contains(content, "[ssh]") {
		t.Errorf("should not contain [ssh] section (empty):\n%s", content)
	}
	if strings.Contains(content, "[setup]") {
		t.Errorf("should not contain [setup] section (empty):\n%s", content)
	}
	if strings.Contains(content, "[agents]") {
		t.Errorf("should not contain [agents] section (empty):\n%s", content)
	}
	if strings.Contains(content, "[provisioning]") {
		t.Errorf("should not contain [provisioning] section (empty):\n%s", content)
	}
	if strings.Contains(content, "[tui]") {
		t.Errorf("should not contain [tui] section (empty):\n%s", content)
	}
}

func TestAgentsConfig_ResolvePreCommands(t *testing.T) {
	tests := []struct {
		name string
		cfg  AgentsConfig
		slug string
		want []string
	}{
		{
			name: "global only",
			cfg:  AgentsConfig{PreCommands: []string{"export A=1"}},
			slug: "backend",
			want: []string{"export A=1"},
		},
		{
			name: "repo only",
			cfg: AgentsConfig{
				Repos: map[string]RepoConfig{
					"backend": {PreCommands: []string{"nvm use 18"}},
				},
			},
			slug: "backend",
			want: []string{"nvm use 18"},
		},
		{
			name: "both global and repo",
			cfg: AgentsConfig{
				PreCommands: []string{"export A=1"},
				Repos: map[string]RepoConfig{
					"backend": {PreCommands: []string{"nvm use 18"}},
				},
			},
			slug: "backend",
			want: []string{"export A=1", "nvm use 18"},
		},
		{
			name: "neither",
			cfg:  AgentsConfig{},
			slug: "backend",
			want: nil,
		},
		{
			name: "slug not in map returns global only",
			cfg: AgentsConfig{
				PreCommands: []string{"export A=1"},
				Repos: map[string]RepoConfig{
					"frontend": {PreCommands: []string{"nvm use 20"}},
				},
			},
			slug: "backend",
			want: []string{"export A=1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.ResolvePreCommands(tt.slug)
			if len(got) != len(tt.want) {
				t.Fatalf("ResolvePreCommands(%q) len = %d, want %d\ngot: %v", tt.slug, len(got), len(tt.want), got)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("ResolvePreCommands(%q)[%d] = %q, want %q", tt.slug, i, got[i], tt.want[i])
				}
			}
		})
	}
}
