package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigInitWritesExampleFile(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	cmd, err := NewRootCommand(&out, "dev")
	if err != nil {
		t.Fatalf("NewRootCommand() error = %v", err)
	}

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	cmd.SetArgs([]string{"--config", configPath, "config", "init"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(content), "base_url:") {
		t.Fatalf("config file missing base_url: %q", string(content))
	}
	if !strings.Contains(out.String(), configPath) {
		t.Fatalf("output %q does not mention config path %q", out.String(), configPath)
	}
}

func TestConfigInitRefusesOverwriteWithoutForce(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("output: json\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	var out bytes.Buffer
	cmd, err := NewRootCommand(&out, "dev")
	if err != nil {
		t.Fatalf("NewRootCommand() error = %v", err)
	}
	cmd.SetArgs([]string{"--config", configPath, "config", "init"})

	err = cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want overwrite refusal")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("error = %q, want already exists", err)
	}
}

func TestBaseURLFlagRejectsMalformedURLBeforeCommand(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	cmd, err := NewRootCommand(&out, "dev")
	if err != nil {
		t.Fatalf("NewRootCommand() error = %v", err)
	}
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	cmd.SetArgs([]string{"--config", configPath, "--base-url", "http://[::1", "auth", "status"})

	err = cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want invalid base URL error")
	}
	if !strings.Contains(err.Error(), "invalid base URL") {
		t.Fatalf("error = %q, want invalid base URL", err)
	}
}

func TestBaseURLEnvRejectsMalformedURLBeforeCommand(t *testing.T) {
	t.Setenv("YAZIO_BASE_URL", "http://[::1")

	var out bytes.Buffer
	cmd, err := NewRootCommand(&out, "dev")
	if err != nil {
		t.Fatalf("NewRootCommand() error = %v", err)
	}
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	cmd.SetArgs([]string{"--config", configPath, "auth", "status"})

	err = cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want invalid base URL error")
	}
	if !strings.Contains(err.Error(), "invalid base URL") {
		t.Fatalf("error = %q, want invalid base URL", err)
	}
}
