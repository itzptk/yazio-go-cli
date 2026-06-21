package yazio

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func testOAuthCredentials() OAuthCredentials {
	return OAuthCredentials{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
	}
}

func TestLogin(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth/token" {
			t.Fatalf("path = %q, want /oauth/token", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("method = %q, want POST", r.Method)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		if body["username"] != "user@example.com" {
			t.Fatalf("username = %v, want user@example.com", body["username"])
		}
		if body["password"] != "super-secret" {
			t.Fatalf("password = %v, want super-secret", body["password"])
		}
		if body["grant_type"] != "password" {
			t.Fatalf("grant_type = %v, want password", body["grant_type"])
		}
		if body["client_id"] != "test-client-id" {
			t.Fatalf("client_id = %v, want test-client-id", body["client_id"])
		}
		if body["client_secret"] != "test-client-secret" {
			t.Fatalf("client_secret = %v, want test-client-secret", body["client_secret"])
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"access_token":"access","refresh_token":"refresh","token_type":"Bearer","expires_in":3600}`)
	}))
	defer server.Close()

	client := NewClient(server.URL, WithOAuthCredentials(testOAuthCredentials()))
	got, err := client.Login(context.Background(), Credentials{Email: "user@example.com", Password: "super-secret"})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	if got.AccessToken != "access" {
		t.Fatalf("AccessToken = %q, want access", got.AccessToken)
	}
	if got.RefreshToken != "refresh" {
		t.Fatalf("RefreshToken = %q, want refresh", got.RefreshToken)
	}
	if got.TokenType != "Bearer" {
		t.Fatalf("TokenType = %q, want Bearer", got.TokenType)
	}
	if time.Until(got.ExpiresAt) <= 59*time.Minute {
		t.Fatalf("ExpiresAt = %s, want about one hour in the future", got.ExpiresAt)
	}
}

func TestRefreshSendsOAuthCredentials(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth/token" {
			t.Fatalf("path = %q, want /oauth/token", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("method = %q, want POST", r.Method)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		if body["grant_type"] != "refresh_token" {
			t.Fatalf("grant_type = %v, want refresh_token", body["grant_type"])
		}
		if body["refresh_token"] != "refresh-input" {
			t.Fatalf("refresh_token = %v, want refresh-input", body["refresh_token"])
		}
		if body["client_id"] != "test-client-id" {
			t.Fatalf("client_id = %v, want test-client-id", body["client_id"])
		}
		if body["client_secret"] != "test-client-secret" {
			t.Fatalf("client_secret = %v, want test-client-secret", body["client_secret"])
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"access_token":"new-access","refresh_token":"new-refresh","token_type":"Bearer","expires_in":3600}`)
	}))
	defer server.Close()

	client := NewClient(server.URL, WithOAuthCredentials(testOAuthCredentials()))
	got, err := client.Refresh(context.Background(), Token{RefreshToken: "refresh-input"})
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	if got.AccessToken != "new-access" {
		t.Fatalf("AccessToken = %q, want new-access", got.AccessToken)
	}
	if got.RefreshToken != "new-refresh" {
		t.Fatalf("RefreshToken = %q, want new-refresh", got.RefreshToken)
	}
}

func TestLoginRequiresOAuthCredentialsBeforeRequest(t *testing.T) {
	t.Parallel()

	var requests int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requests, 1)
		http.Error(w, "unexpected request", http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.Login(context.Background(), Credentials{Email: "user@example.com", Password: "super-secret"})
	if err == nil {
		t.Fatal("Login() error = nil, want missing credentials error")
	}
	if !strings.Contains(err.Error(), "missing YAZIO OAuth credentials") {
		t.Fatalf("Login() error = %q, want missing OAuth credentials", err)
	}
	if got := atomic.LoadInt64(&requests); got != 0 {
		t.Fatalf("requests = %d, want 0", got)
	}
}

func TestRefreshRequiresOAuthCredentialsBeforeRequest(t *testing.T) {
	t.Parallel()

	var requests int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requests, 1)
		http.Error(w, "unexpected request", http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.Refresh(context.Background(), Token{RefreshToken: "refresh-input"})
	if err == nil {
		t.Fatal("Refresh() error = nil, want missing credentials error")
	}
	if !strings.Contains(err.Error(), "missing YAZIO OAuth credentials") {
		t.Fatalf("Refresh() error = %q, want missing OAuth credentials", err)
	}
	if got := atomic.LoadInt64(&requests); got != 0 {
		t.Fatalf("requests = %d, want 0", got)
	}
}

