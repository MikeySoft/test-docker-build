package protocol

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// MessageType represents the type of WebSocket message
type MessageType string

const (
	MessageTypeCommand   MessageType = "command"
	MessageTypeResponse  MessageType = "response"
	MessageTypeEvent     MessageType = "event"
	MessageTypeHeartbeat MessageType = "heartbeat"
	MessageTypeMetrics   MessageType = "metrics"
)

// Message represents a WebSocket message between server and agent
type Message struct {
	Type      MessageType    `json:"type"`
	ID        string         `json:"id"`
	Timestamp time.Time      `json:"timestamp"`
	Payload   map[string]any `json:"payload"`
}

// Command represents a command sent from server to agent
type Command struct {
	Action string         `json:"action"`
	Params map[string]any `json:"params"`
}

// Response represents a response sent from agent to server
type Response struct {
	Status string      `json:"status"` // success, error
	Data   interface{} `json:"data,omitempty"`
	Error  string      `json:"error,omitempty"`
}

// Event represents an event sent from agent to server
type Event struct {
	EventType string         `json:"event_type"`
	Data      map[string]any `json:"data"`
}

// Heartbeat represents a heartbeat message
type Heartbeat struct {
	AgentID           string `json:"agent_id"`
	AgentName         string `json:"agent_name"`
	Hostname          string `json:"hostname"`
	Status            string `json:"status"` // healthy, unhealthy
	Uptime            int64  `json:"uptime"` // seconds
	ContainersRunning int    `json:"containers_running"`
}

// MetricsPayload represents metrics data sent from agent to server
type MetricsPayload struct {
	Timestamp        time.Time         `json:"timestamp"`
	HostID           string            `json:"host_id"`
	ContainerMetrics []ContainerMetric `json:"container_metrics,omitempty"`
	HostMetrics      *HostMetric       `json:"host_metrics,omitempty"`
}

// ContainerMetric represents container-level metrics
type ContainerMetric struct {
	Timestamp      time.Time `json:"timestamp,omitempty"`
	ContainerID    string    `json:"container_id"`
	ContainerName  string    `json:"container_name"`
	StackName      string    `json:"stack_name,omitempty"`
	CPUPercent     float64   `json:"cpu_percent"`
	MemoryUsage    uint64    `json:"memory_usage"`
	MemoryLimit    uint64    `json:"memory_limit"`
	DiskReadBytes  uint64    `json:"disk_read_bytes"`
	DiskWriteBytes uint64    `json:"disk_write_bytes"`
	NetworkRxBytes uint64    `json:"network_rx_bytes,omitempty"`
	NetworkTxBytes uint64    `json:"network_tx_bytes,omitempty"`
}

// HostMetric represents host-level system metrics
type HostMetric struct {
	Timestamp   time.Time `json:"timestamp,omitempty"`
	CPUPercent  float64   `json:"cpu_percent"`
	MemoryUsage uint64    `json:"memory_usage"`
	MemoryTotal uint64    `json:"memory_total"`
	DiskUsage   uint64    `json:"disk_usage"`
	DiskTotal   uint64    `json:"disk_total"`
}

// NewMessage creates a new message with the given type and payload
func NewMessage(msgType MessageType, id string, payload map[string]any) *Message {
	return &Message{
		Type:      msgType,
		ID:        id,
		Timestamp: time.Now(),
		Payload:   payload,
	}
}

// NewCommand creates a new command message
func NewCommand(id, action string, params map[string]any) *Message {
	return NewMessage(MessageTypeCommand, id, map[string]any{
		"action": action,
		"params": params,
	})
}

// NewCommandWithAction creates a new command with a generated unique ID.
func NewCommandWithAction(action string, params map[string]any) *Message {
	return NewCommand(uuid.NewString(), action, params)
}

// NewResponse creates a new response message
func NewResponse(id string, status string, data interface{}, err error) *Message {
	payload := map[string]any{
		"status": status,
	}

	if data != nil {
		payload["data"] = data
	}

	if err != nil {
		payload["error"] = err.Error()
	}

	return NewMessage(MessageTypeResponse, id, payload)
}

// NewEvent creates a new event message
func NewEvent(eventType string, data map[string]any) *Message {
	return NewMessage(MessageTypeEvent, "", map[string]any{
		"event_type": eventType,
		"data":       data,
	})
}

// NewHeartbeat creates a new heartbeat message
func NewHeartbeat(agentID, agentName, hostname, status string, uptime int64, containersRunning int) *Message {
	return NewMessage(MessageTypeHeartbeat, "", map[string]any{
		"agent_id":           agentID,
		"agent_name":         agentName,
		"hostname":           hostname,
		"status":             status,
		"uptime":             uptime,
		"containers_running": containersRunning,
	})
}

// NewMetrics creates a new metrics message
func NewMetrics(hostID string, payload *MetricsPayload) *Message {
	return NewMessage(MessageTypeMetrics, "", map[string]any{
		"timestamp":         payload.Timestamp,
		"host_id":           hostID,
		"container_metrics": payload.ContainerMetrics,
		"host_metrics":      payload.HostMetrics,
	})
}

// Serialize converts the message to JSON bytes
func (m *Message) Serialize() ([]byte, error) {
	return json.Marshal(m)
}

// DeserializeMessage parses JSON bytes into a Message
func DeserializeMessage(data []byte) (*Message, error) {
	var msg Message
	err := json.Unmarshal(data, &msg)
	return &msg, err
}

