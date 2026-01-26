package cli

import (
	"errors"
	"strings"
	"testing"
)

func TestHandleSSHError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		host       string
		port       int
		wantNil    bool
		wantErrMsg string
	}{
		{
			name:    "no authentication methods returns nil",
			err:     errors.New("no SSH authentication methods available"),
			host:    "10.0.0.1",
			port:    22,
			wantNil: true,
		},
		{
			name:    "connection refused returns nil",
			err:     errors.New("connection refused"),
			host:    "10.0.0.1",
			port:    22,
			wantNil: true,
		},
		{
			name:    "unknown host key returns nil",
			err:     errors.New("knownhosts: key is unknown"),
			host:    "10.0.0.1",
			port:    22,
			wantNil: true,
		},
		{
			name:    "unknown host key with custom port",
			err:     errors.New("knownhosts: key is unknown"),
			host:    "10.0.0.1",
			port:    2222,
			wantNil: true,
		},
		{
			name:    "timeout returns nil",
			err:     errors.New("i/o timeout"),
			host:    "10.0.0.1",
			port:    22,
			wantNil: true,
		},
		{
			name:    "connection timed out returns nil",
			err:     errors.New("connection timed out"),
			host:    "10.0.0.1",
			port:    22,
			wantNil: true,
		},
		{
			name:    "unable to authenticate returns nil",
			err:     errors.New("unable to authenticate"),
			host:    "10.0.0.1",
			port:    22,
			wantNil: true,
		},
		{
			name:    "no supported methods returns nil",
			err:     errors.New("no supported methods remain"),
			host:    "10.0.0.1",
			port:    22,
			wantNil: true,
		},
		{
			name:       "other errors are returned",
			err:        errors.New("some other SSH error"),
			host:       "10.0.0.1",
			port:       22,
			wantNil:    false,
			wantErrMsg: "some other SSH error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handleSSHError(tt.err, tt.host, tt.port)
			if tt.wantNil {
				if result != nil {
					t.Errorf("handleSSHError() = %v, want nil", result)
				}
			} else {
				if result == nil {
					t.Errorf("handleSSHError() = nil, want error containing %q", tt.wantErrMsg)
				} else if !strings.Contains(result.Error(), tt.wantErrMsg) {
					t.Errorf("handleSSHError() = %v, want error containing %q", result, tt.wantErrMsg)
				}
			}
		})
	}
}
