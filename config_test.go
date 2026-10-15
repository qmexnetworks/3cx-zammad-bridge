package zammadbridge

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	f := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(f, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestLoadConfigLogOutboundCallsDefaultsToTrue(t *testing.T) {
	config, err := LoadConfigFromYaml(writeConfig(t, "Zammad:\n  endpoint: https://zammad.example.com\n"))
	if err != nil {
		t.Fatal(err)
	}

	if !config.Zammad.LogOutboundCalls {
		t.Error("expected log_outbound_calls to default to true")
	}
}

func TestLoadConfigLogOutboundCallsCanBeDisabled(t *testing.T) {
	config, err := LoadConfigFromYaml(writeConfig(t, "Zammad:\n  log_outbound_calls: false\n"))
	if err != nil {
		t.Fatal(err)
	}

	if config.Zammad.LogOutboundCalls {
		t.Error("expected log_outbound_calls to be false")
	}
}
