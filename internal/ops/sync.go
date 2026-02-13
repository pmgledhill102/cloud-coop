package ops

import (
	"errors"
	"fmt"

	"github.com/cloud-coop/cloudcoop/internal/config"
	"github.com/cloud-coop/cloudcoop/internal/deploykey"
	"github.com/cloud-coop/cloudcoop/internal/log"
	"github.com/cloud-coop/cloudcoop/internal/ssh"
	"github.com/cloud-coop/cloudcoop/internal/workspace"
)

// DeployKeyPreflightError indicates the deploy key verification step failed.
type DeployKeyPreflightError struct {
	VerifyError string
}

func (e *DeployKeyPreflightError) Error() string {
	return fmt.Sprintf("deploy key verification failed: %s", e.VerifyError)
}

// DeployKeyManualError indicates the deploy key requires manual setup.
type DeployKeyManualError struct {
	Message string
}

func (e *DeployKeyManualError) Error() string {
	return e.Message
}

// SyncWorkspace runs the full workspace sync sequence: deploy key setup,
// git identity, and workspace sync. The commandOverride parameter allows
// callers to override the agent command from config (pass "" to use config).
func SyncWorkspace(
	client ssh.Runner,
	cfg *config.Config,
	wsInfo *workspace.Info,
	commandOverride string,
) (*workspace.SyncResult, error) {
	// 1. Deploy key setup
	repo, err := deploykey.ParseRepoRef(wsInfo.RemoteURL)
	if err != nil {
		return nil, fmt.Errorf("parse repo: %w", err)
	}

	fs := deploykey.NewFileSystem()
	cmd := deploykey.NewCommandRunner()
	dkOpts := deploykey.Options{
		Slug:      wsInfo.Slug,
		RemoteURL: wsInfo.RemoteURL,
		Repo:      repo,
	}

	setupResult, err := deploykey.EnsureKey(fs, cmd, dkOpts)
	if err != nil {
		return nil, fmt.Errorf("deploy key: %w", err)
	}
	if setupResult.ManualNeeded {
		return nil, &DeployKeyManualError{Message: setupResult.ManualMessage}
	}

	vmSetup, err := deploykey.SetupVM(client, fs, setupResult.KeyPair, dkOpts)
	if err != nil {
		if errors.Is(err, deploykey.ErrPreflightFailed) && vmSetup != nil {
			return nil, &DeployKeyPreflightError{VerifyError: vmSetup.VerifyError}
		}
		return nil, fmt.Errorf("VM key setup: %w", err)
	}

	// 2. Git identity: copy local user.name/user.email to VM
	gitID, ok := workspace.LocalGitIdentity(workspace.NewGitRunner("."))
	if ok {
		if err := workspace.SetupVMGitIdentity(client, gitID); err != nil {
			return nil, fmt.Errorf("git identity setup: %w", err)
		}
		log.Debug("set VM git identity", "name", gitID.Name, "email", gitID.Email)
	}

	// 3. Resolve agent command and pre-commands
	agentCommand := commandOverride
	if agentCommand == "" {
		agentCommand = cfg.Agents.ResolveCommand(wsInfo.Slug)
	}
	preCommands := cfg.Agents.ResolvePreCommands(wsInfo.Slug)

	// 4. Sync
	return workspace.Sync(client, wsInfo, workspace.SyncOptions{
		AgentCommand: agentCommand,
		PreCommands:  preCommands,
		RepoOwner:    repo.Owner,
		RepoName:     repo.Name,
	})
}
