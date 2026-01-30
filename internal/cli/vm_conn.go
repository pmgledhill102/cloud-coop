package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/cloud-coop/cloudcoop/internal/cloud"
	"github.com/cloud-coop/cloudcoop/internal/config"
	"github.com/cloud-coop/cloudcoop/internal/log"
	"github.com/cloud-coop/cloudcoop/internal/ssh"
)

// sshClientFactory creates an SSH client from a config.
// This is a package-level variable to allow test injection.
var sshClientFactory func(cfg ssh.Config) (ssh.Runner, error) = defaultSSHClientFactory

func defaultSSHClientFactory(cfg ssh.Config) (ssh.Runner, error) {
	return ssh.NewClient(cfg)
}

// vmConn holds a resolved VM connection for CLI commands.
type vmConn struct {
	Config *config.Config
	Client ssh.Runner
	IP     string
	User   string
	Port   int
	VMInfo *cloud.VMInfo
}

// Close releases the SSH client connection.
func (c *vmConn) Close() {
	if c.Client != nil {
		_ = c.Client.Close()
	}
}

// connectToVM handles the common CLI boilerplate: load config, create provider,
// get VM info, check VM is running, resolve IP, and create SSH client.
// The caller must call conn.Close() when done with the connection.
func connectToVM(cmd *cobra.Command) (*vmConn, error) {
	cfg, err := configLoader()
	if err != nil {
		return nil, handleConfigError(err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, handleConfigError(fmt.Errorf("invalid configuration: %w", err))
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
	defer cancel()

	provider, cleanup, err := createProvider(ctx, cfg)
	if err != nil {
		return nil, handleProviderError(err)
	}
	defer cleanup()

	log.Debug("querying VM status", "name", cfg.VM.Name, "provider", provider.Name())
	vmInfo, err := provider.GetVMInfo(ctx, cfg.VM.Name)
	if err != nil {
		return nil, fmt.Errorf("get VM status: %w", err)
	}

	if vmInfo.Status == cloud.VMStatusNotFound {
		fmt.Fprintln(os.Stderr, "VM not found:", cfg.VM.Name)
		return nil, nil
	}
	if vmInfo.Status != cloud.VMStatusRunning {
		fmt.Fprintf(os.Stderr, "VM is %s (must be running)\n", vmInfo.Status)
		return nil, nil
	}

	ip, err := ssh.ResolveVMIP(vmInfo.ExternalIP, vmInfo.InternalIP)
	if err != nil {
		fmt.Fprintln(os.Stderr, "VM has no IP address available for SSH connection")
		return nil, nil
	}

	sshUser := ssh.ResolveSSHUser(cfg.SSH.User)
	sshPort := ssh.ResolvePort(cfg.SSH.Port)
	log.Debug("connecting to VM via SSH", "host", ip, "user", sshUser, "port", sshPort)

	sshCfg := ssh.SetupClientConfig(ip, sshUser, cfg.SSH.Port)
	sshCfg.VM = ssh.NewVMIdentity(vmInfo.Name, vmInfo.CloudcoopCreated)
	client, err := sshClientFactory(sshCfg)
	if err != nil {
		return nil, handleSSHError(err, ip, sshPort)
	}

	return &vmConn{
		Config: cfg,
		Client: client,
		IP:     ip,
		User:   sshUser,
		Port:   sshPort,
		VMInfo: vmInfo,
	}, nil
}
