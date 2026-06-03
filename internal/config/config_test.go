package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSaveAndLoad(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	want := File{
		BaseURL: "https://yzapi.yazio.com/v15",
		Output:  "json",
		Token: &Token{
			AccessToken:  "access-token",
			RefreshToken: "refresh-token",
			TokenType:    "Bearer",
			ExpiresAt:    time.Unix(1_717_280_000, 0).UTC(),
		},
	}

	if err := Save(path, want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got.BaseURL != want.BaseURL {
		t.Fatalf("BaseURL = %q, want %q", got.BaseURL, want.BaseURL)
	}
	if got.Output != want.Output {
		t.Fatalf("Output = %q, want %q", got.Output, want.Output)
	}
	if got.Token == nil {
		t.Fatal("Token = nil, want token")
	}
	if got.Token.AccessToken != want.Token.AccessToken {
		t.Fatalf("AccessToken = %q, want %q", got.Token.AccessToken, want.Token.AccessToken)
	}
	if !got.Token.ExpiresAt.Equal(want.Token.ExpiresAt) {
		t.Fatalf("ExpiresAt = %s, want %s", got.Token.ExpiresAt, want.Token.ExpiresAt)
	}
}

func TestLoadMissingReturnsDefaults(t *testing.T) {
	t.Parallel()

	got, err := Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got.BaseURL != DefaultBaseURL {
		t.Fatalf("BaseURL = %q, want %q", got.BaseURL, DefaultBaseURL)
	}
	if got.Output != DefaultOutput {
		t.Fatalf("Output = %q, want %q", got.Output, DefaultOutput)
	}
	if got.Token != nil {
		t.Fatalf("Token = %#v, want nil", got.Token)
	}
}

func TestExampleFileParsesAsDefaultsWithoutToken(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(ExampleFile), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got.BaseURL != DefaultBaseURL {
		t.Fatalf("BaseURL = %q, want %q", got.BaseURL, DefaultBaseURL)
	}
	if got.Output != DefaultOutput {
		t.Fatalf("Output = %q, want %q", got.Output, DefaultOutput)
	}
	if got.Token != nil {
		t.Fatalf("Token = %#v, want nil", got.Token)
	}
	if !strings.Contains(ExampleFile, "base_url") || !strings.Contains(ExampleFile, "output") {
		t.Fatalf("ExampleFile missing expected keys: %q", ExampleFile)
	}
}
