package config

import (
	"testing"
	"time"
)

func TestGetServerAddress(t *testing.T) {
	cfg := &Config{}
	cfg.Host = "127.0.0.1"
	cfg.Port = 9090

	if got, want := cfg.GetServerAddress(), "127.0.0.1:9090"; got != want {
		t.Fatalf("GetServerAddress() = %s, want %s", got, want)
	}
}

func TestWebSocketAccessors(t *testing.T) {
	cfg := &Config{}
	cfg.WSReadBufferSize = 2048
	cfg.WSWriteBufferSize = 4096
	cfg.WSHandshakeTimeout = 3 * time.Second

	if cfg.GetWebSocketReadBufferSize() != 2048 {
		t.Fatalf("unexpected read buffer size")
	}
	if cfg.GetWebSocketWriteBufferSize() != 4096 {
		t.Fatalf("unexpected write buffer size")
	}
	if cfg.GetWebSocketHandshakeTimeout() != 3*time.Second {
		t.Fatalf("unexpected handshake timeout")
	}
}
