package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/mikeysoft/flotilla/internal/server/database"
	"github.com/mikeysoft/flotilla/internal/server/metrics"
	"github.com/mikeysoft/flotilla/internal/shared/protocol"
	"github.com/sirupsen/logrus"
)

const hostIDQuery = "id = ?"

// Hub manages WebSocket connections for agents and UI clients
type Hub struct {
	// Agent connections
	agents map[string]*AgentConnection

	// UI connections (for future use)
	uiClients map[string]*UIConnection

	// Log stream connections
	logStreams map[string]*LogStreamConnection

	// Command responses channel
	responses chan *CommandResponse

	// Response waiters keyed by command ID
	responseWaiters map[string]chan *CommandResponse

	// Metrics client for InfluxDB
	metricsClient *metrics.Client

	// Register/unregister channels
	registerAgent       chan *AgentConnection
	unregisterAgent     chan *AgentConnection
	registerUI          chan *UIConnection
	unregisterUI        chan *UIConnection
	registerLogStream   chan *LogStreamConnection
	unregisterLogStream chan *LogStreamConnection

	// Mutex for thread-safe access
	mu sync.RWMutex

	// Mode controls logging verbosity (DEV or PROD)
	Mode string
	// one-time log flag when metrics storage is disabled and metrics are received
	metricsDropLogged bool
}

// AgentConnection represents a WebSocket connection from an agent
type AgentConnection struct {
	ID           string
	HostID       string
	Conn         *websocket.Conn
	Send         chan []byte
	Hub          *Hub
	LastSeen     time.Time
	PumpsStarted bool         // Track if pumps have been started
	mu           sync.RWMutex // Protect pump state
}

// UIConnection represents a WebSocket connection from a UI client
type UIConnection struct {
	ID           string
	Conn         *websocket.Conn
	Send         chan []byte
	Hub          *Hub
	PumpsStarted bool         // Track if pumps have been started
	mu           sync.RWMutex // Protect pump state
}

// CommandResponse represents a response to a command
type CommandResponse struct {
	CommandID string
	AgentID   string
	Response  *protocol.Message
	Error     error
}

// NewHub creates a new WebSocket hub
func NewHub() *Hub {
	return &Hub{
		agents:              make(map[string]*AgentConnection),
		uiClients:           make(map[string]*UIConnection),
		logStreams:          make(map[string]*LogStreamConnection),
		responses:           make(chan *CommandResponse, 256),
		responseWaiters:     make(map[string]chan *CommandResponse),
		metricsClient:       nil, // Will be set later
		registerAgent:       make(chan *AgentConnection),
		unregisterAgent:     make(chan *AgentConnection),
		registerUI:          make(chan *UIConnection),
		unregisterUI:        make(chan *UIConnection),
		registerLogStream:   make(chan *LogStreamConnection),
		unregisterLogStream: make(chan *LogStreamConnection),
	}
}

// SetMetricsClient sets the metrics client for the hub
func (h *Hub) SetMetricsClient(client *metrics.Client) {
	h.metricsClient = client
}

// GetMetricsClient returns the metrics client from the hub
func (h *Hub) GetMetricsClient() *metrics.Client {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.metricsClient
}

// Run starts the hub's main loop
func (h *Hub) Run(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second) // Heartbeat check interval
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logrus.Info("WebSocket hub shutting down...")
			return

		case agent := <-h.registerAgent:
			h.registerAgentConnection(agent)

		case agent := <-h.unregisterAgent:
			h.unregisterAgentConnection(agent)

		case uiClient := <-h.registerUI:
			h.registerUIConnection(uiClient)

		case uiClient := <-h.unregisterUI:
			h.unregisterUIConnection(uiClient)

		case logStream := <-h.registerLogStream:
			h.registerLogStreamConnection(logStream)

		case logStream := <-h.unregisterLogStream:
			h.unregisterLogStreamConnection(logStream)

		case <-ticker.C:
			h.checkAgentHeartbeats()
		}
	}
}

// RegisterAgent registers a new agent connection
func (h *Hub) RegisterAgent(conn *websocket.Conn, agentID, hostID string) *AgentConnection {
	agent := &AgentConnection{
		ID:       agentID,
		HostID:   hostID,
		Conn:     conn,
		Send:     make(chan []byte, 256),
		Hub:      h,
		LastSeen: time.Now(),
	}

	h.registerAgent <- agent
	return agent
}

// RegisterUI registers a new UI client connection
func (h *Hub) RegisterUI(conn *websocket.Conn, clientID string) *UIConnection {
	uiClient := &UIConnection{
		ID:   clientID,
		Conn: conn,
		Send: make(chan []byte, 256),
		Hub:  h,
	}

	h.registerUI <- uiClient
	return uiClient
}

// SendCommand sends a command to a specific agent
func (h *Hub) SendCommand(agentID string, command *protocol.Message) error {
	h.mu.RLock()
	agent, exists := h.agents[agentID]
	h.mu.RUnlock()

	if !exists {
		return ErrAgentNotFound
	}

	data, err := command.Serialize()
	if err != nil {
		return err
	}

	// Send command via channel to avoid concurrent writes
	select {
	case agent.Send <- data:
		return nil
	case <-time.After(10 * time.Second):
		return fmt.Errorf("timeout sending command to agent %s", agentID)
	}
}

