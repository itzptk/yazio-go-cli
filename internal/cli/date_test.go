package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/itzptk/yazio-go-cli/internal/yazio"
	"github.com/spf13/cobra"
)

func TestDiaryCommandsDefaultToLocalCalendarDate(t *testing.T) {
	t.Parallel()

	loc := time.FixedZone("UTC+14", 14*60*60)
	now := func() time.Time {
		return time.Date(2026, 1, 1, 0, 30, 0, 0, loc)
	}

	tests := []struct {
		name string
		args []string
		got  func(*recordingAPIClient) []string
	}{
		{
			name: "summary",
			args: []string{"summary"},
			got:  func(client *recordingAPIClient) []string { return client.summaryDates },
		},
		{
			name: "consumed",
			args: []string{"consumed"},
			got:  func(client *recordingAPIClient) []string { return client.consumedDates },
		},
		{
			name: "export diary",
			args: []string{"export", "diary"},
			got:  func(client *recordingAPIClient) []string { return client.consumedDates },
		},
		{
			name: "add",
			args: []string{"add", "--product-id", "11111111-1111-1111-1111-111111111111", "--meal", "breakfast", "--amount", "100"},
			got:  func(client *recordingAPIClient) []string { return client.addDates },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &recordingAPIClient{}
			cmd := newTestRootCommandWithClock(t, client, now)
			cmd.SetArgs(append([]string{"--config", writeTestConfigWithToken(t)}, tt.args...))

			if err := cmd.Execute(); err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if strings.Join(tt.got(client), ",") != "2026-01-01" {
				t.Fatalf("dates = %#v, want 2026-01-01", tt.got(client))
			}
		})
	}
}

func TestDiaryCommandsPassExplicitCalendarDate(t *testing.T) {
	t.Parallel()

	now := func() time.Time {
		return time.Date(2026, 1, 1, 0, 30, 0, 0, time.FixedZone("UTC+14", 14*60*60))
	}

	tests := []struct {
		name string
		args []string
		got  func(*recordingAPIClient) []string
	}{
		{
			name: "summary",
			args: []string{"summary", "2026-06-02"},
			got:  func(client *recordingAPIClient) []string { return client.summaryDates },
		},
		{
			name: "consumed",
			args: []string{"consumed", "2026-06-02"},
			got:  func(client *recordingAPIClient) []string { return client.consumedDates },
		},
		{
			name: "export diary",
			args: []string{"export", "diary", "2026-06-02"},
			got:  func(client *recordingAPIClient) []string { return client.consumedDates },
		},
		{
			name: "add",
			args: []string{"add", "--product-id", "11111111-1111-1111-1111-111111111111", "--meal", "breakfast", "--amount", "100", "--date", "2026-06-02"},
			got:  func(client *recordingAPIClient) []string { return client.addDates },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &recordingAPIClient{}
			cmd := newTestRootCommandWithClock(t, client, now)
			cmd.SetArgs(append([]string{"--config", writeTestConfigWithToken(t)}, tt.args...))

			if err := cmd.Execute(); err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if strings.Join(tt.got(client), ",") != "2026-06-02" {
				t.Fatalf("dates = %#v, want 2026-06-02", tt.got(client))
			}
		})
	}
}

func newTestRootCommandWithClock(t *testing.T, client *recordingAPIClient, now func() time.Time) *cobra.Command {
	t.Helper()

	var out bytes.Buffer
	cmd, err := newRootCommandWithClock(&out, "dev", func(baseURL string, _ yazio.OAuthCredentials) apiClient { return client }, now)
	if err != nil {
		t.Fatalf("newRootCommandWithClock() error = %v", err)
	}
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	return cmd
}

type recordingAPIClient struct {
	summaryDates  []string
	consumedDates []string
	addDates      []string
}

func (f *recordingAPIClient) Login(context.Context, yazio.Credentials) (yazio.Token, error) {
	return yazio.Token{}, errors.New("unexpected Login call")
}

func (f *recordingAPIClient) Refresh(context.Context, yazio.Token) (yazio.Token, error) {
	return yazio.Token{}, errors.New("unexpected Refresh call")
}

func (f *recordingAPIClient) GetUser(context.Context, yazio.Token) (yazio.User, error) {
	return yazio.User{}, errors.New("unexpected GetUser call")
}

func (f *recordingAPIClient) GetDailySummary(_ context.Context, _ yazio.Token, date time.Time) (yazio.DailySummary, error) {
	f.summaryDates = append(f.summaryDates, date.Format("2006-01-02"))
	return yazio.DailySummary{}, nil
}

func (f *recordingAPIClient) GetConsumedItems(_ context.Context, _ yazio.Token, date time.Time) (yazio.ConsumedItemsResponse, error) {
	f.consumedDates = append(f.consumedDates, date.Format("2006-01-02"))
	return yazio.ConsumedItemsResponse{}, nil
}

func (f *recordingAPIClient) SearchProducts(context.Context, yazio.Token, yazio.SearchOptions) ([]yazio.ProductSearchResult, error) {
	return nil, errors.New("unexpected SearchProducts call")
}

func (f *recordingAPIClient) AddConsumedItem(_ context.Context, _ yazio.Token, entry yazio.AddConsumedItemRequest) error {
	f.addDates = append(f.addDates, entry.Date.Format("2006-01-02"))
	return nil
}

func (f *recordingAPIClient) RemoveConsumedItem(context.Context, yazio.Token, string) error {
	return errors.New("unexpected RemoveConsumedItem call")
}
