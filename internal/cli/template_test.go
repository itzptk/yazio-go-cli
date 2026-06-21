package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/itzptk/yazio-go-cli/internal/config"
)

func TestTemplateCreateListAndRemovePersistTemplates(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := config.Save(configPath, config.File{
		BaseURL: config.DefaultBaseURL,
		Output:  config.DefaultOutput,
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	_, err := executeCLIWithOutput(t, []string{
		"--config", configPath,
		"template", "create", "weekday-breakfast",
		"--product-id", "11111111-1111-1111-1111-111111111111",
		"--meal", "breakfast",
		"--amount", "100",
		"--serving", "g",
		"--serving-quantity", "1.5",
		"--notes", "weekday default",
	})
	if err != nil {
		t.Fatalf("template create error = %v", err)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	template, ok := cfg.Templates["weekday-breakfast"]
	if !ok {
		t.Fatalf("template not persisted: %#v", cfg.Templates)
	}
	if template.ProductID != "11111111-1111-1111-1111-111111111111" || template.Meal != "breakfast" || template.Amount != 100 {
		t.Fatalf("template = %#v, want product/meal/amount", template)
	}
	if template.Serving != "g" || template.ServingQuantity == nil || *template.ServingQuantity != 1.5 {
		t.Fatalf("serving = %q quantity = %v, want g/1.5", template.Serving, template.ServingQuantity)
	}
	if template.Notes != "weekday default" {
		t.Fatalf("notes = %q, want weekday default", template.Notes)
	}

	out, err := executeCLIWithOutput(t, []string{"--config", configPath, "template", "list"})
	if err != nil {
		t.Fatalf("template list error = %v", err)
	}
	if !strings.Contains(out, "weekday-breakfast") || !strings.Contains(out, "weekday default") {
		t.Fatalf("list output = %q, want template name and notes", out)
	}

	_, err = executeCLIWithOutput(t, []string{"--config", configPath, "template", "remove", "weekday-breakfast"})
	if err != nil {
		t.Fatalf("template remove error = %v", err)
	}
	cfg, err = config.Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(cfg.Templates) != 0 {
		t.Fatalf("templates after remove = %#v, want empty", cfg.Templates)
	}
}

func TestTemplateRejectsInvalidNamesBeforeSaving(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")

	cases := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "missing",
			args: []string{"--config", configPath, "template", "create"},
			want: "template name is required",
		},
		{
			name: "surrounding whitespace",
			args: []string{
				"--config", configPath,
				"template", "create", " weekday-breakfast ",
				"--product-id", "11111111-1111-1111-1111-111111111111",
				"--meal", "breakfast",
				"--amount", "100",
			},
			want: "leading or trailing whitespace",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := executeCLIWithOutput(t, tc.args)
			if err == nil {
				t.Fatal("Execute() error = nil, want validation error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want %q", err, tc.want)
			}
		})
	}

	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatalf("config path exists after invalid names: stat err = %v", err)
	}
}

func TestTemplateAddRequiresAuthBeforeAPI(t *testing.T) {
	apiCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiCalls++
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	configPath := writeConfigFile(t, config.File{
		BaseURL: server.URL,
		Output:  config.DefaultOutput,
		Templates: map[string]config.MealTemplate{
			"weekday-breakfast": {
				ProductID: "11111111-1111-1111-1111-111111111111",
				Meal:      "breakfast",
				Amount:    100,
			},
		},
	})

	_, err := executeCLIWithOutput(t, []string{"--config", configPath, "template", "add", "weekday-breakfast"})
	if err == nil {
		t.Fatal("Execute() error = nil, want missing auth error")
	}
	if !strings.Contains(err.Error(), "not logged in; run `yazio auth login` first") {
		t.Fatalf("error = %q, want missing auth", err)
	}
	if apiCalls != 0 {
		t.Fatalf("API calls = %d, want 0", apiCalls)
	}
}

