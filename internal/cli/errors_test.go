package cli

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestHandleConfigError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantNil    bool
		wantErrMsg string
	}{
		{
			name:    "path error returns nil (handles gracefully)",
			err:     &os.PathError{Op: "open", Path: "/test", Err: os.ErrNotExist},
			wantNil: true,
		},
		{
			name:    "ErrNotExist returns nil (handles gracefully)",
			err:     os.ErrNotExist,
			wantNil: true,
		},
		{
			name:       "other errors are returned",
			err:        errors.New("some other error"),
			wantNil:    false,
			wantErrMsg: "some other error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handleConfigError(tt.err)
			if tt.wantNil {
				if result != nil {
					t.Errorf("handleConfigError() = %v, want nil", result)
				}
			} else {
				if result == nil {
					t.Errorf("handleConfigError() = nil, want error containing %q", tt.wantErrMsg)
				} else if !strings.Contains(result.Error(), tt.wantErrMsg) {
					t.Errorf("handleConfigError() = %v, want error containing %q", result, tt.wantErrMsg)
				}
			}
		})
	}
}

func TestHandleProviderError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantNil    bool
		wantErrMsg string
	}{
		{
			name:    "default credentials error returns nil",
			err:     errors.New("could not find default credentials"),
			wantNil: true,
		},
		{
			name:    "credentials file not found returns nil",
			err:     errors.New("no such file or directory for credentials"),
			wantNil: true,
		},
		{
			name:       "other errors are wrapped",
			err:        errors.New("some API error"),
			wantNil:    false,
			wantErrMsg: "create cloud provider",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handleProviderError(tt.err)
			if tt.wantNil {
				if result != nil {
					t.Errorf("handleProviderError() = %v, want nil", result)
				}
			} else {
				if result == nil {
					t.Errorf("handleProviderError() = nil, want error containing %q", tt.wantErrMsg)
				} else if !strings.Contains(result.Error(), tt.wantErrMsg) {
					t.Errorf("handleProviderError() = %v, want error containing %q", result, tt.wantErrMsg)
				}
			}
		})
	}
}
