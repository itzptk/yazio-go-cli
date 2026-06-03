package config

import (
	"errors"
	"os"
	"path/filepath"
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

type File struct {
	BaseURL string `yaml:"base_url"`
	Output  string `yaml:"output"`
	Token   *Token `yaml:"token,omitempty"`
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
	"# Tokens are written here after `yazio auth login` succeeds.\n" +
	"# Leave this section commented out when sharing the file.\n" +
	"# token:\n" +
	"#   access_token: your-access-token\n" +
	"#   refresh_token: your-refresh-token\n" +
	"#   token_type: Bearer\n" +
	"#   expires_at: 2026-06-02T12:34:56Z\n"

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
	return cfg, nil
}

func Save(path string, cfg File) error {
	if path == "" {
		return errors.New("config path is required")
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}
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
	return os.WriteFile(path, content, 0o600)
}
