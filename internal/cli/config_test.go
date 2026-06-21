package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/itzptk/yazio-go-cli/internal/config"
	"github.com/itzptk/yazio-go-cli/internal/yazio"
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

func TestResolveOAuthCredentialsFromConfig(t *testing.T) {
	unsetEnvForTest(t, "YAZIO_CLIENT_ID")
	unsetEnvForTest(t, "YAZIO_CLIENT_SECRET")

	got := resolveOAuthCredentials(config.File{
		OAuth: &config.OAuth{
			ClientID:     "config-client-id",
			ClientSecret: "config-client-secret",
		},
	})

	if got.ClientID != "config-client-id" {
		t.Fatalf("ClientID = %q, want config-client-id", got.ClientID)
	}
	if got.ClientSecret != "config-client-secret" {
		t.Fatalf("ClientSecret = %q, want config-client-secret", got.ClientSecret)
	}
}

func TestResolveOAuthCredentialsFromEnvOverridesConfig(t *testing.T) {
	t.Setenv("YAZIO_CLIENT_ID", "env-client-id")
	t.Setenv("YAZIO_CLIENT_SECRET", "env-client-secret")

	got := resolveOAuthCredentials(config.File{
		OAuth: &config.OAuth{
			ClientID:     "config-client-id",
			ClientSecret: "config-client-secret",
		},
	})

	if got.ClientID != "env-client-id" {
		t.Fatalf("ClientID = %q, want env-client-id", got.ClientID)
	}
	if got.ClientSecret != "env-client-secret" {
		t.Fatalf("ClientSecret = %q, want env-client-secret", got.ClientSecret)
	}
}

func TestAuthLoginPassesEnvOAuthCredentialsToClientFactory(t *testing.T) {
	t.Setenv("YAZIO_CLIENT_ID", "env-client-id")
	t.Setenv("YAZIO_CLIENT_SECRET", "env-client-secret")

	var gotOAuth yazio.OAuthCredentials
	var out bytes.Buffer
	cmd, err := newRootCommand(&out, "dev", func(_ string, oauth yazio.OAuthCredentials) apiClient {
		gotOAuth = oauth
		return &loginClient{
			token: yazio.Token{
				AccessToken:  "access",
				RefreshToken: "refresh",
				TokenType:    "Bearer",
				ExpiresAt:    time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC),
			},
		}
	})
	if err != nil {
		t.Fatalf("newRootCommand() error = %v", err)
	}
	cmd.SetArgs([]string{
		"--config", filepath.Join(t.TempDir(), "config.yaml"),
		"auth", "login",
		"--email", "user@example.com",
		"--password", "password",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if gotOAuth.ClientID != "env-client-id" {
		t.Fatalf("ClientID = %q, want env-client-id", gotOAuth.ClientID)
	}
	if gotOAuth.ClientSecret != "env-client-secret" {
		t.Fatalf("ClientSecret = %q, want env-client-secret", gotOAuth.ClientSecret)
	}
}

func unsetEnvForTest(t *testing.T, key string) {
	t.Helper()

	value, ok := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("Unsetenv(%q) error = %v", key, err)
	}
	t.Cleanup(func() {
		if ok {
			_ = os.Setenv(key, value)
			return
		}
		_ = os.Unsetenv(key)
	})
}

type loginClient struct {
	token yazio.Token
}

func (f *loginClient) Login(context.Context, yazio.Credentials) (yazio.Token, error) {
	return f.token, nil
}

func (f *loginClient) Refresh(context.Context, yazio.Token) (yazio.Token, error) {
	return yazio.Token{}, errors.New("unexpected Refresh call")
}

func (f *loginClient) GetUser(context.Context, yazio.Token) (yazio.User, error) {
	return yazio.User{}, errors.New("unexpected GetUser call")
}

func (f *loginClient) GetDailySummary(context.Context, yazio.Token, time.Time) (yazio.DailySummary, error) {
	return yazio.DailySummary{}, errors.New("unexpected GetDailySummary call")
}

func (f *loginClient) GetConsumedItems(context.Context, yazio.Token, time.Time) (yazio.ConsumedItemsResponse, error) {
	return yazio.ConsumedItemsResponse{}, errors.New("unexpected GetConsumedItems call")
}

func (f *loginClient) SearchProducts(context.Context, yazio.Token, yazio.SearchOptions) ([]yazio.ProductSearchResult, error) {
	return nil, errors.New("unexpected SearchProducts call")
}

func (f *loginClient) AddConsumedItem(context.Context, yazio.Token, yazio.AddConsumedItemRequest) error {
	return errors.New("unexpected AddConsumedItem call")
}

func (f *loginClient) RemoveConsumedItem(context.Context, yazio.Token, string) error {
	return errors.New("unexpected RemoveConsumedItem call")
}

func TestConfigInitRefusesOverwriteWithoutForce(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	original := "output: json\n"
	if err := os.WriteFile(configPath, []byte(original), 0o600); err != nil {
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
	content, readErr := os.ReadFile(configPath)
	if readErr != nil {
		t.Fatalf("ReadFile() error = %v", readErr)
	}
	if string(content) != original {
		t.Fatalf("config content = %q, want unchanged %q", string(content), original)
	}
}

func TestConfigInitForceOverwritesMalformedConfig(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("base_url: [\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	var out bytes.Buffer
	cmd, err := NewRootCommand(&out, "dev")
	if err != nil {
		t.Fatalf("NewRootCommand() error = %v", err)
	}
	cmd.SetArgs([]string{"--config", configPath, "config", "init", "--force"})

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
	if !strings.Contains(string(content), "output: table") {
		t.Fatalf("config file missing default output: %q", string(content))
	}
	if !strings.Contains(out.String(), configPath) {
		t.Fatalf("output %q does not mention config path %q", out.String(), configPath)
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

func TestOutputFlagRejectsInvalidValueBeforeCommand(t *testing.T) {
	var out bytes.Buffer
	cmd, err := NewRootCommand(&out, "dev")
	if err != nil {
		t.Fatalf("NewRootCommand() error = %v", err)
	}
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	cmd.SetArgs([]string{"--config", configPath, "--output", "csv", "auth", "status"})

	err = cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want invalid output error")
	}
	if !strings.Contains(err.Error(), "invalid output") || !strings.Contains(err.Error(), "expected table or json") {
		t.Fatalf("error = %q, want invalid output format guidance", err)
	}
}

func TestOutputEnvRejectsInvalidValueBeforeCommand(t *testing.T) {
	t.Setenv("YAZIO_OUTPUT", "yaml")

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
		t.Fatal("Execute() error = nil, want invalid output error")
	}
	if !strings.Contains(err.Error(), "invalid output") || !strings.Contains(err.Error(), "expected table or json") {
		t.Fatalf("error = %q, want invalid output format guidance", err)
	}
}

func TestOutputConfigRejectsInvalidValueBeforeCommand(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("output: xml\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	var out bytes.Buffer
	cmd, err := NewRootCommand(&out, "dev")
	if err != nil {
		t.Fatalf("NewRootCommand() error = %v", err)
	}
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{"--config", configPath, "auth", "status"})

	err = cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want invalid output error")
	}
	if !strings.Contains(err.Error(), "invalid output") || !strings.Contains(err.Error(), "expected table or json") {
		t.Fatalf("error = %q, want invalid output format guidance", err)
	}
}
