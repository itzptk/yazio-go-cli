package cli

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/itzptk/yazio-go-cli/internal/config"
	"github.com/itzptk/yazio-go-cli/internal/yazio"
)

func TestDiaryCSVExportWritesInclusiveDateRangeToStdout(t *testing.T) {
	t.Parallel()

	cfgPath := writeTestConfigWithToken(t)
	fake := &fakeAPIClient{
		consumedByDate: map[string]yazio.ConsumedItemsResponse{
			"2026-06-01": {
				Products: []yazio.ConsumedItem{
					{
						ID:              "entry-1",
						Date:            "2026-06-01",
						Daytime:         "breakfast",
						Type:            "product",
						ProductID:       "product-1",
						Name:            "Banana",
						Producer:        "YAZIO",
						Amount:          100,
						Serving:         stringPtrForTest("g"),
						ServingQuantity: float64PtrForTest(1),
					},
				},
			},
			"2026-06-02": {
				Products: []yazio.ConsumedItem{
					{
						ID:        "entry-2",
						Date:      "2026-06-02",
						Daytime:   "lunch",
						Type:      "product",
						ProductID: "product-2",
						Name:      "Protein Yogurt",
						Amount:    250,
					},
				},
				RecipePortions: []any{
					map[string]any{
						"amount":  1,
						"date":    "2026-06-02",
						"daytime": "dinner",
						"id":      "recipe-1",
						"name":    "Pasta bowl",
						"type":    "recipe_portion",
					},
				},
				SimpleProducts: []any{
					map[string]any{
						"amount":  75,
						"date":    "2026-06-02",
						"daytime": "snack",
						"id":      "simple-1",
						"name":    "Black coffee",
						"serving": "ml",
						"type":    "simple_product",
					},
				},
			},
		},
	}

	var out bytes.Buffer
	cmd, err := newRootCommand(&out, "dev", func(baseURL string) apiClient { return fake })
	if err != nil {
		t.Fatalf("newRootCommand() error = %v", err)
	}
	cmd.SetArgs([]string{"--config", cfgPath, "export", "diary", "--from", "2026-06-01", "--to", "2026-06-02"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	records, err := csv.NewReader(strings.NewReader(out.String())).ReadAll()
	if err != nil {
		t.Fatalf("CSV ReadAll() error = %v\noutput:\n%s", err, out.String())
	}

	want := [][]string{
		{"date", "category", "meal", "entry_id", "type", "product_id", "name", "producer", "amount", "serving", "serving_quantity", "raw_json"},
		{"2026-06-01", "product", "breakfast", "entry-1", "product", "product-1", "Banana", "YAZIO", "100", "g", "1", ""},
		{"2026-06-02", "product", "lunch", "entry-2", "product", "product-2", "Protein Yogurt", "", "250", "", "", ""},
		{"2026-06-02", "recipe_portion", "dinner", "recipe-1", "recipe_portion", "", "Pasta bowl", "", "1", "", "", `{"amount":1,"date":"2026-06-02","daytime":"dinner","id":"recipe-1","name":"Pasta bowl","type":"recipe_portion"}`},
		{"2026-06-02", "simple_product", "snack", "simple-1", "simple_product", "", "Black coffee", "", "75", "ml", "", `{"amount":75,"date":"2026-06-02","daytime":"snack","id":"simple-1","name":"Black coffee","serving":"ml","type":"simple_product"}`},
	}
	if !equalRecords(records, want) {
		t.Fatalf("records = %#v, want %#v", records, want)
	}
	if strings.Join(fake.consumedDates, ",") != "2026-06-01,2026-06-02" {
		t.Fatalf("consumed dates = %#v, want 2026-06-01,2026-06-02", fake.consumedDates)
	}
}

func TestDiaryCSVExportWritesToFileWhenRequested(t *testing.T) {
	t.Parallel()

	cfgPath := writeTestConfigWithToken(t)
	fake := &fakeAPIClient{
		consumedByDate: map[string]yazio.ConsumedItemsResponse{
			"2026-06-03": {
				Products: []yazio.ConsumedItem{
					{ID: "entry-3", Date: "2026-06-03", Daytime: "snack", Type: "product", ProductID: "product-3", Name: "Apple", Amount: 1, Serving: stringPtrForTest("piece")},
				},
			},
		},
	}

	var out bytes.Buffer
	cmd, err := newRootCommand(&out, "dev", func(baseURL string) apiClient { return fake })
	if err != nil {
		t.Fatalf("newRootCommand() error = %v", err)
	}
	exportPath := filepath.Join(t.TempDir(), "diary.csv")
	cmd.SetArgs([]string{"--config", cfgPath, "export", "diary", "2026-06-03", "--file", exportPath})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out.String(), "exported 1 diary entries") || !strings.Contains(out.String(), exportPath) {
		t.Fatalf("output = %q, want entry count and file path", out.String())
	}

	content, err := os.ReadFile(exportPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(content), "2026-06-03,product,snack,entry-3") {
		t.Fatalf("file content = %q, want exported diary row", string(content))
	}
}

func TestDiaryCSVExportRejectsMixedDateArgAndRangeFlags(t *testing.T) {
	t.Parallel()

	cfgPath := writeTestConfigWithToken(t)
	fake := &fakeAPIClient{}
	var out bytes.Buffer
	cmd, err := newRootCommand(&out, "dev", func(baseURL string) apiClient { return fake })
	if err != nil {
		t.Fatalf("newRootCommand() error = %v", err)
	}
	cmd.SetArgs([]string{"--config", cfgPath, "export", "diary", "2026-06-03", "--from", "2026-06-01", "--to", "2026-06-02"})

	err = cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want mixed date/range validation error")
	}
	if !strings.Contains(err.Error(), "pass either a positional date or --from/--to") {
		t.Fatalf("error = %q, want mixed date/range validation", err)
	}
	if len(fake.consumedDates) != 0 {
		t.Fatalf("consumed dates = %#v, want no API calls", fake.consumedDates)
	}
}

