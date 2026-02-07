package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"

	"github.com/cloud-coop/cloudcoop/internal/config"
	"github.com/cloud-coop/cloudcoop/internal/setup"
)

// mockSetupProvider implements setup.SetupProvider for testing.
type mockSetupProvider struct {
	projects      []setup.ProjectInfo
	projectsErr   error
	apiStatuses   []setup.APIStatus
	apiErr        error
	enableAPIErr  error
	saExists      bool
	saExistsErr   error
	createSAEmail string
	createSAErr   error
	iamBindings   map[string]bool
	iamCheckErr   error
	grantIAMErr   error
	fwExists      bool
	fwExistsErr   error
	createFWErr   error
	adcErr        error
	enabledAPIs   []string
	grantedRoles  []string
	createdSA     bool
	createdFW     bool
}

func (m *mockSetupProvider) ListProjects(ctx context.Context) ([]setup.ProjectInfo, error) {
	return m.projects, m.projectsErr
}

func (m *mockSetupProvider) CheckAPIs(ctx context.Context, project string) ([]setup.APIStatus, error) {
	if m.apiErr != nil {
		return nil, m.apiErr
	}
	if m.apiStatuses != nil {
		return m.apiStatuses, nil
	}
	// Default: all enabled
	var statuses []setup.APIStatus
	for _, api := range setup.RequiredAPIs {
		statuses = append(statuses, setup.APIStatus{Name: api, Enabled: true})
	}
	return statuses, nil
}

func (m *mockSetupProvider) EnableAPI(ctx context.Context, project, api string) error {
	if m.enableAPIErr != nil {
		return m.enableAPIErr
	}
	m.enabledAPIs = append(m.enabledAPIs, api)
	return nil
}

func (m *mockSetupProvider) ServiceAccountExists(ctx context.Context, project, name string) (bool, error) {
	return m.saExists, m.saExistsErr
}

func (m *mockSetupProvider) CreateServiceAccount(ctx context.Context, project, name, displayName string) (string, error) {
	if m.createSAErr != nil {
		return "", m.createSAErr
	}
	m.createdSA = true
	if m.createSAEmail != "" {
		return m.createSAEmail, nil
	}
	return name + "@" + project + ".iam.gserviceaccount.com", nil
}

func (m *mockSetupProvider) CheckIAMBinding(ctx context.Context, project, member, role string) (bool, error) {
	if m.iamCheckErr != nil {
		return false, m.iamCheckErr
	}
	if m.iamBindings != nil {
		return m.iamBindings[role], nil
	}
	return true, nil
}

func (m *mockSetupProvider) GrantIAMRole(ctx context.Context, project, member, role string) error {
	if m.grantIAMErr != nil {
		return m.grantIAMErr
	}
	m.grantedRoles = append(m.grantedRoles, role)
	return nil
}

func (m *mockSetupProvider) FirewallRuleExists(ctx context.Context, project, name string) (bool, error) {
	return m.fwExists, m.fwExistsErr
}

func (m *mockSetupProvider) CreateIAPFirewallRule(ctx context.Context, project, network string) error {
	if m.createFWErr != nil {
		return m.createFWErr
	}
	m.createdFW = true
	return nil
}

func (m *mockSetupProvider) CheckADCCredentials(ctx context.Context) error {
	return m.adcErr
}

func (m *mockSetupProvider) Close() error { return nil }

// withMockSetupProvider injects a mock setup provider and SSH key check, returns cleanup.
func withMockSetupProvider(mock *mockSetupProvider) func() {
	origProvider := setupProviderFactory
	origSSH := sshKeyChecker
	origGen := sshKeyGenerator

	setupProviderFactory = func(ctx context.Context) (setup.SetupProvider, error) {
		return mock, nil
	}
	sshKeyChecker = func() setup.PrereqStatus {
		return setup.PrereqStatus{Name: "SSH key", OK: true, Detail: "~/.ssh/id_ed25519.pub"}
	}
	sshKeyGenerator = func() (string, error) {
		return "~/.ssh/id_ed25519.pub", nil
	}

	return func() {
		setupProviderFactory = origProvider
		sshKeyChecker = origSSH
		sshKeyGenerator = origGen
	}
}

func TestRunSetup_AllGreen_DryRun(t *testing.T) {
	// All resources exist, dry run
	mock := &mockSetupProvider{
		saExists: true,
		fwExists: true,
	}
	cleanup := withMockSetupProvider(mock)
	defer cleanup()

	// Create temp dir for project config
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.Flags().String("project", "test-project", "")
	cmd.Flags().String("zone", "us-central1-a", "")
	cmd.Flags().Bool("dry-run", true, "")

	err := runSetup(cmd, []string{})
	if err != nil {
		t.Errorf("runSetup() error = %v", err)
	}
}

func TestRunSetup_NeedsEverything_DryRun(t *testing.T) {
	mock := &mockSetupProvider{
		saExists: false,
		fwExists: false,
		apiStatuses: []setup.APIStatus{
			{Name: "compute.googleapis.com", Enabled: false},
			{Name: "iam.googleapis.com", Enabled: false},
			{Name: "logging.googleapis.com", Enabled: false},
			{Name: "monitoring.googleapis.com", Enabled: false},
		},
	}
	cleanup := withMockSetupProvider(mock)
	defer cleanup()

	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.Flags().String("project", "test-project", "")
	cmd.Flags().String("zone", "us-central1-a", "")
	cmd.Flags().Bool("dry-run", true, "")

	err := runSetup(cmd, []string{})
	if err != nil {
		t.Errorf("runSetup() error = %v", err)
	}
}

