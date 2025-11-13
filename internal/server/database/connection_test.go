package database

import "testing"

func TestMigrateRequiresConnection(t *testing.T) {
	DB = nil
	if err := Migrate(); err == nil {
		t.Fatal("expected migrate to fail without initialized DB")
	}
}

func TestCloseNilDB(t *testing.T) {
	DB = nil
	if err := Close(); err != nil {
		t.Fatalf("Close() with nil DB should return nil, got %v", err)
	}
}
