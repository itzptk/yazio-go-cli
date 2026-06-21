package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/itzptk/yazio-go-cli/internal/config"
	"github.com/itzptk/yazio-go-cli/internal/yazio"
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

func TestAddAcceptsAllowedMealBuckets(t *testing.T) {
	for _, meal := range []string{"breakfast", "lunch", "dinner", "snack"} {
		t.Run(meal, func(t *testing.T) {
			fake := &addCommandClient{}
			configPath := writeCLIConfig(t, "https://example.test/v15")

			err := executeCLIWithClient(t, []string{
				"--config", configPath,
				"add",
				"--product-id", "11111111-1111-1111-1111-111111111111",
				"--meal", meal,
				"--amount", "100",
				"--date", "2026-06-02",
			}, func(string) apiClient { return fake })

			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if fake.addCalls != 1 {
				t.Fatalf("AddConsumedItem calls = %d, want 1", fake.addCalls)
			}
			if fake.addInput.Daytime != meal {
				t.Fatalf("Daytime = %q, want %q", fake.addInput.Daytime, meal)
			}
		})
	}
}

func TestAddRejectsInvalidMealBucketBeforeAPI(t *testing.T) {
	fake := &addCommandClient{}
	configPath := writeCLIConfig(t, "https://example.test/v15")

	err := executeCLIWithClient(t, []string{
		"--config", configPath,
		"add",
		"--product-id", "11111111-1111-1111-1111-111111111111",
		"--meal", "brunch",
		"--amount", "100",
		"--date", "2026-06-02",
	}, func(string) apiClient { return fake })

	if err == nil {
		t.Fatal("Execute() error = nil, want meal bucket validation error")
	}
	if !strings.Contains(err.Error(), "--meal must be one of: breakfast, lunch, dinner, snack") {
		t.Fatalf("error = %q, want meal bucket validation", err)
	}
	if fake.addCalls != 0 {
		t.Fatalf("AddConsumedItem calls = %d, want 0", fake.addCalls)
	}
}

func executeCLI(t *testing.T, args []string) error {
	t.Helper()

	return executeCLIWithClient(t, args, func(baseURL string) apiClient {
		return yazio.NewClient(baseURL)
	})
}

func executeCLIWithClient(t *testing.T, args []string, clientFactory func(string) apiClient) error {
	t.Helper()

	var out bytes.Buffer
	cmd, err := newRootCommand(&out, "dev", clientFactory)
	if err != nil {
		t.Fatalf("newRootCommand() error = %v", err)
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

type addCommandClient struct {
	addCalls int
	addInput yazio.AddConsumedItemRequest
}

func (f *addCommandClient) Login(context.Context, yazio.Credentials) (yazio.Token, error) {
	return yazio.Token{}, errors.New("unexpected Login call")
}

func (f *addCommandClient) Refresh(context.Context, yazio.Token) (yazio.Token, error) {
	return yazio.Token{}, errors.New("unexpected Refresh call")
}

func (f *addCommandClient) GetUser(context.Context, yazio.Token) (yazio.User, error) {
	return yazio.User{}, errors.New("unexpected GetUser call")
}

func (f *addCommandClient) GetDailySummary(context.Context, yazio.Token, time.Time) (yazio.DailySummary, error) {
	return yazio.DailySummary{}, errors.New("unexpected GetDailySummary call")
}

func (f *addCommandClient) GetConsumedItems(context.Context, yazio.Token, time.Time) (yazio.ConsumedItemsResponse, error) {
	return yazio.ConsumedItemsResponse{}, errors.New("unexpected GetConsumedItems call")
}

func (f *addCommandClient) SearchProducts(context.Context, yazio.Token, yazio.SearchOptions) ([]yazio.ProductSearchResult, error) {
	return nil, errors.New("unexpected SearchProducts call")
}

func (f *addCommandClient) AddConsumedItem(_ context.Context, _ yazio.Token, entry yazio.AddConsumedItemRequest) error {
	f.addCalls++
	f.addInput = entry
	return nil
}

func (f *addCommandClient) RemoveConsumedItem(context.Context, yazio.Token, string) error {
	return errors.New("unexpected RemoveConsumedItem call")
}
