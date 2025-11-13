package auth

import "testing"

func TestAPIKeyOperationsRequireDatabase(t *testing.T) {
	if _, err := GenerateAPIKey("name", nil); err == nil {
		t.Fatal("expected GenerateAPIKey to fail without database")
	}
	if _, err := ValidateAPIKey("FLA_prefix_secret"); err == nil {
		t.Fatal("expected ValidateAPIKey to fail without database")
	}
	if err := RevokeAPIKey("FLA_prefix_secret"); err == nil {
		t.Fatal("expected RevokeAPIKey to fail without database")
	}
	if _, err := ListAPIKeys(); err == nil {
		t.Fatal("expected ListAPIKeys to fail without database")
	}
}