func TestGetDailySummary(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user/widgets/daily-summary" {
			t.Fatalf("path = %q, want /user/widgets/daily-summary", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer access" {
			t.Fatalf("Authorization = %q, want Bearer access", r.Header.Get("Authorization"))
		}
		if r.URL.Query().Get("date") != "2026-06-02" {
			t.Fatalf("date = %q, want 2026-06-02", r.URL.Query().Get("date"))
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
			"activity_energy": 492,
			"consume_activity_energy": true,
			"steps": 8132,
			"water_intake": 2000,
			"goals": {
				"energy.energy": 3064.7,
				"water": 2000,
				"activity.step": 10000,
				"nutrient.protein": 156.9,
				"nutrient.fat": 83.0,
				"nutrient.carb": 282.4,
				"bodyvalue.weight": 65
			},
			"units": {
				"unit_mass": "kg",
				"unit_energy": "kcal",
				"unit_serving": "g",
				"unit_length": "cm"
			},
			"meals": {
				"breakfast": {"energy_goal": 919.4, "nutrients": {"energy.energy": 727.1, "nutrient.carb": 68.2, "nutrient.fat": 33.2, "nutrient.protein": 33.6}},
				"lunch": {"energy_goal": 1225.9, "nutrients": {"energy.energy": 800.0, "nutrient.carb": 80.0, "nutrient.fat": 20.0, "nutrient.protein": 40.0}},
				"dinner": {"energy_goal": 766.2, "nutrients": {"energy.energy": 500.0, "nutrient.carb": 50.0, "nutrient.fat": 18.0, "nutrient.protein": 35.0}},
				"snack": {"energy_goal": 153.2, "nutrients": {"energy.energy": 120.0, "nutrient.carb": 10.0, "nutrient.fat": 4.0, "nutrient.protein": 5.0}}
			},
			"user": {
				"start_weight": 64.1,
				"current_weight": 64.5,
				"goal": "build_muscle",
				"sex": "male"
			},
			"active_fasting_countdown_template_key": null
		}`)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	token := Token{AccessToken: "access"}
	date := time.Date(2026, 6, 2, 14, 0, 0, 0, time.UTC)

	got, err := client.GetDailySummary(context.Background(), token, date)
	if err != nil {
		t.Fatalf("GetDailySummary() error = %v", err)
	}

	if got.WaterIntake != 2000 {
		t.Fatalf("WaterIntake = %d, want 2000", got.WaterIntake)
	}
	if got.Steps != 8132 {
		t.Fatalf("Steps = %d, want 8132", got.Steps)
	}
	if got.Meals.Breakfast.EnergyGoal != 919.4 {
		t.Fatalf("Breakfast.EnergyGoal = %v, want 919.4", got.Meals.Breakfast.EnergyGoal)
	}
}

func TestSearchProducts(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/products/search" {
			t.Fatalf("path = %q, want /products/search", r.URL.Path)
		}
		query := r.URL.Query()
		if query.Get("query") != "banana" {
			t.Fatalf("query = %q, want banana", query.Get("query"))
		}
		if query.Get("countries") != "DE,US" {
			t.Fatalf("countries = %q, want DE,US", query.Get("countries"))
		}
		if query.Get("locales") != "en_US,de_US" {
			t.Fatalf("locales = %q, want en_US,de_US", query.Get("locales"))
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[{"score": 0.99, "name":"Banana", "product_id":"11111111-1111-1111-1111-111111111111", "serving":"g", "serving_quantity":100, "amount":100, "base_unit":"g", "producer":"YAZIO", "is_verified":true, "nutrients":{"energy.energy":89, "nutrient.carb":23, "nutrient.protein":1.1, "nutrient.fat":0.3}, "countries":["DE"], "language":"en"}]`)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	got, err := client.SearchProducts(context.Background(), Token{AccessToken: "access"}, SearchOptions{Query: "banana"})
	if err != nil {
		t.Fatalf("SearchProducts() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(got))
	}
	if got[0].Name != "Banana" {
		t.Fatalf("Name = %q, want Banana", got[0].Name)
	}
}

