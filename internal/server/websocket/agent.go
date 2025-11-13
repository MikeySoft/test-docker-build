package websocket

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mikeysoft/flotilla/internal/shared/protocol"
	"github.com/sirupsen/logrus"
)

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer
	maxMessageSize = 1024 * 1024 // 1MB
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
}

// readPump pumps messages from the websocket connection to the hub
func (c *AgentConnection) readPump() {
	// Add panic recovery
	defer func() {
		if r := recover(); r != nil {
			logrus.Errorf("Panic in agent readPump for agent %s: %v", c.ID, r)
		}
		c.Hub.unregisterAgent <- c
		if err := c.Conn.Close(); err != nil && !errors.Is(err, websocket.ErrCloseSent) {
			logrus.WithError(err).Debugf("Failed to close agent connection %s", c.ID)
		}
	}()

	c.Conn.SetReadLimit(maxMessageSize)
	if err := c.Conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		logrus.WithError(err).Warnf("Failed to set read deadline for agent %s", c.ID)
	}
	c.Conn.SetPongHandler(func(string) error {
		if err := c.Conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
			logrus.WithError(err).Warnf("Failed to extend read deadline for agent %s", c.ID)
		}
		c.LastSeen = time.Now()
		return nil
	})

	for {
		_, messageData, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logrus.Errorf("WebSocket error: %v", err)
			}
			break
		}

		// Parse the message
		msg, err := protocol.DeserializeMessage(messageData)
		if err != nil {
			logrus.Errorf("Failed to parse message from agent %s: %v", c.ID, err)
			continue
		}

		// Handle different message types
		switch msg.Type {
		case protocol.MessageTypeResponse:
			c.handleResponse(msg)
		case protocol.MessageTypeEvent:
			c.handleEvent(msg)
		case protocol.MessageTypeHeartbeat:
			c.handleHeartbeat(msg)
		case protocol.MessageTypeMetrics:
			c.handleMetrics(msg)
		default:
			logrus.Warnf("Unknown message type from agent %s: %s", c.ID, msg.Type)
		}
	}
}

