package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/itzptk/yazio-go-cli/internal/yazio"
)

func TestConsumedTableIncludesAllDiaryItemTypes(t *testing.T) {
	t.Parallel()

	cfgPath := writeTestConfigWithToken(t)
	fake := &fakeAPIClient{
		consumedByDate: map[string]yazio.ConsumedItemsResponse{
			"2026-06-02": {
				Products: []yazio.ConsumedItem{
					{
						ID:        "entry-1",
						Daytime:   "breakfast",
						ProductID: "product-1",
						Amount:    100,
						Serving:   stringPtrForTest("g"),
					},
				},
				RecipePortions: []any{
					map[string]any{
						"amount":    1,
						"daytime":   "dinner",
						"id":        "recipe-entry-1",
						"recipe_id": "recipe-1",
						"serving":   "portion",
					},
				},
				SimpleProducts: []any{
					map[string]any{
						"amount":            75,
						"daytime":           "snack",
						"id":                "simple-entry-1",
						"serving":           "ml",
						"simple_product_id": "simple-1",
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
	cmd.SetArgs([]string{"--config", cfgPath, "consumed", "2026-06-02"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	assertTableFields(t, out.String(), []string{"ENTRY", "ID", "MEAL", "PRODUCT", "ID", "AMOUNT", "SERVING"})
	assertTableFields(t, out.String(), []string{"entry-1", "breakfast", "product-1", "100.00", "g"})
	assertTableFields(t, out.String(), []string{"recipe-entry-1", "dinner", "recipe-1", "1.00", "portion"})
	assertTableFields(t, out.String(), []string{"simple-entry-1", "snack", "simple-1", "75.00", "ml"})
	if strings.Join(fake.consumedDates, ",") != "2026-06-02" {
		t.Fatalf("consumed dates = %#v, want 2026-06-02", fake.consumedDates)
	}
}

func assertTableFields(t *testing.T, output string, want []string) {
	t.Helper()

	for _, line := range strings.Split(output, "\n") {
		if equalStrings(strings.Fields(line), want) {
			return
		}
	}
	t.Fatalf("output missing table row %#v:\n%s", want, output)
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
