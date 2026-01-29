package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/cloud-coop/cloudcoop/internal/cloud"
	"github.com/cloud-coop/cloudcoop/internal/deploykey"
	"github.com/cloud-coop/cloudcoop/internal/log"
	"github.com/cloud-coop/cloudcoop/internal/ssh"
	"github.com/cloud-coop/cloudcoop/internal/workspace"
)

var agentsSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync local worktrees to the VM",
	Long: `Sync local git worktrees to the cloud VM.

Reads the local repository's worktrees, clones a bare repo on the VM if needed,
creates matching worktrees, and starts tmux windows for each.

Run from within a git repository with an origin remote configured.

Examples:
  cloudcoop agents sync                    # sync current repo
  cloudcoop agents sync --command claude   # override agent command`,
	RunE: runAgentsSync,
}

var syncCommand string

func init() {
	agentsSyncCmd.Flags().StringVar(&syncCommand, "command", "", "command to run in tmux windows")
}

func runAgentsSync(cmd *cobra.Command, args []string) error {
	// 1. Detect workspace.
	info, err := workspace.Detect(workspace.NewGitRunner("."))
	if err != nil {
		if errors.Is(err, workspace.ErrNotGitRepo) {
			fmt.Fprintln(os.Stderr, "Must run from a git repository.")
			return nil
		}
		if errors.Is(err, workspace.ErrNoRemote) {
			fmt.Fprintln(os.Stderr, "No origin remote configured.")
			return nil
		}
		return fmt.Errorf("detect workspace: %w", err)
	}

	log.Debug("detected workspace", "slug", info.Slug, "worktrees", len(info.Worktrees))

	// 2. Standard VM connection.
	cfg, err := configLoader()
	if err != nil {
		return handleConfigError(err)
	}
	if err := cfg.Validate(); err != nil {
		return handleConfigError(fmt.Errorf("invalid configuration: %w", err))
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
	defer cancel()

	provider, cleanup, err := createProvider(ctx, cfg)
	if err != nil {
		return handleProviderError(err)
	}
	defer cleanup()

	log.Debug("querying VM status", "name", cfg.VM.Name, "provider", provider.Name())
	vmInfo, err := provider.GetVMInfo(ctx, cfg.VM.Name)
	if err != nil {
		return fmt.Errorf("get VM status: %w", err)
	}

	if vmInfo.Status == cloud.VMStatusNotFound {
		fmt.Fprintln(os.Stderr, "VM not found:", cfg.VM.Name)
		return nil
	}
	if vmInfo.Status != cloud.VMStatusRunning {
		fmt.Fprintf(os.Stderr, "VM is %s (must be running to sync)\n", vmInfo.Status)
		return nil
	}

	ip, err := ssh.ResolveVMIP(vmInfo.ExternalIP, vmInfo.InternalIP)
	if err != nil {
		fmt.Fprintln(os.Stderr, "VM has no IP address available for SSH connection")
		return nil
	}

	sshUser := ssh.ResolveSSHUser(cfg.SSH.User)
	log.Debug("connecting to VM via SSH", "host", ip, "user", sshUser, "port", cfg.SSH.Port)

	client, err := ssh.NewClient(ssh.SetupClientConfig(ip, sshUser, cfg.SSH.Port))
	if err != nil {
		return fmt.Errorf("SSH connection failed: %w", err)
	}
	defer func() { _ = client.Close() }()

	// 3. Deploy key setup.
	repo, err := deploykey.ParseRepoRef(info.RemoteURL)
	if err != nil {
		return fmt.Errorf("parse remote URL: %w", err)
	}

	fs := deploykey.NewFileSystem()
	cmdRunner := deploykey.NewCommandRunner()
	dkOpts := deploykey.Options{
		Slug:      info.Slug,
		RemoteURL: info.RemoteURL,
		Repo:      repo,
	}

	setupResult, err := deploykey.EnsureKey(fs, cmdRunner, dkOpts)
	if err != nil {
		return fmt.Errorf("deploy key setup: %w", err)
	}

	if setupResult.ManualNeeded {
		fmt.Fprintln(os.Stderr, setupResult.ManualMessage)
		return nil
	}

	vmSetup, err := deploykey.SetupVM(client, fs, setupResult.KeyPair, dkOpts)
	if err != nil {
		if errors.Is(err, deploykey.ErrPreflightFailed) {
			fmt.Fprintf(os.Stderr, "Deploy key verification failed: %s\n", vmSetup.VerifyError)
			return nil
		}
		return fmt.Errorf("deploy key VM setup: %w", err)
	}

	// 4. Resolve agent command: flag > repo-specific > default > "" (sync defaults to "bash").
	agentCommand := syncCommand
	if agentCommand == "" {
		agentCommand = cfg.Agents.ResolveCommand(info.Slug)
	}

	// 5. Resolve pre-commands: global + repo-specific.
	preCommands := cfg.Agents.ResolvePreCommands(info.Slug)

	// 6. Sync.
	syncResult, err := workspace.Sync(client, info, workspace.SyncOptions{
		AgentCommand: agentCommand,
		PreCommands:  preCommands,
		RepoOwner:    repo.Owner,
		RepoName:     repo.Name,
	})
	if err != nil {
		return fmt.Errorf("sync: %w", err)
	}

	// 7. Print results.
	printSyncResult(syncResult)
	return nil
}

func printSyncResult(r *workspace.SyncResult) {
	fmt.Printf("Repository: %s\n", r.Slug)
	fmt.Println()

	if r.Cloned {
		fmt.Println("Bare clone: created")
	} else {
		fmt.Println("Bare clone: exists")
	}
	if r.Fetched {
		fmt.Println("Fetched latest")
	}
	fmt.Println()

	fmt.Println("Worktrees:")
	for _, name := range r.WorktreesCreated {
		fmt.Printf("  + %-20s (created)\n", name)
	}
	for _, name := range r.WorktreesSkipped {
		fmt.Printf("  = %-20s (exists)\n", name)
	}
	fmt.Println()

	fmt.Println("Agent sessions:")
	for _, name := range r.WindowsCreated {
		fmt.Printf("  + %-20s (started)\n", name)
	}
	for _, name := range r.WindowsSkipped {
		fmt.Printf("  = %-20s (exists)\n", name)
	}

	if len(r.StaleWorktrees) > 0 {
		fmt.Println()
		fmt.Println("Stale (remote only):")
		for _, name := range r.StaleWorktrees {
			fmt.Printf("  ! %s\n", name)
		}
	}
}
