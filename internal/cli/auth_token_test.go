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

func TestEnsureTokenRefreshesTokenWithoutExpiry(t *testing.T) {
	t.Parallel()

	cfgPath := writeRawTokenConfigForTest(t, `token:
  access_token: old-access
  refresh_token: refresh-token
  token_type: Bearer
`)
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	refreshedExpiresAt := time.Date(2026, 6, 5, 13, 0, 0, 0, time.UTC)
	fake := &refreshTokenClient{
		refreshedToken: yazio.Token{
			AccessToken:  "new-access",
			RefreshToken: "new-refresh",
			TokenType:    "Bearer",
			ExpiresAt:    refreshedExpiresAt,
		},
	}
	app := &App{
		cfgPath:       cfgPath,
		cfg:           cfg,
		baseURL:       cfg.BaseURL,
		clientFactory: func(string) apiClient { return fake },
	}

	token, err := app.ensureToken(context.Background())
	if err != nil {
		t.Fatalf("ensureToken() error = %v", err)
	}
	if fake.refreshCalls != 1 {
		t.Fatalf("Refresh calls = %d, want 1", fake.refreshCalls)
	}
	if fake.refreshInput.AccessToken != "old-access" || fake.refreshInput.RefreshToken != "refresh-token" {
		t.Fatalf("Refresh input = %#v, want old stored token", fake.refreshInput)
	}
	if token.AccessToken != "new-access" || !token.ExpiresAt.Equal(refreshedExpiresAt) {
		t.Fatalf("token = %#v, want refreshed token", token)
	}
	saved, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load(saved) error = %v", err)
	}
	if saved.Token == nil || saved.Token.AccessToken != "new-access" || !saved.Token.ExpiresAt.Equal(refreshedExpiresAt) {
		t.Fatalf("saved token = %#v, want refreshed token", saved.Token)
	}
}

func TestEnsureTokenWithoutExpiryAndRefreshTokenRequiresLogin(t *testing.T) {
	t.Parallel()

	cfgPath := writeRawTokenConfigForTest(t, `token:
  access_token: old-access
  token_type: Bearer
`)
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	fake := &refreshTokenClient{}
	app := &App{
		cfgPath:       cfgPath,
		cfg:           cfg,
		baseURL:       cfg.BaseURL,
		clientFactory: func(string) apiClient { return fake },
	}

	_, err = app.ensureToken(context.Background())
	if err == nil {
		t.Fatal("ensureToken() error = nil, want login-needed error")
	}
	if !strings.Contains(err.Error(), "stored token expired and no refresh token is available") {
		t.Fatalf("ensureToken() error = %q, want expired/no-refresh message", err)
	}
	if fake.refreshCalls != 0 {
		t.Fatalf("Refresh calls = %d, want 0", fake.refreshCalls)
	}
}

func TestAuthStatusReportsTokenWithoutExpiryAsExpired(t *testing.T) {
	t.Parallel()

	cfgPath := writeRawTokenConfigForTest(t, `token:
  access_token: old-access
  refresh_token: refresh-token
  token_type: Bearer
`)
	var out bytes.Buffer
	cmd, err := newRootCommand(&out, "dev", func(string) apiClient { return &refreshTokenClient{} })
	if err != nil {
		t.Fatalf("newRootCommand() error = %v", err)
	}
	cmd.SetArgs([]string{"--config", cfgPath, "auth", "status"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out.String(), "status: expired") {
		t.Fatalf("output = %q, want expired status", out.String())
	}
	if strings.Contains(out.String(), "status: valid") {
		t.Fatalf("output = %q, must not report valid status", out.String())
	}
}

func writeRawTokenConfigForTest(t *testing.T, tokenYAML string) string {
	t.Helper()
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	content := "base_url: https://example.test/v15\noutput: table\n" + tokenYAML
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return cfgPath
}

type refreshTokenClient struct {
	refreshCalls   int
	refreshInput   yazio.Token
	refreshedToken yazio.Token
}

func (f *refreshTokenClient) Login(context.Context, yazio.Credentials) (yazio.Token, error) {
	return yazio.Token{}, errors.New("unexpected Login call")
}

func (f *refreshTokenClient) Refresh(_ context.Context, token yazio.Token) (yazio.Token, error) {
	f.refreshCalls++
	f.refreshInput = token
	if f.refreshedToken.AccessToken == "" {
		return yazio.Token{}, errors.New("unexpected Refresh call")
	}
	return f.refreshedToken, nil
}

func (f *refreshTokenClient) GetUser(context.Context, yazio.Token) (yazio.User, error) {
	return yazio.User{}, errors.New("unexpected GetUser call")
}

func (f *refreshTokenClient) GetDailySummary(context.Context, yazio.Token, time.Time) (yazio.DailySummary, error) {
	return yazio.DailySummary{}, errors.New("unexpected GetDailySummary call")
}

func (f *refreshTokenClient) GetConsumedItems(context.Context, yazio.Token, time.Time) (yazio.ConsumedItemsResponse, error) {
	return yazio.ConsumedItemsResponse{}, errors.New("unexpected GetConsumedItems call")
}

func (f *refreshTokenClient) SearchProducts(context.Context, yazio.Token, yazio.SearchOptions) ([]yazio.ProductSearchResult, error) {
	return nil, errors.New("unexpected SearchProducts call")
}

func (f *refreshTokenClient) AddConsumedItem(context.Context, yazio.Token, yazio.AddConsumedItemRequest) error {
	return errors.New("unexpected AddConsumedItem call")
}

func (f *refreshTokenClient) RemoveConsumedItem(context.Context, yazio.Token, string) error {
	return errors.New("unexpected RemoveConsumedItem call")
}
