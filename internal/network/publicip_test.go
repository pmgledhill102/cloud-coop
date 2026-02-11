package network

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDetectPublicIP(t *testing.T) {
	tests := []struct {
		name       string
		response   string
		statusCode int
		wantIP     string
		wantErr    bool
	}{
		{
			name:       "valid IPv4",
			response:   "203.0.113.50\n",
			statusCode: http.StatusOK,
			wantIP:     "203.0.113.50",
		},
		{
			name:       "valid IPv4 no newline",
			response:   "10.0.0.1",
			statusCode: http.StatusOK,
			wantIP:     "10.0.0.1",
		},
		{
			name:       "valid IPv6",
			response:   "2001:db8::1\n",
			statusCode: http.StatusOK,
			wantIP:     "2001:db8::1",
		},
		{
			name:       "server error",
			response:   "error",
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
		},
		{
			name:       "invalid response",
			response:   "not-an-ip",
			statusCode: http.StatusOK,
			wantErr:    true,
		},
		{
			name:       "empty response",
			response:   "",
			statusCode: http.StatusOK,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.response))
			}))
			defer srv.Close()

			// Override the URL by using a custom transport
			origDetect := detectURL
			detectURL = srv.URL
			defer func() { detectURL = origDetect }()

			ip, err := DetectPublicIP(context.Background())
			if (err != nil) != tt.wantErr {
				t.Fatalf("DetectPublicIP() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && ip != tt.wantIP {
				t.Errorf("DetectPublicIP() = %q, want %q", ip, tt.wantIP)
			}
		})
	}
}

func TestDetectPublicIP_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := DetectPublicIP(ctx)
	if err == nil {
		t.Error("expected error for canceled context")
	}
}

func TestDetectPublicIP_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Second)
	}))
	defer srv.Close()

	origDetect := detectURL
	detectURL = srv.URL
	defer func() { detectURL = origDetect }()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := DetectPublicIP(ctx)
	if err == nil {
		t.Error("expected timeout error")
	}
}