func TestDiaryCSVExportRejectsFromAfterTo(t *testing.T) {
	t.Parallel()

	cfgPath := writeTestConfigWithToken(t)
	fake := &fakeAPIClient{}
	var out bytes.Buffer
	cmd, err := newRootCommand(&out, "dev", func(baseURL string) apiClient { return fake })
	if err != nil {
		t.Fatalf("newRootCommand() error = %v", err)
	}
	cmd.SetArgs([]string{"--config", cfgPath, "export", "diary", "--from", "2026-06-04", "--to", "2026-06-03"})

	err = cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want range validation error")
	}
	if !strings.Contains(err.Error(), "--from must be on or before --to") {
		t.Fatalf("error = %q, want from/to validation", err)
	}
}

func TestResolveDiaryExportDatesAllows366DayRange(t *testing.T) {
	t.Parallel()

	dates, err := resolveDiaryExportDates(nil, "2024-01-01", "2024-12-31")
	if err != nil {
		t.Fatalf("resolveDiaryExportDates() error = %v, want nil", err)
	}
	if len(dates) != maxDiaryExportDays {
		t.Fatalf("len(dates) = %d, want %d", len(dates), maxDiaryExportDays)
	}
	if got := dates[0].Format("2006-01-02"); got != "2024-01-01" {
		t.Fatalf("first date = %s, want 2024-01-01", got)
	}
	if got := dates[len(dates)-1].Format("2006-01-02"); got != "2024-12-31" {
		t.Fatalf("last date = %s, want 2024-12-31", got)
	}
}

func TestResolveDiaryExportDatesRejects367DayRange(t *testing.T) {
	t.Parallel()

	_, err := resolveDiaryExportDates(nil, "2024-01-01", "2025-01-01")
	if err == nil {
		t.Fatal("resolveDiaryExportDates() error = nil, want range validation error")
	}
	if !strings.Contains(err.Error(), "diary export range cannot exceed 366 days") {
		t.Fatalf("error = %q, want range limit validation", err)
	}
}

func writeTestConfigWithToken(t *testing.T) string {
	t.Helper()
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := config.Save(cfgPath, config.File{
		BaseURL: "https://example.test/v15",
		Output:  "table",
		Token: &config.Token{
			AccessToken:  "access",
			RefreshToken: "refresh",
			TokenType:    "Bearer",
			ExpiresAt:    time.Date(2099, 6, 3, 12, 0, 0, 0, time.UTC),
		},
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	return cfgPath
}

type fakeAPIClient struct {
	consumedByDate map[string]yazio.ConsumedItemsResponse
	consumedDates  []string
}

func (f *fakeAPIClient) Login(context.Context, yazio.Credentials) (yazio.Token, error) {
	return yazio.Token{}, errors.New("unexpected Login call")
}

func (f *fakeAPIClient) Refresh(context.Context, yazio.Token) (yazio.Token, error) {
	return yazio.Token{}, errors.New("unexpected Refresh call")
}

func (f *fakeAPIClient) GetUser(context.Context, yazio.Token) (yazio.User, error) {
	return yazio.User{}, errors.New("unexpected GetUser call")
}

func (f *fakeAPIClient) GetDailySummary(context.Context, yazio.Token, time.Time) (yazio.DailySummary, error) {
	return yazio.DailySummary{}, errors.New("unexpected GetDailySummary call")
}

func (f *fakeAPIClient) GetConsumedItems(_ context.Context, _ yazio.Token, date time.Time) (yazio.ConsumedItemsResponse, error) {
	key := date.Format("2006-01-02")
	f.consumedDates = append(f.consumedDates, key)
	if f.consumedByDate == nil {
		return yazio.ConsumedItemsResponse{}, nil
	}
	return f.consumedByDate[key], nil
}

func (f *fakeAPIClient) SearchProducts(context.Context, yazio.Token, yazio.SearchOptions) ([]yazio.ProductSearchResult, error) {
	return nil, errors.New("unexpected SearchProducts call")
}

func (f *fakeAPIClient) AddConsumedItem(context.Context, yazio.Token, yazio.AddConsumedItemRequest) error {
	return errors.New("unexpected AddConsumedItem call")
}

func (f *fakeAPIClient) RemoveConsumedItem(context.Context, yazio.Token, string) error {
	return errors.New("unexpected RemoveConsumedItem call")
}

func stringPtrForTest(v string) *string    { return &v }
func float64PtrForTest(v float64) *float64 { return &v }

func equalRecords(got, want [][]string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if len(got[i]) != len(want[i]) {
			return false
		}
		for j := range got[i] {
			if got[i][j] != want[i][j] {
				return false
			}
		}
	}
	return true
}