func TestTemplateAddRejectsInvalidPersistedTemplateBeforeAuthAndAPI(t *testing.T) {
	apiCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiCalls++
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	configPath := writeConfigFile(t, config.File{
		BaseURL: server.URL,
		Output:  config.DefaultOutput,
		Templates: map[string]config.MealTemplate{
			"broken": {
				ProductID: "11111111-1111-1111-1111-111111111111",
				Meal:      "breakfast",
				Amount:    0,
			},
		},
	})

	_, err := executeCLIWithOutput(t, []string{"--config", configPath, "template", "add", "broken"})
	if err == nil {
		t.Fatal("Execute() error = nil, want template validation error")
	}
	if !strings.Contains(err.Error(), "invalid template \"broken\"") || !strings.Contains(err.Error(), "--product-id, --meal, and --amount are required") {
		t.Fatalf("error = %q, want invalid template validation", err)
	}
	if apiCalls != 0 {
		t.Fatalf("API calls = %d, want 0", apiCalls)
	}
}

func TestTemplateAddSendsConsumedItemRequest(t *testing.T) {
	apiCalls := 0
	var authHeader string
	var received struct {
		Products []struct {
			ID              string   `json:"id"`
			ProductID       string   `json:"product_id"`
			Date            string   `json:"date"`
			Daytime         string   `json:"daytime"`
			Amount          float64  `json:"amount"`
			Serving         *string  `json:"serving"`
			ServingQuantity *float64 `json:"serving_quantity"`
		} `json:"products"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiCalls++
		authHeader = r.Header.Get("Authorization")
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

	servingQuantity := 2.5
	configPath := writeConfigFile(t, config.File{
		BaseURL: server.URL,
		Output:  config.DefaultOutput,
		Token: &config.Token{
			AccessToken: "access-token",
			TokenType:   "Bearer",
			ExpiresAt:   time.Now().Add(time.Hour),
		},
		Templates: map[string]config.MealTemplate{
			"weekday-breakfast": {
				ProductID:       "11111111-1111-1111-1111-111111111111",
				Meal:            "breakfast",
				Amount:          100,
				Serving:         "g",
				ServingQuantity: &servingQuantity,
			},
		},
	})

	out, err := executeCLIWithOutput(t, []string{
		"--config", configPath,
		"template", "add", "weekday-breakfast",
		"--date", "2026-06-02",
		"--meal", "lunch",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if apiCalls != 1 {
		t.Fatalf("API calls = %d, want 1", apiCalls)
	}
	if authHeader != "Bearer access-token" {
		t.Fatalf("Authorization = %q, want bearer token", authHeader)
	}
	if len(received.Products) != 1 {
		t.Fatalf("products = %d, want 1", len(received.Products))
	}
	product := received.Products[0]
	if _, err := uuid.Parse(product.ID); err != nil {
		t.Fatalf("entry id = %q, want UUID: %v", product.ID, err)
	}
	if product.ProductID != "11111111-1111-1111-1111-111111111111" || product.Date != "2026-06-02" || product.Daytime != "lunch" || product.Amount != 100 {
		t.Fatalf("product payload = %#v, want template values with meal override", product)
	}
	if product.Serving == nil || *product.Serving != "g" {
		t.Fatalf("serving = %v, want g", product.Serving)
	}
	if product.ServingQuantity == nil || *product.ServingQuantity != 2.5 {
		t.Fatalf("serving quantity = %v, want 2.5", product.ServingQuantity)
	}
	if !strings.Contains(out, product.ID) || !strings.Contains(out, "weekday-breakfast") {
		t.Fatalf("output = %q, want entry id and template name", out)
	}
}

func TestTemplateAddJSONOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	configPath := writeConfigFile(t, config.File{
		BaseURL: server.URL,
		Output:  "json",
		Token: &config.Token{
			AccessToken: "access-token",
			TokenType:   "Bearer",
			ExpiresAt:   time.Now().Add(time.Hour),
		},
		Templates: map[string]config.MealTemplate{
			"snack": {
				ProductID: "11111111-1111-1111-1111-111111111111",
				Meal:      "snack",
				Amount:    42,
			},
		},
	})

	out, err := executeCLIWithOutput(t, []string{"--config", configPath, "template", "add", "snack", "--date", "2026-06-02"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	var result templateAddResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("Unmarshal(%q) error = %v", out, err)
	}
	if _, err := uuid.Parse(result.EntryID); err != nil {
		t.Fatalf("entry_id = %q, want UUID: %v", result.EntryID, err)
	}
	if result.Template != "snack" || result.Date != "2026-06-02" || result.Meal != "snack" || result.ProductID != "11111111-1111-1111-1111-111111111111" || result.Amount != 42 {
		t.Fatalf("result = %#v, want template add result", result)
	}
}

func writeConfigFile(t *testing.T, cfg config.File) string {
	t.Helper()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	return configPath
}
