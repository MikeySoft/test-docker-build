package websocket

import (
	"testing"
	"time"

	agentconfig "github.com/mikeysoft/flotilla/internal/agent/config"
	sharedconfig "github.com/mikeysoft/flotilla/internal/shared/config"
	"github.com/mikeysoft/flotilla/internal/shared/protocol"
)

func newTestConfig() *agentconfig.Config {
	return &agentconfig.Config{
		AgentConfig: sharedconfig.AgentConfig{
			ServerAddress:     "localhost",
			ServerPort:        8080,
			APIKey:            "api",
			AgentID:           "agent",
			AgentName:         "agent-name",
			HeartbeatInterval: 10 * time.Millisecond,
		},
	}
}

func TestSendCommandNotConnected(t *testing.T) {
	client := NewClient(newTestConfig())
	err := client.SendCommand(protocol.NewCommand("id", "action", nil))
	if err == nil {
		t.Fatal("expected error when sending command without connection")
	}
}

func TestHandleCommandResponse(t *testing.T) {
	client := NewClient(newTestConfig())
	msg := protocol.NewCommand("id", "noop", nil)

	client.handleCommand(msg)

	select {
	case resp := <-client.GetResponses():
		if resp.ID != "id" || resp.Type != protocol.MessageTypeResponse {
			t.Fatalf("unexpected response: %#v", resp)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for response")
	}
}

func TestHandleEvent(t *testing.T) {
	client := NewClient(newTestConfig())
	event := protocol.NewMessage(protocol.MessageTypeEvent, "evt", map[string]any{})
	client.handleEvent(event)

	select {
	case got := <-client.GetEvents():
		if got.ID != "evt" {
			t.Fatalf("expected event id evt, got %s", got.ID)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestReconnectDeprecated(t *testing.T) {
	client := NewClient(newTestConfig())
	if err := client.Reconnect(nil); err == nil {
		t.Fatal("expected reconnect to return error")
	}
}

func TestGetHostname(t *testing.T) {
	if host := getHostname(); host == "" {
		t.Fatal("expected hostname to be non-empty")
	}
}
