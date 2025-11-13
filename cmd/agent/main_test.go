package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestLoadOrGenerateAgentIDUsesEnv(t *testing.T) {
	t.Setenv("AGENT_ID", "env-id")
	if id := loadOrGenerateAgentID(os.Getenv("AGENT_ID")); id != "env-id" {
		t.Fatalf("expected env id, got %s", id)
	}
}

func TestSaveAndLoadAgentIDFromHome(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	const expected = "test-id"
	if err := saveAgentIDToFile(expected); err != nil {
		t.Fatalf("saveAgentIDToFile error: %v", err)
	}
	got := loadAgentIDFromFile()
	if got != expected {
		t.Fatalf("loadAgentIDFromFile = %s, want %s", got, expected)
	}

	homePath := filepath.Join(tmp, agentIDFileHome)
	if _, err := os.Stat(homePath); err != nil {
		t.Fatalf("expected agent id file at %s: %v", homePath, err)
	}
}

func TestSetupLoggingAppliesLevel(t *testing.T) {
	setupLogging("debug", "text")
	if logrus.GetLevel() != logrus.DebugLevel {
		t.Fatalf("expected debug level, got %s", logrus.GetLevel())
	}
}