// writePump pumps messages from the hub to the websocket connection
func (c *AgentConnection) writePump() {
	// Add panic recovery
	defer func() {
		if r := recover(); r != nil {
			logrus.Errorf("Panic in agent writePump for agent %s: %v", c.ID, r)
		}
	}()

	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		if err := c.Conn.Close(); err != nil && !errors.Is(err, websocket.ErrCloseSent) {
			logrus.WithError(err).Debugf("Failed to close agent connection %s", c.ID)
		}
	}()

	for {
		select {
		case message, ok := <-c.Send:
			if err := c.Conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				logrus.WithError(err).Warnf("Failed to set write deadline for agent %s", c.ID)
				return
			}
			if !ok {
				if err := c.Conn.WriteMessage(websocket.CloseMessage, []byte{}); err != nil && !errors.Is(err, websocket.ErrCloseSent) {
					logrus.WithError(err).Debugf("Failed to send close message to agent %s", c.ID)
				}
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			if _, err := w.Write(message); err != nil {
				logrus.WithError(err).Debugf("Failed to write message to agent %s", c.ID)
				_ = w.Close()
				return
			}

			// Add queued messages to the current websocket message
			n := len(c.Send)
		drain:
			for i := 0; i < n; i++ {
				select {
				case queuedMessage := <-c.Send:
					if _, err := w.Write([]byte{'\n'}); err != nil {
						logrus.WithError(err).Debugf("Failed to write queued separator for agent %s", c.ID)
						break drain
					}
					if _, err := w.Write(queuedMessage); err != nil {
						logrus.WithError(err).Debugf("Failed to write queued message for agent %s", c.ID)
						break drain
					}
				default:
					// No more messages available
					break drain
				}
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			if err := c.Conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				logrus.WithError(err).Warnf("Failed to set ping deadline for agent %s", c.ID)
				return
			}
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleResponse handles a response message from the agent
func (c *AgentConnection) handleResponse(msg *protocol.Message) {
	logrus.Infof("Received response from agent %s: ID=%s, Type=%s", c.ID, msg.ID, msg.Type)

	response, err := msg.GetResponse()
	if err != nil {
		logrus.Errorf("Failed to parse response from agent %s: %v", c.ID, err)
		return
	}

	// DEV: log full payload; PROD: summarize only
	if strings.EqualFold(c.Hub.Mode, "DEV") {
		logrus.Debugf("Parsed response (DEV): Status=%s, Data=%+v", response.Status, response.Data)
	} else {
		containersCount := extractContainersCount(response.Data)
		logrus.WithFields(logrus.Fields{
			"agent_id":         c.ID,
			"host_id":          c.HostID,
			"command_id":       msg.ID,
			"status":           response.Status,
			"containers_count": containersCount,
		}).Info("agent response")
	}

	cmdResp := &CommandResponse{
		CommandID: msg.ID,
		AgentID:   c.ID,
		Response:  msg,
		Error:     nil,
	}

	if waiter, ok := c.Hub.getResponseWaiter(msg.ID); ok {
		select {
		case waiter <- cmdResp:
			logrus.Infof("Delivered response to waiter for command %s", msg.ID)
		default:
			logrus.Warnf("Response waiter channel full for command %s, dropping response", msg.ID)
		}
		return
	}

	// Fallback to the hub's shared response channel
	select {
	case c.Hub.responses <- cmdResp:
		logrus.Infof("Successfully queued response for command %s", msg.ID)
	default:
		logrus.Warn("Response channel full, dropping response")
	}

	logrus.Infof("Received response from agent %s for command %s: %s", c.ID, msg.ID, response.Status)
}

// extractContainersCount attempts to derive containers_count from a generic response payload
func extractContainersCount(data interface{}) int {
	// Expecting data to be a map with key "containers" -> slice
	m, ok := data.(map[string]interface{})
	if !ok {
		return 0
	}
	if v, exists := m["containers"]; exists {
		if arr, ok := v.([]interface{}); ok {
			return len(arr)
		}
	}
	return 0
}

// handleEvent handles an event message from the agent
func (c *AgentConnection) handleEvent(msg *protocol.Message) {
	event, err := msg.GetEvent()
	if err != nil {
		logrus.Errorf("Failed to parse event from agent %s: %v", c.ID, err)
		return
	}

	logrus.Debugf("Received event from agent %s: %s", c.ID, event.EventType)

	// Handle log_data events specifically
	if event.EventType == "log_data" {
		c.handleLogDataEvent(event)
		return
	}

	// Broadcast other events to UI clients
	c.broadcastEventToUI(msg)
}

// handleLogDataEvent handles log data events from agents and forwards them to UI clients
func (c *AgentConnection) handleLogDataEvent(event *protocol.Event) {
	// Extract log data from event data
	containerID, _ := event.Data["container_id"].(string)
	data, _ := event.Data["data"].(string)
	stream, _ := event.Data["stream"].(string)
	timestamp, _ := event.Data["timestamp"].(string)

	if containerID == "" || data == "" {
		logrus.Errorf("Missing required log data fields from agent %s", c.ID)
		return
	}

	// Forward log event to UI clients
	c.Hub.ForwardLogEvent(c.HostID, containerID, data, stream, timestamp)
}

// handleHeartbeat handles a heartbeat message from the agent
func (c *AgentConnection) handleHeartbeat(msg *protocol.Message) {
	heartbeat, err := msg.GetHeartbeat()
	if err != nil {
		logrus.Errorf("Failed to parse heartbeat from agent %s: %v", c.ID, err)
		return
	}

	c.LastSeen = time.Now()

	logrus.Debugf("Received heartbeat from agent %s: status=%s, uptime=%ds, containers=%d",
		c.ID, heartbeat.Status, heartbeat.Uptime, heartbeat.ContainersRunning)

	// Update host status based on heartbeat
	status := "online"
	if heartbeat.Status != "healthy" {
		status = "error"
	}

	// Create or update host with metadata from heartbeat
	c.Hub.createOrUpdateHostWithMetadata(c.HostID, c.ID, heartbeat.AgentName, heartbeat.Hostname, status)
}

// handleMetrics handles a metrics message from the agent
// SonarQube Won't Fix: This handler coordinates parsing, logging, and conditional writes
// to metrics storage. Branching reflects feature flags and payload presence. Further
// splitting would complicate flow without real complexity reduction. // NOSONAR
func (c *AgentConnection) handleMetrics(msg *protocol.Message) { // NOSONAR
	metricsPayload, err := msg.GetMetrics()
	if err != nil {
		logrus.Errorf("Failed to parse metrics from agent %s: %v", c.ID, err)
		return
	}

	logrus.Infof("Received metrics from agent %s: %d container metrics", c.ID, len(metricsPayload.ContainerMetrics))
	logrus.Debugf("Metrics payload HostID: %s, Agent connection HostID: %s", metricsPayload.HostID, c.HostID)
	if metricsPayload.HostMetrics != nil {
		logrus.Infof("Received host metrics from agent %s: CPU=%.2f%%, Memory=%d/%d", c.ID, metricsPayload.HostMetrics.CPUPercent, metricsPayload.HostMetrics.MemoryUsage, metricsPayload.HostMetrics.MemoryTotal)
	}

	// Write metrics to InfluxDB if available, else drop fast
	if c.Hub.metricsClient != nil && c.Hub.metricsClient.IsEnabled() {
		// Write container metrics
		if len(metricsPayload.ContainerMetrics) > 0 {
			// Always use the server-side HostID associated with this agent connection
			if err := c.Hub.metricsClient.WriteContainerMetrics(c.HostID, metricsPayload.ContainerMetrics, metricsPayload.Timestamp); err != nil {
				logrus.Errorf("Failed to write container metrics to InfluxDB: %v", err)
			} else {
				logrus.Infof("Successfully wrote %d container metrics to InfluxDB", len(metricsPayload.ContainerMetrics))
			}
		}

		// Write host metrics if present
		if metricsPayload.HostMetrics != nil {
			// Always tag with the server-side HostID so API queries match
			if err := c.Hub.metricsClient.WriteHostMetrics(c.HostID, metricsPayload.HostMetrics, metricsPayload.Timestamp); err != nil {
				logrus.Errorf("Failed to write host metrics to InfluxDB: %v", err)
			} else {
				logrus.Infof("Successfully wrote host metrics to InfluxDB")
			}
		}
	} else {
		// Drop quickly when storage disabled; log once at debug level
		if !c.Hub.metricsDropLogged {
			c.Hub.metricsDropLogged = true
			logrus.Debug("Metrics storage disabled; dropping incoming metrics messages")
		}
	}
}

// broadcastEventToUI broadcasts an event to all connected UI clients
func (c *AgentConnection) broadcastEventToUI(msg *protocol.Message) {
	c.Hub.mu.RLock()
	defer c.Hub.mu.RUnlock()

	// Create a UI event message
	uiEvent := map[string]interface{}{
		"type":      "event",
		"host_id":   c.HostID,
		"agent_id":  c.ID,
		"timestamp": msg.Timestamp,
		"payload":   msg.Payload,
	}

	eventData, err := json.Marshal(uiEvent)
	if err != nil {
		logrus.Errorf("Failed to marshal UI event: %v", err)
		return
	}

	// Send to all UI clients
	for _, uiClient := range c.Hub.uiClients {
		select {
		case uiClient.Send <- eventData:
		default:
			// UI client is not ready, skip
		}
	}
}

// startPumps starts the read and write pumps with duplicate prevention
func (c *AgentConnection) startPumps() {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Prevent duplicate pump creation
	if c.PumpsStarted {
		logrus.Warnf("Pumps already started for agent %s, skipping duplicate start", c.ID)
		return
	}

	c.PumpsStarted = true
	logrus.Debugf("Starting pumps for agent %s", c.ID)

	// Start goroutines for reading and writing
	go c.writePump()
	go c.readPump()
}
