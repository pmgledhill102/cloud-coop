package ops

import (
	"fmt"

	"github.com/cloud-coop/cloudcoop/internal/cloud"
	"github.com/cloud-coop/cloudcoop/internal/config"
	"github.com/cloud-coop/cloudcoop/internal/ssh"
)

// SSHClientFactory creates an SSH client from a config. Replaceable for testing.
var SSHClientFactory func(cfg ssh.Config) (ssh.Runner, error) = defaultSSHClientFactory

func defaultSSHClientFactory(cfg ssh.Config) (ssh.Runner, error) {
	return ssh.NewClient(cfg)
}

// ConnectSSH creates an SSH client for a running VM.
func ConnectSSH(cfg *config.Config, vmInfo *cloud.VMInfo) (ssh.Runner, error) {
	ip, err := ssh.ResolveVMIP(vmInfo.ExternalIP, vmInfo.InternalIP)
	if err != nil {
		return nil, fmt.Errorf("no IP address available")
	}

	sshUser := ssh.ResolveSSHUser(cfg.SSH.User)
	sshCfg := ssh.SetupClientConfig(ip, sshUser, cfg.SSH.Port)
	sshCfg.VM = ssh.NewVMIdentity(vmInfo.Name, vmInfo.CloudcoopCreated)

	return SSHClientFactory(sshCfg)
}
