package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/itzptk/yazio-go-cli/internal/config"
)

func TestAddRejectsNonPositiveServingQuantityBeforeAPI(t *testing.T) {
	cases := []struct {
		name            string
		servingQuantity string
	}{
		{name: "zero", servingQuantity: "0"},
		{name: "negative", servingQuantity: "-1"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			apiCalls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				apiCalls++
				w.WriteHeader(http.StatusNoContent)
			}))
			defer server.Close()

			configPath := writeCLIConfig(t, server.URL)
			err := executeCLI(t, []string{
				"--config", configPath,
				"add",
				"--product-id", "11111111-1111-1111-1111-111111111111",
				"--meal", "breakfast",
				"--amount", "100",
				"--serving", "g",
				"--serving-quantity", tc.servingQuantity,
				"--date", "2026-06-02",
			})

			if err == nil {
				t.Fatal("Execute() error = nil, want serving quantity validation error")
			}
			if !strings.Contains(err.Error(), "--serving-quantity must be greater than zero") {
				t.Fatalf("error = %q, want serving quantity validation", err)
			}
			if apiCalls != 0 {
				t.Fatalf("API calls = %d, want 0", apiCalls)
			}
		})
	}
}

func TestAddSendsPositiveServingQuantity(t *testing.T) {
	apiCalls := 0
	var received struct {
		Products []struct {
			ProductID       string   `json:"product_id"`
			Serving         *string  `json:"serving"`
			ServingQuantity *float64 `json:"serving_quantity"`
		} `json:"products"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiCalls++
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/user/consumed-items" {
			t.Fatalf("path = %s, want /user/consumed-items", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	configPath := writeCLIConfig(t, server.URL)
	err := executeCLI(t, []string{
		"--config", configPath,
		"add",
		"--product-id", "11111111-1111-1111-1111-111111111111",
		"--meal", "breakfast",
		"--amount", "100",
		"--serving", "g",
		"--serving-quantity", "2.5",
		"--date", "2026-06-02",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if apiCalls != 1 {
		t.Fatalf("API calls = %d, want 1", apiCalls)
	}
	if len(received.Products) != 1 {
		t.Fatalf("products = %d, want 1", len(received.Products))
	}
	product := received.Products[0]
	if product.Serving == nil || *product.Serving != "g" {
		t.Fatalf("serving = %v, want g", product.Serving)
	}
	if product.ServingQuantity == nil || *product.ServingQuantity != 2.5 {
		t.Fatalf("serving quantity = %v, want 2.5", product.ServingQuantity)
	}
}

func executeCLI(t *testing.T, args []string) error {
	t.Helper()

	var out bytes.Buffer
	cmd, err := NewRootCommand(&out, "dev")
	if err != nil {
		t.Fatalf("NewRootCommand() error = %v", err)
	}
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs(args)
	return cmd.Execute()
}

func writeCLIConfig(t *testing.T, baseURL string) string {
	t.Helper()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := config.Save(configPath, config.File{
		BaseURL: baseURL,
		Output:  config.DefaultOutput,
		Token: &config.Token{
			AccessToken: "access-token",
			TokenType:   "Bearer",
			ExpiresAt:   time.Now().Add(time.Hour),
		},
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	return configPath
}
