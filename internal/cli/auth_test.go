package cli

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/cloud-coop/cloudcoop/internal/cloud"
	"github.com/cloud-coop/cloudcoop/internal/testutil"
)

func TestClassifyAuthStatus(t *testing.T) {
	tests := []struct {
		name       string
		output     string
		runErr     error
		wantStatus string
		wantDetail string
	}{
		{
			name:       "AUTH_OK output",
			output:     "some preamble AUTH_OK trailing text",
			runErr:     nil,
			wantStatus: "authenticated",
		},
		{
			name:       "command not found",
			output:     "bash: claude: command not found",
			runErr:     errors.New("exit status 127"),
			wantStatus: "not_installed",
		},
		{
			name:       "not found variant",
			output:     "claude: not found",
			runErr:     errors.New("exit status 127"),
			wantStatus: "not_installed",
		},
		{
			name:       "Invalid API key",
			output:     "Error: Invalid API key provided",
			runErr:     nil,
			wantStatus: "not_authenticated",
		},
		{
			name:       "Please run login",
			output:     "Please run /login to authenticate",
			runErr:     nil,
			wantStatus: "not_authenticated",
		},
		{
			name:       "generic error with runErr",
			output:     "something went wrong",
			runErr:     errors.New("exit status 1"),
			wantStatus: "not_authenticated",
		},
		{
			name:       "authentication keyword",
			output:     "authentication required",
			runErr:     nil,
			wantStatus: "not_authenticated",
		},
		{
			name:       "unknown output",
			output:     "some unexpected response from claude",
			runErr:     nil,
			wantStatus: "unknown",
			wantDetail: "some unexpected response from claude",
		},
		{
			name:       "empty output no error",
			output:     "",
			runErr:     nil,
			wantStatus: "unknown",
			wantDetail: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, detail := classifyAuthStatus(tt.output, tt.runErr)
			if status != tt.wantStatus {
				t.Errorf("classifyAuthStatus() status = %q, want %q", status, tt.wantStatus)
			}
			if detail != tt.wantDetail {
				t.Errorf("classifyAuthStatus() detail = %q, want %q", detail, tt.wantDetail)
			}
		})
	}
}

func TestRunAuthStatus_UnsupportedAgent(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runAuthStatus(cmd, []string{"unsupported-agent"})
	if err == nil {
		t.Error("expected error for unsupported agent")
	}
	if !strings.Contains(err.Error(), "unsupported agent") {
		t.Errorf("error = %v, want containing 'unsupported agent'", err)
	}
}

func TestRunAuthStatus_VMNotFound(t *testing.T) {
	cfg := testConfig()
	mock := cloud.NewMockProvider().WithVMStatus(cloud.VMStatusNotFound)
	cleanup := withMocks(cfg, mock)
	defer cleanup()

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runAuthStatus(cmd, nil)
	if err != nil {
		t.Errorf("runAuthStatus() unexpected error: %v", err)
	}
}

func TestRunAuthStatus_VMStopped(t *testing.T) {
	cfg := testConfig()
	mock := cloud.NewMockProvider().WithVMStatus(cloud.VMStatusStopped)
	cleanup := withMocks(cfg, mock)
	defer cleanup()

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runAuthStatus(cmd, nil)
	if err != nil {
		t.Errorf("runAuthStatus() unexpected error: %v", err)
	}
}

func TestRunAuthStatus_HappyPath(t *testing.T) {
	cfg := testConfig()
	mock := cloud.NewMockProvider()
	sshMock := testutil.NewMockSSHClient()
	sshMock.ExpectCommand("claude *").Return("AUTH_OK", nil)
	cleanup := withFullMocks(cfg, mock, sshMock)
	defer cleanup()

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runAuthStatus(cmd, nil)
	if err != nil {
		t.Errorf("runAuthStatus() unexpected error: %v", err)
	}
}

func TestRunAuthLogin_UnsupportedAgent(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runAuthLogin(cmd, []string{"unsupported-agent"})
	if err == nil {
		t.Error("expected error for unsupported agent")
	}
	if !strings.Contains(err.Error(), "unsupported agent") {
		t.Errorf("error = %v, want containing 'unsupported agent'", err)
	}
}

func TestRunAuthLogin_VMNotFound(t *testing.T) {
	cfg := testConfig()
	mock := cloud.NewMockProvider().WithVMStatus(cloud.VMStatusNotFound)
	cleanup := withMocks(cfg, mock)
	defer cleanup()

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runAuthLogin(cmd, nil)
	if err != nil {
		t.Errorf("runAuthLogin() unexpected error: %v", err)
	}
}

func TestRunAuthLogin_VMStopped(t *testing.T) {
	cfg := testConfig()
	mock := cloud.NewMockProvider().WithVMStatus(cloud.VMStatusStopped)
	cleanup := withMocks(cfg, mock)
	defer cleanup()

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runAuthLogin(cmd, nil)
	if err != nil {
		t.Errorf("runAuthLogin() unexpected error: %v", err)
	}
}
