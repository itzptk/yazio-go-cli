package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	DefaultBaseURL = "https://yzapi.yazio.com/v15"
	DefaultOutput  = "table"
)

type Token struct {
	AccessToken  string    `yaml:"access_token"`
	RefreshToken string    `yaml:"refresh_token"`
	TokenType    string    `yaml:"token_type"`
	ExpiresAt    time.Time `yaml:"expires_at"`
}

type OAuth struct {
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
}

type MealTemplate struct {
	ProductID       string   `yaml:"product_id" json:"product_id"`
	Meal            string   `yaml:"meal" json:"meal"`
	Amount          float64  `yaml:"amount" json:"amount"`
	Serving         string   `yaml:"serving,omitempty" json:"serving,omitempty"`
	ServingQuantity *float64 `yaml:"serving_quantity,omitempty" json:"serving_quantity,omitempty"`
	Notes           string   `yaml:"notes,omitempty" json:"notes,omitempty"`
}

type File struct {
	BaseURL   string                  `yaml:"base_url"`
	Output    string                  `yaml:"output"`
	OAuth     *OAuth                  `yaml:"oauth,omitempty"`
	Token     *Token                  `yaml:"token,omitempty"`
	Templates map[string]MealTemplate `yaml:"templates,omitempty"`
}

const ExampleFile = "# Example config for yazio-go-cli.\n" +
	"# Copy to ~/.config/yazio-go-cli/config.yaml or pass with --config.\n" +
	"\n" +
	"# Optional override for the unofficial YAZIO API base URL.\n" +
	"base_url: https://yzapi.yazio.com/v15\n" +
	"\n" +
	"# Default output mode for commands.\n" +
	"# Valid values: table, json\n" +
	"output: table\n" +
	"\n" +
	"# OAuth client credentials are required for `yazio auth login` and token refresh.\n" +
	"# You can also provide them with YAZIO_CLIENT_ID and YAZIO_CLIENT_SECRET.\n" +
	"# oauth:\n" +
	"#   client_id: your-client-id\n" +
	"#   client_secret: your-client-secret\n" +
	"\n" +
	"# Tokens are written here after `yazio auth login` succeeds.\n" +
	"# Leave this section commented out when sharing the file.\n" +
	"# token:\n" +
	"#   access_token: your-access-token\n" +
	"#   refresh_token: your-refresh-token\n" +
	"#   token_type: Bearer\n" +
	"#   expires_at: 2026-06-02T12:34:56Z\n" +
	"\n" +
	"# Saved meal templates are managed with `yazio template ...` commands.\n" +
	"# templates:\n" +
	"#   weekday-breakfast:\n" +
	"#     product_id: 11111111-1111-1111-1111-111111111111\n" +
	"#     meal: breakfast\n" +
	"#     amount: 100\n" +
	"#     serving: g\n" +
	"#     serving_quantity: 1\n" +
	"#     notes: optional reminder\n"

func DefaultFile() File {
	return File{
		BaseURL: DefaultBaseURL,
		Output:  DefaultOutput,
	}
}

func DefaultPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "yazio-go-cli", "config.yaml"), nil
}

func Load(path string) (File, error) {
	cfg := DefaultFile()
	if path == "" {
		return cfg, errors.New("config path is required")
	}

	content, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return File{}, err
	}
	if len(content) == 0 {
		return cfg, nil
	}
	if err := yaml.Unmarshal(content, &cfg); err != nil {
		return File{}, err
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	if cfg.Output == "" {
		cfg.Output = DefaultOutput
	}
	baseURL, err := NormalizeBaseURL(cfg.BaseURL)
	if err != nil {
		return File{}, err
	}
	cfg.BaseURL = baseURL
	output, err := NormalizeOutput(cfg.Output)
	if err != nil {
		return File{}, err
	}
	cfg.Output = output
	return cfg, nil
}

func NormalizeBaseURL(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		trimmed = DefaultBaseURL
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("invalid base URL %q: %w", value, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("invalid base URL %q: expected http or https URL", value)
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("invalid base URL %q: expected host", value)
	}
	return strings.TrimRight(trimmed, "/"), nil
}

func NormalizeOutput(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return DefaultOutput, nil
	}
	switch trimmed {
	case "table", "json":
		return trimmed, nil
	default:
		return "", fmt.Errorf("invalid output %q: expected table or json", value)
	}
}

func Save(path string, cfg File) error {
	if path == "" {
		return errors.New("config path is required")
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	baseURL, err := NormalizeBaseURL(cfg.BaseURL)
	if err != nil {
		return err
	}
	cfg.BaseURL = baseURL
	if cfg.Output == "" {
		cfg.Output = DefaultOutput
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	content, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}