func TestRunSetup_ProviderCreationFailure(t *testing.T) {
	// Simulate ADC failure
	origProvider := setupProviderFactory
	origSSH := sshKeyChecker
	origGen := sshKeyGenerator
	setupProviderFactory = func(ctx context.Context) (setup.SetupProvider, error) {
		return nil, errors.New("could not find default credentials")
	}
	sshKeyChecker = func() setup.PrereqStatus {
		return setup.PrereqStatus{Name: "SSH key", OK: true, Detail: "~/.ssh/id_ed25519.pub"}
	}
	sshKeyGenerator = func() (string, error) {
		return "~/.ssh/id_ed25519.pub", nil
	}
	defer func() {
		setupProviderFactory = origProvider
		sshKeyChecker = origSSH
		sshKeyGenerator = origGen
	}()

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.Flags().String("project", "", "")
	cmd.Flags().String("zone", "", "")
	cmd.Flags().Bool("dry-run", false, "")

	// Should handle gracefully
	err := runSetup(cmd, []string{})
	if err != nil {
		t.Errorf("runSetup() should handle ADC failure gracefully, got: %v", err)
	}
}

func TestRunSetup_ExistingProjectConfig(t *testing.T) {
	mock := &mockSetupProvider{
		saExists: true,
		fwExists: true,
	}
	cleanup := withMockSetupProvider(mock)
	defer cleanup()

	// Create temp dir with existing project config
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	// Write existing config
	cfgDir := filepath.Join(dir, ".cloudcoop")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatal(err)
	}
	existingCfg := &config.Config{
		Cloud: config.CloudConfig{
			GCP: config.GCPConfig{
				Project:        "existing-project",
				Zone:           "europe-north2-a",
				ServiceAccount: "cloudcoop-vm@existing-project.iam.gserviceaccount.com",
			},
		},
		VM: config.VMConfig{Name: "my-vm"},
	}
	if err := existingCfg.Save(filepath.Join(cfgDir, "config.toml")); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.Flags().String("project", "", "")
	cmd.Flags().String("zone", "", "")
	cmd.Flags().Bool("dry-run", true, "")

	err := runSetup(cmd, []string{})
	if err != nil {
		t.Errorf("runSetup() error = %v", err)
	}
}

func TestRunSetup_SSHKeyAutoGenerated(t *testing.T) {
	mock := &mockSetupProvider{
		saExists: true,
		fwExists: true,
	}
	origProvider := setupProviderFactory
	origSSH := sshKeyChecker
	origGen := sshKeyGenerator

	setupProviderFactory = func(ctx context.Context) (setup.SetupProvider, error) {
		return mock, nil
	}
	// Simulate no SSH key found
	sshKeyChecker = func() setup.PrereqStatus {
		return setup.PrereqStatus{
			Name:    "SSH key",
			OK:      false,
			Detail:  "no SSH key found in ~/.ssh",
			HelpMsg: "Generate an SSH key with: ssh-keygen -t ed25519",
		}
	}
	var generated bool
	sshKeyGenerator = func() (string, error) {
		generated = true
		return "~/.ssh/id_ed25519.pub", nil
	}
	defer func() {
		setupProviderFactory = origProvider
		sshKeyChecker = origSSH
		sshKeyGenerator = origGen
	}()

	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.Flags().String("project", "test-project", "")
	cmd.Flags().String("zone", "us-central1-a", "")
	cmd.Flags().Bool("dry-run", true, "")

	err := runSetup(cmd, []string{})
	if err != nil {
		t.Errorf("runSetup() error = %v", err)
	}
	if !generated {
		t.Error("expected SSH key to be auto-generated")
	}
}

func TestRunSetup_SSHKeyGenerationFailure(t *testing.T) {
	origProvider := setupProviderFactory
	origSSH := sshKeyChecker
	origGen := sshKeyGenerator

	setupProviderFactory = func(ctx context.Context) (setup.SetupProvider, error) {
		return &mockSetupProvider{}, nil
	}
	sshKeyChecker = func() setup.PrereqStatus {
		return setup.PrereqStatus{
			Name:    "SSH key",
			OK:      false,
			Detail:  "no SSH key found in ~/.ssh",
			HelpMsg: "Generate an SSH key with: ssh-keygen -t ed25519",
		}
	}
	sshKeyGenerator = func() (string, error) {
		return "", errors.New("permission denied")
	}
	defer func() {
		setupProviderFactory = origProvider
		sshKeyChecker = origSSH
		sshKeyGenerator = origGen
	}()

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.Flags().String("project", "test-project", "")
	cmd.Flags().String("zone", "us-central1-a", "")
	cmd.Flags().Bool("dry-run", false, "")

	err := runSetup(cmd, []string{})
	if err == nil {
		t.Error("expected error when SSH key generation fails")
	}
}

func TestRunSetup_CheckAPIsError(t *testing.T) {
	mock := &mockSetupProvider{
		apiErr: errors.New("permission denied"),
	}
	cleanup := withMockSetupProvider(mock)
	defer cleanup()

	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.Flags().String("project", "test-project", "")
	cmd.Flags().String("zone", "us-central1-a", "")
	cmd.Flags().Bool("dry-run", false, "")

	err := runSetup(cmd, []string{})
	if err == nil {
		t.Error("expected error for CheckAPIs failure")
	}
}
