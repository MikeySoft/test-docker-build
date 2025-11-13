package auth

import "testing"

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("secret")
	if err != nil {
		t.Fatalf("HashPassword error: %v", err)
	}
	ok, err := VerifyPassword("secret", hash)
	if err != nil || !ok {
		t.Fatalf("VerifyPassword failed, ok=%v err=%v", ok, err)
	}
}

func TestVerifyPasswordInvalidFormat(t *testing.T) {
	if ok, err := VerifyPassword("secret", "invalid"); err == nil || ok {
		t.Fatal("expected invalid format to fail validation")
	}
}
