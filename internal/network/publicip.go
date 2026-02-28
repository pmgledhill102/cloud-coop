// Package network provides network utility functions for cloudcoop.
package network

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// detectURL is the URL used to detect the public IP. Overridable for testing.
var detectURL = "https://checkip.amazonaws.com"

// DetectPublicIP returns the workstation's public IP address.
// It makes an HTTP GET request to an external service and validates the response.
func DetectPublicIP(ctx context.Context) (string, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, detectURL, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	resp, err := client.Do(req) //nolint:gosec // G704: URL is from hardcoded package var, not user input
	if err != nil {
		return "", fmt.Errorf("detect public IP: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("detect public IP: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 256))
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	ip := strings.TrimSpace(string(body))
	if net.ParseIP(ip) == nil {
		return "", fmt.Errorf("invalid IP response: %q", ip)
	}

	return ip, nil
}