// GetCommand extracts command data from message payload
func (m *Message) GetCommand() (*Command, error) {
	if m.Type != MessageTypeCommand {
		return nil, ErrInvalidMessageType
	}

	action, ok := m.Payload["action"].(string)
	if !ok {
		return nil, ErrInvalidPayload
	}

	params, ok := m.Payload["params"].(map[string]any)
	if !ok {
		params = make(map[string]any)
	}

	return &Command{
		Action: action,
		Params: params,
	}, nil
}

// GetResponse extracts response data from message payload
func (m *Message) GetResponse() (*Response, error) {
	if m.Type != MessageTypeResponse {
		return nil, ErrInvalidMessageType
	}

	status, ok := m.Payload["status"].(string)
	if !ok {
		return nil, ErrInvalidPayload
	}

	response := &Response{
		Status: status,
		Data:   m.Payload["data"],
	}

	if err, ok := m.Payload["error"].(string); ok {
		response.Error = err
	}

	return response, nil
}

// GetEvent extracts event data from message payload
func (m *Message) GetEvent() (*Event, error) {
	if m.Type != MessageTypeEvent {
		return nil, ErrInvalidMessageType
	}

	eventType, ok := m.Payload["event_type"].(string)
	if !ok {
		return nil, ErrInvalidPayload
	}

	data, ok := m.Payload["data"].(map[string]any)
	if !ok {
		data = make(map[string]any)
	}

	return &Event{
		EventType: eventType,
		Data:      data,
	}, nil
}

// GetHeartbeat extracts heartbeat data from message payload
func (m *Message) GetHeartbeat() (*Heartbeat, error) {
	if m.Type != MessageTypeHeartbeat {
		return nil, ErrInvalidMessageType
	}

	agentID, _ := m.Payload["agent_id"].(string)
	agentName, _ := m.Payload["agent_name"].(string)
	hostname, _ := m.Payload["hostname"].(string)
	status, _ := m.Payload["status"].(string)
	uptime, _ := m.Payload["uptime"].(float64)
	containersRunning, _ := m.Payload["containers_running"].(float64)

	return &Heartbeat{
		AgentID:           agentID,
		AgentName:         agentName,
		Hostname:          hostname,
		Status:            status,
		Uptime:            int64(uptime),
		ContainersRunning: int(containersRunning),
	}, nil
}

// GetMetrics extracts metrics data from message payload
// SonarQube Won't Fix: This function must coerce dynamically-typed JSON payloads into
// strongly-typed metrics structures and includes necessary guards for partial data.
// Splitting would fragment closely related mapping logic without meaningful complexity gains. // NOSONAR
func (m *Message) GetMetrics() (*MetricsPayload, error) { // NOSONAR
	if m.Type != MessageTypeMetrics {
		return nil, ErrInvalidMessageType
	}

	hostID, _ := m.Payload["host_id"].(string)

	var timestamp time.Time
	if ts, ok := m.Payload["timestamp"].(string); ok {
		var err error
		timestamp, err = time.Parse(time.RFC3339, ts)
		if err != nil {
			timestamp = time.Now()
		}
	} else {
		timestamp = time.Now()
	}

	payload := &MetricsPayload{
		Timestamp:        timestamp,
		HostID:           hostID,
		ContainerMetrics: []ContainerMetric{},
	}

	// Extract container metrics
	if cm, ok := m.Payload["container_metrics"].([]interface{}); ok {
		for _, c := range cm {
			if cmap, ok := c.(map[string]interface{}); ok {
				cm := ContainerMetric{}
				if id, ok := cmap["container_id"].(string); ok {
					cm.ContainerID = id
				}
				if name, ok := cmap["container_name"].(string); ok {
					cm.ContainerName = name
				}
				if stack, ok := cmap["stack_name"].(string); ok {
					cm.StackName = stack
				}
				if cpu, ok := cmap["cpu_percent"].(float64); ok {
					cm.CPUPercent = cpu
				}
				if mem, ok := cmap["memory_usage"].(float64); ok {
					cm.MemoryUsage = uint64(mem)
				}
				if limit, ok := cmap["memory_limit"].(float64); ok {
					cm.MemoryLimit = uint64(limit)
				}
				if read, ok := cmap["disk_read_bytes"].(float64); ok {
					cm.DiskReadBytes = uint64(read)
				}
				if write, ok := cmap["disk_write_bytes"].(float64); ok {
					cm.DiskWriteBytes = uint64(write)
				}
				if rx, ok := cmap["network_rx_bytes"].(float64); ok {
					cm.NetworkRxBytes = uint64(rx)
				}
				if tx, ok := cmap["network_tx_bytes"].(float64); ok {
					cm.NetworkTxBytes = uint64(tx)
				}
				nouncm := ContainerMetric(cm)
				payload.ContainerMetrics = append(payload.ContainerMetrics, nouncm)
			}
		}
	}

	// Extract host metrics
	if hm, ok := m.Payload["host_metrics"].(map[string]interface{}); ok {
		hostMetric := &HostMetric{}
		if cpu, ok := hm["cpu_percent"].(float64); ok {
			hostMetric.CPUPercent = cpu
		}
		if mem, ok := hm["memory_usage"].(float64); ok {
			hostMetric.MemoryUsage = uint64(mem)
		}
		if total, ok := hm["memory_total"].(float64); ok {
			hostMetric.MemoryTotal = uint64(total)
		}
		if disk, ok := hm["disk_usage"].(float64); ok {
			hostMetric.DiskUsage = uint64(disk)
		}
		if diskTotal, ok := hm["disk_total"].(float64); ok {
			hostMetric.DiskTotal = uint64(diskTotal)
		}
		payload.HostMetrics = hostMetric
	}

	return payload, nil
}
