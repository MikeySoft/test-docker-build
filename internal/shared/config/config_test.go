package config

import (
	"testing"
	"time"
)

func TestAgentConfigGetServerURL(t *testing.T) {
	cfg := &AgentConfig{
		ServerAddress: "example.com",
		ServerPort:    9443,
		ServerUseTLS:  true,
	}

	got := cfg.GetServerURL()
	want := "wss://example.com:9443/ws/agent"
	if got != want {
		t.Fatalf("GetServerURL() = %s, want %s", got, want)
	}
}

func TestLoadAgentConfigAutoHostStats(t *testing.T) {
	t.Setenv("METRICS_COLLECT_HOST_STATS", "auto")
	t.Setenv("SERVER_ADDRESS", "host")
	t.Setenv("SERVER_PORT", "1234")
	t.Setenv("SERVER_USE_TLS", "true")

	cfg := LoadAgentConfig()

	if !cfg.MetricsCollectHostStatsAuto {
		t.Fatal("expected MetricsCollectHostStatsAuto to be true when env=auto")
	}
	if cfg.ServerAddress != "host" {
		t.Fatalf("ServerAddress = %s, want host", cfg.ServerAddress)
	}
	if cfg.ServerPort != 1234 {
		t.Fatalf("ServerPort = %d, want 1234", cfg.ServerPort)
	}
	if !cfg.ServerUseTLS {
		t.Fatal("expected ServerUseTLS to be true")
	}
}

func TestLoadServerConfigOverrides(t *testing.T) {
	t.Setenv("SERVER_HOST", "0.0.0.0")
	t.Setenv("SERVER_PORT", "9000")
	t.Setenv("MODE", "DEV")
	t.Setenv("WS_HANDSHAKE_TIMEOUT", "5s")
	t.Setenv("INFLUXDB_ENABLED", "true")

	cfg := LoadServerConfig()

	if cfg.Host != "0.0.0.0" {
		t.Fatalf("Host = %s, want 0.0.0.0", cfg.Host)
	}
	if cfg.Port != 9000 {
		t.Fatalf("Port = %d, want 9000", cfg.Port)
	}
	if cfg.Mode != "DEV" {
		t.Fatalf("Mode = %s, want DEV", cfg.Mode)
	}
	if cfg.WSHandshakeTimeout != 5*time.Second {
		t.Fatalf("WSHandshakeTimeout = %v, want 5s", cfg.WSHandshakeTimeout)
	}
	if !cfg.InfluxDBEnabled {
		t.Fatal("expected InfluxDBEnabled to be true")
	}
}

func TestEnvHelpersFallback(t *testing.T) {
	t.Setenv("TEST_INT", "not-a-number")
	t.Setenv("TEST_BOOL", "not-bool")
	t.Setenv("TEST_DURATION", "not-duration")

	if got := getEnv("MISSING_VALUE", "fallback"); got != "fallback" {
		t.Fatalf("getEnv fallback = %s, want fallback", got)
	}
	if got := getEnvAsInt("TEST_INT", 42); got != 42 {
		t.Fatalf("getEnvAsInt fallback = %d, want 42", got)
	}
	if got := getEnvAsBool("TEST_BOOL", true); !got {
		t.Fatal("getEnvAsBool fallback should return default true")
	}
	if got := getEnvAsDuration("TEST_DURATION", time.Minute); got != time.Minute {
		t.Fatalf("getEnvAsDuration fallback = %v, want 1m", got)
	}

	hostname := getHostname()
	if hostname == "" {
		t.Fatal("expected hostname to be non-empty")
	}
}
