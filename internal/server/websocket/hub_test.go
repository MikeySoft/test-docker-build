package websocket

import (
	"encoding/json"
	"testing"
)

func TestExtractContainersCount(t *testing.T) {
	cases := []struct {
		name string
		data interface{}
		want int
	}{
		{"nil", nil, 0},
		{"not a map", 123, 0},
		{"map without containers", map[string]interface{}{"foo": 1}, 0},
		{"containers not a slice", map[string]interface{}{"containers": 1}, 0},
		{"containers slice", map[string]interface{}{"containers": []interface{}{1, 2, 3}}, 3},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := extractContainersCount(tc.data); got != tc.want {
				t.Fatalf("extractContainersCount() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestForwardLogEvent(t *testing.T) {
	hub := NewHub()

	// Create a fake log stream connection with a buffered channel
	recv := make(chan []byte, 1)
	ls := &LogStreamConnection{
		ID:          "ls-1",
		Send:        recv,
		ContainerID: "cont-123",
		HostID:      "host-abc",
		Hub:         hub,
	}

	// Register directly into map under lock
	hub.mu.Lock()
	hub.logStreams[ls.ID] = ls
	hub.mu.Unlock()

	// Forward a log event
	hub.ForwardLogEvent("host-abc", "cont-123", "hello", "stdout", "2025-10-29T10:00:00Z")

	// Ensure a message was sent
	select {
	case payload := <-recv:
		var msg map[string]interface{}
		if err := json.Unmarshal(payload, &msg); err != nil {
			t.Fatalf("invalid JSON sent to log stream: %v", err)
		}
		if msg["type"] != "log_data" {
			t.Fatalf("unexpected type: %v", msg["type"])
		}
		pl, ok := msg["payload"].(map[string]interface{})
		if !ok {
			t.Fatalf("payload not a map: %T", msg["payload"])
		}
		if pl["data"] != "hello" || pl["stream"] != "stdout" {
			t.Fatalf("unexpected payload: %+v", pl)
		}
	default:
		t.Fatal("no message received on log stream channel")
	}
}