// SendCommandToHost sends a command to the agent managing a specific host
func (h *Hub) SendCommandToHost(hostID string, command *protocol.Message) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, agent := range h.agents {
		if agent.HostID == hostID {
			return h.SendCommand(agent.ID, command)
		}
	}

	return ErrHostNotFound
}

// GetResponses returns the responses channel
func (h *Hub) GetResponses() <-chan *CommandResponse {
	return h.responses
}

// SubscribeResponse registers a waiter channel for a specific command ID.
func (h *Hub) SubscribeResponse(commandID string) <-chan *CommandResponse {
	ch := make(chan *CommandResponse, 1)
	h.mu.Lock()
	h.responseWaiters[commandID] = ch
	h.mu.Unlock()
	return ch
}

// UnsubscribeResponse removes a waiter channel for a specific command ID.
func (h *Hub) UnsubscribeResponse(commandID string) {
	h.mu.Lock()
	delete(h.responseWaiters, commandID)
	h.mu.Unlock()
}

func (h *Hub) getResponseWaiter(commandID string) (chan *CommandResponse, bool) {
	h.mu.RLock()
	ch, ok := h.responseWaiters[commandID]
	h.mu.RUnlock()
	return ch, ok
}

// GetAgent returns an agent connection by ID
func (h *Hub) GetAgent(agentID string) (*AgentConnection, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	agent, exists := h.agents[agentID]
	return agent, exists
}

// GetAgents returns all agent connections
func (h *Hub) GetAgents() map[string]*AgentConnection {
	h.mu.RLock()
	defer h.mu.RUnlock()

	agents := make(map[string]*AgentConnection)
	for id, agent := range h.agents {
		agents[id] = agent
	}
	return agents
}

// GetAgentByHost returns an agent connection by host ID
func (h *Hub) GetAgentByHost(hostID string) (*AgentConnection, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, agent := range h.agents {
		if agent.HostID == hostID {
			return agent, true
		}
	}
	return nil, false
}

// registerAgentConnection registers a new agent connection
func (h *Hub) registerAgentConnection(agent *AgentConnection) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.agents[agent.ID] = agent

	// Create or update host in database
	h.createOrUpdateHost(agent.HostID, agent.ID)

	logrus.Infof("Agent %s connected for host %s", agent.ID, agent.HostID)

	// Start goroutines for reading and writing (with duplicate prevention)
	agent.startPumps()

	// Send initial server settings (handshake hint) to agent
	metricsEnabled := false
	if h.metricsClient != nil && h.metricsClient.IsEnabled() {
		metricsEnabled = true
	}
	settings := map[string]any{
		"server_settings": map[string]any{
			"metrics_enabled": metricsEnabled,
		},
	}
	msg := protocol.NewEvent("server_settings", settings)
	if data, err := msg.Serialize(); err == nil {
		select {
		case agent.Send <- data:
		default:
			logrus.Debugf("Agent %s settings channel full; skipping handshake hint", agent.ID)
		}
	}
}

// unregisterAgentConnection unregisters an agent connection
func (h *Hub) unregisterAgentConnection(agent *AgentConnection) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, exists := h.agents[agent.ID]; exists {
		delete(h.agents, agent.ID)
		close(agent.Send)

		// Update host status in database
		h.updateHostStatus(agent.HostID, "offline")

		logrus.Infof("Agent %s disconnected", agent.ID)
	}
}

// registerUIConnection registers a new UI client connection
func (h *Hub) registerUIConnection(uiClient *UIConnection) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.uiClients[uiClient.ID] = uiClient

	logrus.Infof("UI client %s connected", uiClient.ID)

	// Start goroutines for reading and writing (with duplicate prevention)
	uiClient.startPumps()
}

// unregisterUIConnection unregisters a UI client connection
func (h *Hub) unregisterUIConnection(uiClient *UIConnection) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, exists := h.uiClients[uiClient.ID]; exists {
		delete(h.uiClients, uiClient.ID)
		close(uiClient.Send)

		logrus.Infof("UI client %s disconnected", uiClient.ID)
	}
}

// registerLogStreamConnection registers a new log stream connection
func (h *Hub) registerLogStreamConnection(logStream *LogStreamConnection) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.logStreams[logStream.ID] = logStream
	logrus.Infof("Log stream %s connected for container %s on host %s",
		logStream.ID, logStream.ContainerID, logStream.HostID)
}

// unregisterLogStreamConnection unregisters a log stream connection
func (h *Hub) unregisterLogStreamConnection(logStream *LogStreamConnection) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, exists := h.logStreams[logStream.ID]; exists {
		delete(h.logStreams, logStream.ID)
		close(logStream.Send)
		logrus.Infof("Log stream %s disconnected", logStream.ID)
	}
}

