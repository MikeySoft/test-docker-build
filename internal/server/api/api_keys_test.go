package api

import (
	"testing"
)

func TestGenerateAPIKeyPrefix(t *testing.T) {
	prefix := generateAPIKeyPrefix()
	if len(prefix) != 8 {
		t.Fatalf("expected prefix length 8, got %d", len(prefix))
	}
	for _, r := range prefix {
		if r < '0' || (r > '9' && r < 'A') || r > 'F' {
			t.Fatalf("prefix contains invalid character: %q", r)
		}
	}
}

func TestGenerateAPIKeySecret(t *testing.T) {
	secret := generateAPIKeySecret()
	if len(secret) != 32 {
		t.Fatalf("expected secret length 32, got %d", len(secret))
	}
}

func TestNewAPIKeysHandler(t *testing.T) {
	if NewAPIKeysHandler() == nil {
		t.Fatal("expected handler instance")
	}
}