func TestAddConsumedItem(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user/consumed-items" {
			t.Fatalf("path = %q, want /user/consumed-items", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("method = %q, want POST", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer access" {
			t.Fatalf("Authorization = %q, want Bearer access", r.Header.Get("Authorization"))
		}
		if ct := r.Header.Get("Content-Type"); !strings.Contains(ct, "application/json") {
			t.Fatalf("Content-Type = %q, want application/json", ct)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("ReadAll() error = %v", err)
		}
		if !strings.Contains(string(body), `"product_id":"11111111-1111-1111-1111-111111111111"`) {
			t.Fatalf("body = %s, want product_id", body)
		}
		if !strings.Contains(string(body), `"daytime":"breakfast"`) {
			t.Fatalf("body = %s, want daytime", body)
		}
		if !strings.Contains(string(body), `"date":"2026-06-02"`) {
			t.Fatalf("body = %s, want date", body)
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	err := client.AddConsumedItem(context.Background(), Token{AccessToken: "access"}, AddConsumedItemRequest{
		ID:              "22222222-2222-2222-2222-222222222222",
		ProductID:       "11111111-1111-1111-1111-111111111111",
		Date:            time.Date(2026, 6, 2, 15, 0, 0, 0, time.UTC),
		Daytime:         "breakfast",
		Amount:          120,
		Serving:         stringPtr("g"),
		ServingQuantity: float64Ptr(1),
	})
	if err != nil {
		t.Fatalf("AddConsumedItem() error = %v", err)
	}
}

func TestRemoveConsumedItem(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("method = %q, want DELETE", r.Method)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("ReadAll() error = %v", err)
		}
		if strings.TrimSpace(string(body)) != `["33333333-3333-3333-3333-333333333333"]` {
			t.Fatalf("body = %s, want entry id array", body)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	if err := client.RemoveConsumedItem(context.Background(), Token{AccessToken: "access"}, "33333333-3333-3333-3333-333333333333"); err != nil {
		t.Fatalf("RemoveConsumedItem() error = %v", err)
	}
}

func TestGetRequestsRetryTransientStatusBeforeSuccess(t *testing.T) {
	t.Parallel()

	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if r.Method != http.MethodGet {
			t.Fatalf("method = %q, want GET", r.Method)
		}
		if r.URL.Path != "/user" {
			t.Fatalf("path = %q, want /user", r.URL.Path)
		}
		if attempts == 1 {
			http.Error(w, "temporary outage", http.StatusServiceUnavailable)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"email":"user@example.com"}`)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	got, err := client.GetUser(context.Background(), Token{AccessToken: "access"})
	if err != nil {
		t.Fatalf("GetUser() error = %v", err)
	}
	if got.Email != "user@example.com" {
		t.Fatalf("Email = %q, want user@example.com", got.Email)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
}

func TestGetRequestsRetryTransientNetworkErrorBeforeSuccess(t *testing.T) {
	t.Parallel()

	attempts := 0
	client := NewClient("https://api.example.test")
	client.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		attempts++
		if req.Method != http.MethodGet {
			t.Fatalf("method = %q, want GET", req.Method)
		}
		if req.URL.Path != "/user" {
			t.Fatalf("path = %q, want /user", req.URL.Path)
		}
		if attempts == 1 {
			return nil, context.DeadlineExceeded
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"email":"user@example.com"}`)),
			Request:    req,
		}, nil
	})}

	got, err := client.GetUser(context.Background(), Token{AccessToken: "access"})
	if err != nil {
		t.Fatalf("GetUser() error = %v", err)
	}
	if got.Email != "user@example.com" {
		t.Fatalf("Email = %q, want user@example.com", got.Email)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
}

func TestGetRequestsReportAttemptsAfterTransientStatusFailures(t *testing.T) {
	t.Parallel()

	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		http.Error(w, "temporary outage", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.GetUser(context.Background(), Token{AccessToken: "access"})
	if err == nil {
		t.Fatal("GetUser() error = nil, want retry exhaustion error")
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
	message := err.Error()
	for _, want := range []string{"GET /user", "attempt 3/3", "503"} {
		if !strings.Contains(message, want) {
			t.Fatalf("GetUser() error = %q, want it to contain %q", message, want)
		}
	}
}

func TestWriteRequestsDoNotRetryTransientStatus(t *testing.T) {
	t.Parallel()

	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if r.Method != http.MethodPost {
			t.Fatalf("method = %q, want POST", r.Method)
		}
		http.Error(w, "temporary outage", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	err := client.AddConsumedItem(context.Background(), Token{AccessToken: "access"}, AddConsumedItemRequest{
		ID:        "22222222-2222-2222-2222-222222222222",
		ProductID: "11111111-1111-1111-1111-111111111111",
		Date:      time.Date(2026, 6, 2, 15, 0, 0, 0, time.UTC),
		Daytime:   "breakfast",
		Amount:    120,
	})
	if err == nil {
		t.Fatal("AddConsumedItem() error = nil, want transient status error")
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
	message := err.Error()
	if !strings.Contains(message, "POST /user/consumed-items") {
		t.Fatalf("AddConsumedItem() error = %q, want endpoint context", message)
	}
	if strings.Contains(message, "attempt") {
		t.Fatalf("AddConsumedItem() error = %q, want no retry attempt marker for single-shot write", message)
	}
}

func TestBuildURL(t *testing.T) {
	t.Parallel()

	client := NewClient("https://yzapi.yazio.com/v15/")
	values := url.Values{}
	values.Set("date", "2026-06-02")

	got, err := client.buildURL("/user/widgets/daily-summary", values)
	if err != nil {
		t.Fatalf("buildURL() error = %v", err)
	}
	want := "https://yzapi.yazio.com/v15/user/widgets/daily-summary?date=2026-06-02"
	if got != want {
		t.Fatalf("buildURL() = %q, want %q", got, want)
	}
}

func TestInvalidBaseURLReturnsError(t *testing.T) {
	t.Parallel()

	client := NewClient("http://[::1")

	_, err := client.GetUser(context.Background(), Token{AccessToken: "access"})
	if err == nil {
		t.Fatal("GetUser() error = nil, want invalid base URL error")
	}
	if !strings.Contains(err.Error(), "invalid base URL") {
		t.Fatalf("GetUser() error = %q, want invalid base URL", err)
	}
}

func stringPtr(v string) *string    { return &v }
func float64Ptr(v float64) *float64 { return &v }

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