// ForwardLogEvent forwards a log event from an agent to all relevant UI clients
func (h *Hub) ForwardLogEvent(hostID, containerID, data, stream, timestamp string) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Find all log stream connections for this host and container
	for _, logStream := range h.logStreams {
		if logStream.HostID == hostID && logStream.ContainerID == containerID {
			// Create log chunk message
			logMessage := map[string]interface{}{
				"type": "log_data",
				"payload": map[string]interface{}{
					"data":      data,
					"timestamp": timestamp,
					"stream":    stream,
				},
			}

			if data, err := json.Marshal(logMessage); err == nil {
				select {
				case logStream.Send <- data:
					// Message sent successfully
				default:
					logrus.Warnf("Failed to send log chunk to UI client %s: channel full", logStream.ID)
				}
			} else {
				logrus.Errorf("Failed to marshal log message: %v", err)
			}
		}
	}
}

// GetAgentByHostID finds an agent connection by host ID
func (h *Hub) GetAgentByHostID(hostID string) *AgentConnection {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, agent := range h.agents {
		if agent.HostID == hostID {
			return agent
		}
	}
	return nil
}

// checkAgentHeartbeats checks for stale agent connections
func (h *Hub) checkAgentHeartbeats() {
	h.mu.RLock()
	agents := make([]*AgentConnection, 0, len(h.agents))
	for _, agent := range h.agents {
		agents = append(agents, agent)
	}
	h.mu.RUnlock()

	now := time.Now()
	for _, agent := range agents {
		if now.Sub(agent.LastSeen) > 2*time.Minute {
			logrus.Warnf("Agent %s heartbeat timeout, disconnecting", agent.ID)
			agent.Conn.Close()
		}
	}
}

// createOrUpdateHostWithMetadata creates or updates a host with metadata from heartbeat
func (h *Hub) createOrUpdateHostWithMetadata(hostID, agentID, agentName, hostname, status string) {
	if database.DB == nil {
		return
	}

	now := time.Now()

	// Try to find existing host
	var host database.Host
	result := database.DB.Where(hostIDQuery, hostID).First(&host)

	if result.Error != nil {
		// Host doesn't exist, create it
		hostUUID, err := uuid.Parse(hostID)
		if err != nil {
			// If hostID is not a valid UUID, generate a new one
			hostUUID = uuid.New()
		}

		host = database.Host{
			ID:           hostUUID,
			Name:         agentName,
			Description:  fmt.Sprintf("Agent running on %s", hostname),
			AgentVersion: "1.0.0",
			Status:       status,
			LastSeen:     &now,
			CreatedAt:    now,
			UpdatedAt:    now,
		}

		if err := database.DB.Create(&host).Error; err != nil {
			logrus.Errorf("Failed to create host %s: %v", hostID, err)
			return
		}

		logrus.Infof("Created new host %s (agent: %s, name: %s)", hostID, agentID, agentName)
	} else {
		// Host exists, update it
		updates := map[string]interface{}{
			"status":     status,
			"last_seen":  &now,
			"updated_at": now,
		}

		// Update name and description if they've changed
		if agentName != "" && host.Name != agentName {
			updates["name"] = agentName
		}
		if hostname != "" {
			updates["description"] = fmt.Sprintf("Agent running on %s", hostname)
		}

		database.DB.Model(&host).Updates(updates)

		logrus.Debugf("Updated existing host %s (agent: %s, name: %s)", hostID, agentID, agentName)
	}
}

// updateHostStatus updates the host status in the database
func (h *Hub) createOrUpdateHost(hostID, agentID string) {
	if database.DB == nil {
		return
	}

	now := time.Now()

	// Try to find existing host
	var host database.Host
	result := database.DB.Where(hostIDQuery, hostID).First(&host)

	if result.Error != nil {
		// Host doesn't exist, create it
		hostUUID, err := uuid.Parse(hostID)
		if err != nil {
			// If hostID is not a valid UUID, generate a new one
			hostUUID = uuid.New()
		}

		host = database.Host{
			ID:           hostUUID,
			Name:         "Test Host",
			Description:  "Test agent host for development",
			AgentVersion: "1.0.0",
			Status:       "online",
			LastSeen:     &now,
			CreatedAt:    now,
			UpdatedAt:    now,
		}

		if err := database.DB.Create(&host).Error; err != nil {
			logrus.Errorf("Failed to create host %s: %v", hostID, err)
			return
		}

		logrus.Infof("Created new host %s (agent: %s)", hostID, agentID)
	} else {
		// Host exists, update it
		database.DB.Model(&host).Updates(map[string]interface{}{
			"status":     "online",
			"last_seen":  &now,
			"updated_at": now,
		})

		logrus.Infof("Updated existing host %s (agent: %s)", hostID, agentID)
	}
}

func (h *Hub) updateHostStatus(hostID string, status string) {
	if database.DB == nil {
		return
	}

	now := time.Now()
	var lastSeen *time.Time
	if status == "online" {
		lastSeen = &now
	}

	database.DB.Model(&database.Host{}).
		Where("id = ?", hostID).
		Updates(map[string]interface{}{
			"status":     status,
			"last_seen":  lastSeen,
			"updated_at": now,
		})
}
