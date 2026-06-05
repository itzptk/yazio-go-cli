package yazio

import (
	"testing"
	"time"
)

func TestTokenExpiredTreatsMissingExpiryAsExpired(t *testing.T) {
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	token := Token{
		AccessToken:  "access",
		RefreshToken: "refresh",
		TokenType:    "Bearer",
	}

	if !token.Expired(now) {
		t.Fatal("Expired() = false for token without ExpiresAt, want true")
	}
}
