package provisioning

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseStatusInfo(t *testing.T) {
	tests := []struct {
		name            string
		statusContent   string
		progressContent string
		wantStatus      ProvisionStatus
		wantProgress    string
		wantError       string
	}{
		{
			name:          "pending status",
			statusContent: "pending",
			wantStatus:    StatusPending,
		},
		{
			name:            "running with progress",
			statusContent:   "running",
			progressContent: "3/34 Installing Node.js",
			wantStatus:      StatusRunning,
			wantProgress:    "3/34 Installing Node.js",
		},
		{
			name:          "completed",
			statusContent: "completed",
			wantStatus:    StatusCompleted,
		},
		{
			name:            "failed with error",
			statusContent:   "failed",
			progressContent: "apt-get failed with exit code 1",
			wantStatus:      StatusFailed,
			wantProgress:    "apt-get failed with exit code 1",
			wantError:       "apt-get failed with exit code 1",
		},
		{
			name:          "unknown status",
			statusContent: "garbage",
			wantStatus:    StatusUnknown,
		},
		{
			name:          "empty status",
			statusContent: "",
			wantStatus:    StatusUnknown,
		},
		{
			name:          "whitespace trimmed",
			statusContent: "  running\n",
			wantStatus:    StatusRunning,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := ParseStatusInfo(tt.statusContent, tt.progressContent)

			if info.Status != tt.wantStatus {
				t.Errorf("Status = %q, want %q", info.Status, tt.wantStatus)
			}
			if info.Progress != tt.wantProgress {
				t.Errorf("Progress = %q, want %q", info.Progress, tt.wantProgress)
			}
			if info.Error != tt.wantError {
				t.Errorf("Error = %q, want %q", info.Error, tt.wantError)
			}
		})
	}
}

func TestIsProvisioningComplete(t *testing.T) {
	tests := []struct {
		name string
		info *StatusInfo
		want bool
	}{
		{
			name: "completed",
			info: &StatusInfo{Status: StatusCompleted},
			want: true,
		},
		{
			name: "running",
			info: &StatusInfo{Status: StatusRunning},
			want: false,
		},
		{
			name: "pending",
			info: &StatusInfo{Status: StatusPending},
			want: false,
		},
		{
			name: "failed",
			info: &StatusInfo{Status: StatusFailed},
			want: false,
		},
		{
			name: "nil info",
			info: nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsProvisioningComplete(tt.info)
			if got != tt.want {
				t.Errorf("IsProvisioningComplete() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFetchScript(t *testing.T) {
	t.Run("successful fetch", func(t *testing.T) {
		script := "#!/bin/bash\necho hello"
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(script))
		}))
		defer server.Close()

		got, err := FetchScript(context.Background(), server.URL)
		if err != nil {
			t.Fatalf("FetchScript() error = %v", err)
		}
		if got != script {
			t.Errorf("FetchScript() = %q, want %q", got, script)
		}
	})

	t.Run("404 error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		_, err := FetchScript(context.Background(), server.URL)
		if err == nil {
			t.Error("FetchScript() expected error for 404")
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := FetchScript(ctx, "http://example.com")
		if err == nil {
			t.Error("FetchScript() expected error for canceled context")
		}
	})
}
