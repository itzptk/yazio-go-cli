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
	defaultClientID     = "1_4hiybetvfksgw40o0sog4s884kwc840wwso8go4k8c04goo4c"
	defaultClientSecret = "6rok2m65xuskgkgogw40wkkk8sw0osg84s8cggsc4woos4s8o"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL == "" {
		baseURL = "https://yzapi.yazio.com/v15"
	}
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
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
	return c.fetchToken(ctx, map[string]any{
		"client_id":     defaultClientID,
		"client_secret": defaultClientSecret,
		"username":      creds.Email,
		"password":      creds.Password,
		"grant_type":    "password",
	})
}

func (c *Client) Refresh(ctx context.Context, token Token) (Token, error) {
	return c.fetchToken(ctx, map[string]any{
		"client_id":     defaultClientID,
		"client_secret": defaultClientSecret,
		"refresh_token": token.RefreshToken,
		"grant_type":    "refresh_token",
	})
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
		locales = []string{"en_US", "de_US"}
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
	var body io.Reader
	if requestBody != nil {
		payload, err := json.Marshal(requestBody)
		if err != nil {
			return err
		}
		body = bytes.NewReader(payload)
	}

	requestURL, err := c.buildURL(endpoint, query)
	if err != nil {
		return err
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
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		msg := strings.TrimSpace(string(body))
		if msg == "" {
			msg = http.StatusText(resp.StatusCode)
		}
		return fmt.Errorf("yazio api %s %s: %s", method, endpoint, msg)
	}
	if responseBody == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(responseBody)
}

func formatDate(date time.Time) string {
	return date.UTC().Format("2006-01-02")
}
