// Package provisioning handles VM provisioning script management and status monitoring.
package provisioning

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ProvisionStatus represents the status of VM provisioning.
type ProvisionStatus string

const (
	// StatusPending indicates provisioning hasn't started yet.
	StatusPending ProvisionStatus = "pending"
	// StatusRunning indicates provisioning is in progress.
	StatusRunning ProvisionStatus = "running"
	// StatusCompleted indicates provisioning completed successfully.
	StatusCompleted ProvisionStatus = "completed"
	// StatusFailed indicates provisioning failed.
	StatusFailed ProvisionStatus = "failed"
	// StatusUnknown indicates the status couldn't be determined.
	StatusUnknown ProvisionStatus = "unknown"
)

// StatusInfo contains information about provisioning status.
type StatusInfo struct {
	// Status is the current provisioning status.
	Status ProvisionStatus
	// Progress is a human-readable progress string (e.g., "3/34 Installing Node.js").
	Progress string
	// Error contains error details if Status is StatusFailed.
	Error string
}

// File paths for provisioning status on the VM.
const (
	StatusFilePath   = "/var/run/cloudcoop/provision-status"
	ProgressFilePath = "/var/run/cloudcoop/provision-progress"
)

// FetchScript fetches a provisioning script from a URL.
// It returns the script content or an error if the fetch fails.
func FetchScript(ctx context.Context, url string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch script: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch script: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read script: %w", err)
	}

	return string(body), nil
}

// ParseStatusInfo parses status and progress file contents into a StatusInfo.
func ParseStatusInfo(statusContent, progressContent string) StatusInfo {
	// Only use the first line of status (file may contain additional error info)
	status := strings.TrimSpace(statusContent)
	if idx := strings.Index(status, "\n"); idx != -1 {
		status = status[:idx]
	}
	progress := strings.TrimSpace(progressContent)

	info := StatusInfo{
		Progress: progress,
	}

	switch status {
	case "pending":
		info.Status = StatusPending
	case "running":
		info.Status = StatusRunning
	case "completed":
		info.Status = StatusCompleted
	case "failed":
		info.Status = StatusFailed
		// If there's progress content during failure, treat it as error details
		if progress != "" {
			info.Error = progress
		}
	default:
		info.Status = StatusUnknown
	}

	return info
}

// IsProvisioningComplete returns true if provisioning has completed successfully.
func IsProvisioningComplete(info *StatusInfo) bool {
	return info != nil && info.Status == StatusCompleted
}
