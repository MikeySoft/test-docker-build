package protocol

import (
	"testing"
)

const (
	testID             = "test-123"
	errDeserializeFmt  = "Failed to deserialize message: %v"
	errExpectedTypeFmt = "Expected message type %s, got %s"
)

func TestMessageSerialization(t *testing.T) {
	// Test command message
	command := NewCommand(testID, "list_containers", map[string]any{
		"all": true,
	})

	data, err := command.Serialize()
	if err != nil {
		t.Fatalf("Failed to serialize command: %v", err)
	}

	// Test deserialization
	msg, err := DeserializeMessage(data)
	if err != nil {
		t.Fatalf(errDeserializeFmt, err)
	}

	if msg.Type != MessageTypeCommand {
		t.Errorf(errExpectedTypeFmt, MessageTypeCommand, msg.Type)
	}

	if msg.ID != testID {
		t.Errorf("Expected message ID test-123, got %s", msg.ID)
	}

	// Test command extraction
	cmd, err := msg.GetCommand()
	if err != nil {
		t.Fatalf("Failed to get command: %v", err)
	}

	if cmd.Action != "list_containers" {
		t.Errorf("Expected action list_containers, got %s", cmd.Action)
	}

	if all, ok := cmd.Params["all"].(bool); !ok || !all {
		t.Errorf("Expected all=true, got %v", cmd.Params["all"])
	}
}

func TestResponseMessage(t *testing.T) {
	// Test success response
	response := NewResponse(testID, "success", map[string]any{
		"containers": []map[string]any{
			{"id": "container1", "name": "test-container"},
		},
	}, nil)

	data, err := response.Serialize()
	if err != nil {
		t.Fatalf("Failed to serialize response: %v", err)
	}

	// Test deserialization
	msg, err := DeserializeMessage(data)
	if err != nil {
		t.Fatalf(errDeserializeFmt, err)
	}

	if msg.Type != MessageTypeResponse {
		t.Errorf(errExpectedTypeFmt, MessageTypeResponse, msg.Type)
	}

	// Test response extraction
	resp, err := msg.GetResponse()
	if err != nil {
		t.Fatalf("Failed to get response: %v", err)
	}

	if resp.Status != "success" {
		t.Errorf("Expected status success, got %s", resp.Status)
	}
}

func TestEventMessage(t *testing.T) {
	// Test event message
	event := NewEvent("container_started", map[string]any{
		"container_id":   "abc123",
		"container_name": "test-container",
	})

	data, err := event.Serialize()
	if err != nil {
		t.Fatalf("Failed to serialize event: %v", err)
	}

	// Test deserialization
	msg, err := DeserializeMessage(data)
	if err != nil {
		t.Fatalf(errDeserializeFmt, err)
	}

	if msg.Type != MessageTypeEvent {
		t.Errorf(errExpectedTypeFmt, MessageTypeEvent, msg.Type)
	}

	// Test event extraction
	evt, err := msg.GetEvent()
	if err != nil {
		t.Fatalf("Failed to get event: %v", err)
	}

	if evt.EventType != "container_started" {
		t.Errorf("Expected event type container_started, got %s", evt.EventType)
	}
}

func TestHeartbeatMessage(t *testing.T) {
	// Test heartbeat message
	heartbeat := NewHeartbeat("agent-123", "agent-name", "host-1", "healthy", 3600, 5)

	data, err := heartbeat.Serialize()
	if err != nil {
		t.Fatalf("Failed to serialize heartbeat: %v", err)
	}

	// Test deserialization
	msg, err := DeserializeMessage(data)
	if err != nil {
		t.Fatalf("Failed to deserialize message: %v", err)
	}

	if msg.Type != MessageTypeHeartbeat {
		t.Errorf("Expected message type %s, got %s", MessageTypeHeartbeat, msg.Type)
	}

	// Test heartbeat extraction
	hb, err := msg.GetHeartbeat()
	if err != nil {
		t.Fatalf("Failed to get heartbeat: %v", err)
	}

	if hb.AgentID != "agent-123" {
		t.Errorf("Expected agent ID agent-123, got %s", hb.AgentID)
	}

	if hb.Status != "healthy" {
		t.Errorf("Expected status healthy, got %s", hb.Status)
	}

	if hb.Uptime != 3600 {
		t.Errorf("Expected uptime 3600, got %d", hb.Uptime)
	}

	if hb.ContainersRunning != 5 {
		t.Errorf("Expected containers running 5, got %d", hb.ContainersRunning)
	}
}
