package ops

import (
	"context"
	"testing"

	"github.com/cloud-coop/cloudcoop/internal/config"
)

func TestNewProvider_UnsupportedProvider(t *testing.T) {
	cfg := &config.Config{
		Cloud: config.CloudConfig{Provider: "azure"},
	}
	_, _, err := NewProvider(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected error for unsupported provider")
	}
	if want := "unsupported provider: azure"; err.Error() != want {
		t.Fatalf("error = %q, want %q", err.Error(), want)
	}
}

func TestNewProvider_UnknownProvider(t *testing.T) {
	cfg := &config.Config{
		Cloud: config.CloudConfig{Provider: "unknown"},
	}
	_, _, err := NewProvider(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}
