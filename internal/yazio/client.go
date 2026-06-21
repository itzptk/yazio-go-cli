package yazio

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

const (
	getRequestMaxAttempts = 3
	getRetryBaseDelay     = 100 * time.Millisecond

	missingOAuthCredentialsHint = "set oauth.client_id/oauth.client_secret in config or YAZIO_CLIENT_ID/YAZIO_CLIENT_SECRET"
)

type OAuthCredentials struct {
	ClientID     string
	ClientSecret string
}

type ClientOption func(*Client)

type Client struct {
	baseURL          string
	httpClient       *http.Client
	oauthCredentials OAuthCredentials
}

func WithOAuthCredentials(credentials OAuthCredentials) ClientOption {
	return func(c *Client) {
		c.oauthCredentials = credentials
	}
}

func NewClient(baseURL string, opts ...ClientOption) *Client {
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL == "" {
		baseURL = "https://yzapi.yazio.com/v15"
	}
	client := &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
	for _, opt := range opts {
		opt(client)
	}
	return client
}

func (c *Client) buildURL(endpoint string, query url.Values) (string, error) {
	base, err := url.Parse(c.baseURL + "/")
	if err != nil {
		return "", fmt.Errorf("invalid base URL %q: %w", c.baseURL, err)
	}
	if base.Scheme != "http" && base.Scheme != "https" {
		return "", fmt.Errorf("invalid base URL %q: expected http or https URL", c.baseURL)
	}
	if base.Host == "" {
		return "", fmt.Errorf("invalid base URL %q: expected host", c.baseURL)
	}
	base.Path = path.Join(base.Path, strings.TrimPrefix(endpoint, "/"))
	if len(query) > 0 {
		base.RawQuery = query.Encode()
	}
	return base.String(), nil
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
}

func (c *Client) Login(ctx context.Context, creds Credentials) (Token, error) {
	oauthCredentials, err := c.validOAuthCredentials()
	if err != nil {
		return Token{}, err
	}
	return c.fetchToken(ctx, map[string]any{
		"client_id":     oauthCredentials.ClientID,
		"client_secret": oauthCredentials.ClientSecret,
		"username":      creds.Email,
		"password":      creds.Password,
		"grant_type":    "password",
	})
}

func (c *Client) Refresh(ctx context.Context, token Token) (Token, error) {
	oauthCredentials, err := c.validOAuthCredentials()
	if err != nil {
		return Token{}, err
	}
	return c.fetchToken(ctx, map[string]any{
		"client_id":     oauthCredentials.ClientID,
		"client_secret": oauthCredentials.ClientSecret,
		"refresh_token": token.RefreshToken,
		"grant_type":    "refresh_token",
	})
}

func (c *Client) validOAuthCredentials() (OAuthCredentials, error) {
	credentials := OAuthCredentials{
		ClientID:     strings.TrimSpace(c.oauthCredentials.ClientID),
		ClientSecret: strings.TrimSpace(c.oauthCredentials.ClientSecret),
	}
	var missing []string
	if credentials.ClientID == "" {
		missing = append(missing, "client ID")
	}
	if credentials.ClientSecret == "" {
		missing = append(missing, "client secret")
	}
	if len(missing) > 0 {
		return OAuthCredentials{}, fmt.Errorf("missing YAZIO OAuth credentials: %s required (%s)", strings.Join(missing, " and "), missingOAuthCredentialsHint)
	}
	return credentials, nil
}

func (c *Client) fetchToken(ctx context.Context, payload map[string]any) (Token, error) {
	var resp tokenResponse
	if err := c.doJSON(ctx, http.MethodPost, "/oauth/token", Token{}, nil, payload, &resp); err != nil {
		return Token{}, err
	}
	return Token{
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		TokenType:    resp.TokenType,
		ExpiresAt:    time.Now().Add(time.Duration(resp.ExpiresIn) * time.Second),
	}, nil
}

func (c *Client) GetUser(ctx context.Context, token Token) (User, error) {
	var user User
	if err := c.doJSON(ctx, http.MethodGet, "/user", token, nil, nil, &user); err != nil {
		return User{}, err
	}
	return user, nil
}

func (c *Client) GetDailySummary(ctx context.Context, token Token, date time.Time) (DailySummary, error) {
	var summary DailySummary
	values := url.Values{}
	values.Set("date", formatDate(date))
	if err := c.doJSON(ctx, http.MethodGet, "/user/widgets/daily-summary", token, values, nil, &summary); err != nil {
		return DailySummary{}, err
	}
	return summary, nil
}

func (c *Client) GetConsumedItems(ctx context.Context, token Token, date time.Time) (ConsumedItemsResponse, error) {
	var resp ConsumedItemsResponse
	values := url.Values{}
	values.Set("date", formatDate(date))
	if err := c.doJSON(ctx, http.MethodGet, "/user/consumed-items", token, values, nil, &resp); err != nil {
		return ConsumedItemsResponse{}, err
	}
	return resp, nil
}

