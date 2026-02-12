//go:build integration

package integration

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"os"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/cloud-coop/cloudcoop/internal/cloud"
	"github.com/cloud-coop/cloudcoop/internal/cloud/gcp"
	"github.com/cloud-coop/cloudcoop/internal/config"
	sshpkg "github.com/cloud-coop/cloudcoop/internal/ssh"
)

// testEnv holds shared state for integration tests.
type testEnv struct {
	cfg      *config.Config
	provider *gcp.Provider

	vmName     string
	vmInfo     *cloud.VMInfo
	projectID  string
	zone       string
	sshUser    string
	sshPubKey  string
	sshPrivKey []byte
}

// env is the shared test environment. Initialized in TestMain.
var env *testEnv

// setupEnv creates the test environment from environment variables.
func setupEnv() *testEnv {
	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if projectID == "" {
		fmt.Fprintln(os.Stderr, "GOOGLE_CLOUD_PROJECT not set, skipping integration tests")
		os.Exit(0)
	}

	zone := os.Getenv("GOOGLE_CLOUD_ZONE")
	if zone == "" {
		zone = "us-central1-a"
	}

	// Generate unique VM name from Unix timestamp
	vmName := fmt.Sprintf("cc-inttest-%d", time.Now().Unix())

	// Generate ephemeral SSH key pair for the test
	pubKey, privKey, err := generateSSHKeyPair()
	if err != nil {
		fmt.Fprintf(os.Stderr, "generate SSH key pair: %v\n", err)
		os.Exit(1)
	}

	cfg := &config.Config{
		Cloud: config.CloudConfig{
			Provider: "gcp",
			GCP: config.GCPConfig{
				Project:        projectID,
				Zone:           zone,
				ServiceAccount: os.Getenv("GCP_SERVICE_ACCOUNT"),
			},
		},
		VM: config.VMConfig{
			Name:             vmName,
			DiskSizeGB:       30,
			Image:            "projects/ubuntu-os-cloud/global/images/family/ubuntu-2510-arm64",
			Spot:             true,
			MaxUptimeMinutes: 60,
			Network:          "default",
		},
		SSH: config.SSHConfig{
			Port: 22,
			User: "integration",
		},
		Provisioning: config.ProvisioningConfig{
			ScriptURL: os.Getenv("PROVISION_SCRIPT_URL"),
		},
	}

	// Use default provisioning URL if not overridden
	if cfg.Provisioning.ScriptURL == "" {
		cfg.Provisioning.ScriptURL = "https://raw.githubusercontent.com/pmgledhill102/cloud-coop/main/scripts/provision-vm.sh"
	}

	// Use default service account if not specified
	if cfg.Cloud.GCP.ServiceAccount == "" {
		cfg.Cloud.GCP.ServiceAccount = fmt.Sprintf("cc-integration-test@%s.iam.gserviceaccount.com", projectID)
	}

	return &testEnv{
		cfg:        cfg,
		vmName:     vmName,
		projectID:  projectID,
		zone:       zone,
		sshUser:    "integration",
		sshPubKey:  pubKey,
		sshPrivKey: privKey,
	}
}

// initProvider creates the GCP provider. Called after setupEnv.
func (e *testEnv) initProvider(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	p, err := gcp.New(ctx, e.projectID, e.zone)
	if err != nil {
		t.Fatalf("create GCP provider: %v", err)
	}
	e.provider = p
}

// cleanup deletes the test VM and firewall rules. Always called, even on failure.
func (e *testEnv) cleanup() {
	if e.provider == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Delete VM (ignore errors — it may not exist)
	fmt.Printf("Cleanup: deleting VM %s...\n", e.vmName)
	if err := e.provider.DeleteVM(ctx, e.vmName); err != nil {
		fmt.Printf("Cleanup: VM delete: %v (may not exist)\n", err)
	} else {
		fmt.Printf("Cleanup: VM %s deleted\n", e.vmName)
	}

	// Close provider clients
	_ = e.provider.Close()
}

// connectSSH creates an SSH client to the test VM.
func (e *testEnv) connectSSH(t *testing.T) *sshpkg.Client {
	t.Helper()

	if e.vmInfo == nil {
		t.Fatal("vmInfo is nil — VM not created yet")
	}

	ip, err := sshpkg.ResolveVMIP(e.vmInfo.ExternalIP, e.vmInfo.InternalIP)
	if err != nil {
		t.Fatalf("resolve VM IP: %v", err)
	}

	sshCfg := sshpkg.SetupClientConfig(ip, e.sshUser, e.cfg.SSH.Port)
	sshCfg.VM = sshpkg.NewVMIdentity(e.vmInfo.Name, e.vmInfo.CloudcoopCreated)

	client, err := sshpkg.NewClient(sshCfg)
	if err != nil {
		t.Fatalf("SSH connect: %v", err)
	}
	return client
}

// refreshVMInfo updates the cached VM info.
func (e *testEnv) refreshVMInfo(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	info, err := e.provider.GetVMInfo(ctx, e.vmName)
	if err != nil {
		t.Fatalf("get VM info: %v", err)
	}
	e.vmInfo = info
}

// waitForStatus polls until the VM reaches the target status or times out.
func (e *testEnv) waitForStatus(t *testing.T, target cloud.VMStatus, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		info, err := e.provider.GetVMInfo(ctx, e.vmName)
		cancel()
		if err != nil {
			t.Logf("poll VM status: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}
		if info.Status == target {
			e.vmInfo = info
			return
		}
		t.Logf("VM status: %s (waiting for %s)", info.Status, target)
		time.Sleep(5 * time.Second)
	}
	t.Fatalf("VM did not reach status %s within %v", target, timeout)
}

// generateSSHKeyPair generates an ephemeral ed25519 key pair for testing.
// Returns the public key in OpenSSH format and the private key in PEM format.
func generateSSHKeyPair() (pubKeyStr string, privKeyPEM []byte, err error) {
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", nil, fmt.Errorf("generate ed25519 key: %w", err)
	}

	sshPub, err := ssh.NewPublicKey(pubKey)
	if err != nil {
		return "", nil, fmt.Errorf("convert to SSH public key: %w", err)
	}
	pubKeyStr = string(ssh.MarshalAuthorizedKey(sshPub))

	privPEM, err := ssh.MarshalPrivateKey(privKey, "")
	if err != nil {
		return "", nil, fmt.Errorf("marshal private key: %w", err)
	}
	privKeyPEM = pem.EncodeToMemory(privPEM)

	return pubKeyStr, privKeyPEM, nil
}
