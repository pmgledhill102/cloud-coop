package provisioning

import (
	"fmt"
	"strings"

	"github.com/cloud-coop/cloudcoop/internal/ssh"
)

// CheckStatus checks the provisioning status on a remote VM via SSH.
// It reads the status and progress files and returns the combined status info.
func CheckStatus(runner ssh.Runner) (*StatusInfo, error) {
	// Read status file
	statusContent, statusErr := runner.Run(fmt.Sprintf("cat %s 2>/dev/null || echo 'pending'", StatusFilePath))
	if statusErr != nil {
		// If we can't run the command at all, return unknown
		return &StatusInfo{Status: StatusUnknown}, fmt.Errorf("read status file: %w", statusErr)
	}

	// Read progress file (optional, may not exist)
	progressContent, _ := runner.Run(fmt.Sprintf("cat %s 2>/dev/null", ProgressFilePath))

	// Parse and return the status info
	info := ParseStatusInfo(statusContent, progressContent)
	return &info, nil
}

// StripShebang removes the shebang line from a script if it starts with one.
// This is useful when appending a script to an existing startup script.
func StripShebang(script string) string {
	lines := strings.Split(script, "\n")
	if len(lines) > 0 && strings.HasPrefix(lines[0], "#!") {
		return strings.Join(lines[1:], "\n")
	}
	return script
}