func (c *Client) SearchProducts(ctx context.Context, token Token, opts SearchOptions) ([]ProductSearchResult, error) {
	values := url.Values{}
	values.Set("query", opts.Query)
	sex := opts.Sex
	if sex == "" {
		sex = "male"
	}
	values.Set("sex", sex)
	countries := opts.Countries
	if len(countries) == 0 {
		countries = []string{"DE", "US"}
	}
	values.Set("countries", strings.Join(countries, ","))
	locales := opts.Locales
	if len(locales) == 0 {
		locales = []string{"en_US", "de_DE"}
	}
	values.Set("locales", strings.Join(locales, ","))

	var results []ProductSearchResult
	if err := c.doJSON(ctx, http.MethodGet, "/products/search", token, values, nil, &results); err != nil {
		return nil, err
	}
	return results, nil
}

func (c *Client) AddConsumedItem(ctx context.Context, token Token, entry AddConsumedItemRequest) error {
	payload := map[string]any{
		"recipe_portions": []any{},
		"simple_products": []any{},
		"products": []map[string]any{
			{
				"id":               entry.ID,
				"product_id":       entry.ProductID,
				"date":             formatDate(entry.Date),
				"daytime":          entry.Daytime,
				"amount":           entry.Amount,
				"serving":          entry.Serving,
				"serving_quantity": entry.ServingQuantity,
			},
		},
	}
	return c.doJSON(ctx, http.MethodPost, "/user/consumed-items", token, nil, payload, nil)
}

func (c *Client) RemoveConsumedItem(ctx context.Context, token Token, entryID string) error {
	return c.doJSON(ctx, http.MethodDelete, "/user/consumed-items", token, nil, []string{entryID}, nil)
}

func (c *Client) doJSON(ctx context.Context, method, endpoint string, token Token, query url.Values, requestBody any, responseBody any) error {
	var payload []byte
	if requestBody != nil {
		var err error
		payload, err = json.Marshal(requestBody)
		if err != nil {
			return err
		}
	}

	requestURL, err := c.buildURL(endpoint, query)
	if err != nil {
		return err
	}

	maxAttempts := maxAttemptsForMethod(method)
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		var body io.Reader
		if payload != nil {
			body = bytes.NewReader(payload)
		}

		req, err := http.NewRequestWithContext(ctx, method, requestURL, body)
		if err != nil {
			return err
		}
		if requestBody != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if token.AccessToken != "" {
			typeName := token.TokenType
			if typeName == "" {
				typeName = "Bearer"
			}
			req.Header.Set("Authorization", fmt.Sprintf("%s %s", typeName, token.AccessToken))
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			if shouldRetryRequest(ctx, method, attempt, maxAttempts) {
				if waitErr := waitForRetry(ctx, attempt); waitErr != nil {
					return formatRetryError(method, endpoint, waitErr, attempt, maxAttempts)
				}
				continue
			}
			return formatRetryError(method, endpoint, err, attempt, maxAttempts)
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			msg := strings.TrimSpace(string(body))
			if msg == "" {
				msg = http.StatusText(resp.StatusCode)
			}
			if isTransientStatus(resp.StatusCode) && shouldRetryRequest(ctx, method, attempt, maxAttempts) {
				if waitErr := waitForRetry(ctx, attempt); waitErr != nil {
					return formatRetryError(method, endpoint, waitErr, attempt, maxAttempts)
				}
				continue
			}
			return formatStatusError(method, endpoint, msg, resp.StatusCode, attempt, maxAttempts)
		}

		if responseBody == nil || resp.StatusCode == http.StatusNoContent {
			_ = resp.Body.Close()
			return nil
		}
		err = json.NewDecoder(resp.Body).Decode(responseBody)
		_ = resp.Body.Close()
		return err
	}

	return fmt.Errorf("yazio api %s %s: retry attempts exhausted", method, endpoint)
}

func maxAttemptsForMethod(method string) int {
	if method == http.MethodGet {
		return getRequestMaxAttempts
	}
	return 1
}

func shouldRetryRequest(ctx context.Context, method string, attempt int, maxAttempts int) bool {
	return method == http.MethodGet && attempt < maxAttempts && ctx.Err() == nil
}

func isTransientStatus(statusCode int) bool {
	switch statusCode {
	case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func waitForRetry(ctx context.Context, attempt int) error {
	delay := getRetryBaseDelay * time.Duration(1<<(attempt-1))
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func formatRetryError(method, endpoint string, err error, attempt int, maxAttempts int) error {
	if maxAttempts > 1 {
		return fmt.Errorf("yazio api %s %s: %w (attempt %d/%d)", method, endpoint, err, attempt, maxAttempts)
	}
	return fmt.Errorf("yazio api %s %s: %w", method, endpoint, err)
}

func formatStatusError(method, endpoint string, message string, statusCode int, attempt int, maxAttempts int) error {
	if maxAttempts > 1 && isTransientStatus(statusCode) {
		return fmt.Errorf("yazio api %s %s: %s (attempt %d/%d, status %d)", method, endpoint, message, attempt, maxAttempts, statusCode)
	}
	return fmt.Errorf("yazio api %s %s: %s", method, endpoint, message)
}

func formatDate(date time.Time) string {
	return date.Format("2006-01-02")
}
