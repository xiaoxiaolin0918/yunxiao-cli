package oauth

import (
	"errors"
	"strings"
	"testing"
)

func TestNewTokenErrorInvalidGrant(t *testing.T) {
	err := newTokenError(400, []byte(`{"error":"invalid_grant","error_description":"refresh token revoked"}`))
	if err.Code != "invalid_grant" {
		t.Fatalf("Code=%q", err.Code)
	}
	if err.Status != 400 {
		t.Fatalf("Status=%d", err.Status)
	}
	var te *TokenError
	if !errors.As(err, &te) || te.Code != "invalid_grant" {
		t.Fatalf("errors.As failed: %#v", err)
	}
	got := err.Error()
	if !strings.Contains(got, "invalid_grant") || !strings.Contains(got, "400") {
		t.Fatalf("Error()=%q", got)
	}
}
