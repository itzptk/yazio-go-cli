package cli

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRemoveRejectsInvalidEntryIDBeforeAPI(t *testing.T) {
	apiCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiCalls++
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	configPath := writeCLIConfig(t, server.URL)
	err := executeCLI(t, []string{"--config", configPath, "remove", "not-a-uuid"})

	if err == nil {
		t.Fatal("Execute() error = nil, want invalid entry ID validation error")
	}
	if !strings.Contains(err.Error(), "invalid entry ID") {
		t.Fatalf("error = %q, want invalid entry ID validation", err)
	}
	if apiCalls != 0 {
		t.Fatalf("API calls = %d, want 0", apiCalls)
	}
}
