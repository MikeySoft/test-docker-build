package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/mikeysoft/flotilla/internal/agent/commands"
	"github.com/mikeysoft/flotilla/internal/agent/config"
	"github.com/mikeysoft/flotilla/internal/agent/docker"
	"github.com/mikeysoft/flotilla/internal/agent/metrics"
	"github.com/mikeysoft/flotilla/internal/shared/protocol"
	"github.com/sirupsen/logrus"
)

const (
	// Agent ID file paths
	agentIDFile     = "/var/lib/flotilla/agent-id"
	agentIDFileHome = ".flotilla/agent-id"
)

type Agent struct {
	ID               string
	Name             string
	Hostname         string
	Docker           *client.Client
	Config           *config.Config
	StartTime        time.Time
	Conn             *websocket.Conn
	Handler          *commands.Handler
	MetricsCollector *metrics.Collector
	writeMu          sync.Mutex // Protects concurrent writes to websocket
}

func main() {
	// Load configuration
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	// Set up logging
	setupLogging(cfg.LogLevel, cfg.LogFormat)

	// Load or generate agent ID
	agentID := loadOrGenerateAgentID(cfg.AgentID)

	// Initialize Docker client
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatalf("Failed to create Docker client: %v", err)
	}
	defer dockerClient.Close()

	// Test Docker connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err = dockerClient.Ping(ctx)
	if err != nil {
		log.Fatalf("Failed to connect to Docker daemon: %v", err)
	}

	// Get hostname
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	// Create Docker wrapper client
	dockerWrapper := docker.NewClient(dockerClient)

	// Create command handler
	commandHandler := commands.NewHandler(dockerWrapper)

	// Create metrics collector (use agentID as hostID for now, will be updated after connection)
	metricsCollector := metrics.NewCollector(cfg, dockerWrapper, agentID, agentID)

	// Create agent instance
	agent := &Agent{
		ID:               agentID,
		Name:             cfg.AgentName,
		Hostname:         hostname,
		Docker:           dockerClient,
		Config:           cfg,
		StartTime:        time.Now(),
		Handler:          commandHandler,
		MetricsCollector: metricsCollector,
	}

	// Set up WebSocket client wrapper for command handler
	wsWrapper := &WebSocketWrapper{agent: agent}
	commandHandler.SetWebSocketClient(wsWrapper)

	// Set up metrics sender wrapper
	metricsSender := &MetricsSenderWrapper{agent: agent}
	metricsCollector.SetMetricsSender(metricsSender)

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	logrus.Infof("Agent starting: %s (ID: %s)", agent.Name, agent.ID)
	logrus.Infof("Connecting to server: %s", cfg.GetServerURL())

	// Main connection loop with exponential backoff
	backoff := time.Second
	maxBackoff := 30 * time.Second

	for {
		// Attempt to connect and run
		if err := agent.connectAndRun(); err != nil {
			// Check if this was a shutdown request
			if err.Error() == "shutdown requested" {
				logrus.Info("Shutdown requested, exiting...")
				return
			}

			logrus.Errorf("Connection lost: %v", err)
			logrus.Infof("Retrying in %v...", backoff)

			select {
			case <-sigChan:
				logrus.Info("Received shutdown signal, exiting...")
				return
			case <-time.After(backoff):
				// Double the backoff time, but cap it at maxBackoff
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
			}
		} else {
			// Connection was successful, reset backoff
			backoff = time.Second
		}
	}
}

// connectAndRun establishes a connection and runs the agent until disconnected
func (a *Agent) connectAndRun() error {
	// Connect to WebSocket with host ID as query parameter
	wsURL, err := url.Parse(a.Config.GetServerURL())
	if err != nil {
		return fmt.Errorf("failed to parse server URL: %w", err)
	}
	query := wsURL.Query()
	query.Set("host_id", a.ID)
	if key := strings.TrimSpace(a.Config.APIKey); key != "" {
		query.Set("api_key", key)
	}
	wsURL.RawQuery = query.Encode()
	// Configure dialer to honor SKIP_TLS_VERIFY or DEV mode
	dialer := *websocket.DefaultDialer
	if strings.EqualFold(os.Getenv("SKIP_TLS_VERIFY"), "true") {
		logrus.Warn("SKIP_TLS_VERIFY is no longer supported; configure trusted certificates instead")
	}
	if a.Config.ServerUseTLS {
		dialer.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}

	conn, _, err := dialer.Dial(wsURL.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer conn.Close()

	a.Conn = conn
	logrus.Info("Connected to server successfully")

	// Update metrics collector with the correct host ID (same as agent ID in testing mode)
	a.MetricsCollector.SetHostID(a.ID)

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start Docker event monitoring
	dockerCtx, dockerCancel := context.WithCancel(context.Background())
	defer dockerCancel()
	go a.monitorDockerEvents(dockerCtx)

	// Start metrics collection if enabled
	if a.MetricsCollector != nil && a.Config.MetricsEnabled {
		go a.MetricsCollector.Start(dockerCtx)
		defer a.MetricsCollector.Stop()
	}

	// Start heartbeat loop
	heartbeatTicker := time.NewTicker(30 * time.Second)
	defer heartbeatTicker.Stop()

	// Start message reading goroutine
	messageCh := make(chan *protocol.Message, 100)
	go a.readMessages(conn, messageCh)

	// Start ping/pong goroutine to keep connection alive
	go a.pingPongLoop(conn)

	// Main message handling loop
	for {
		select {
		case msg, ok := <-messageCh:
			if !ok {
				// Channel closed, connection lost
				logrus.Info("Message channel closed, connection lost")
				return fmt.Errorf("connection lost")
			}
			if msg == nil {
				logrus.Warn("Received nil message, skipping")
				continue
			}
			logrus.Infof("Received message: type=%s, id=%s", msg.Type, msg.ID)
			if msg.Type == protocol.MessageTypeCommand {
				a.handleCommand(msg)
			} else {
				logrus.Debugf("Received message type: %s", msg.Type)
			}
		case <-heartbeatTicker.C:
			a.sendHeartbeat(conn)
		case <-dockerCtx.Done():
			return fmt.Errorf("context cancelled")
		case <-sigChan:
			logrus.Info("Received shutdown signal in connectAndRun, exiting...")
			return fmt.Errorf("shutdown requested")
		}
	}
}

// loadOrGenerateAgentID loads agent ID from file or generates a new one
func loadOrGenerateAgentID(envAgentID string) string {
	// If provided via environment variable, use it
	if envAgentID != "" {
		logrus.Infof("Using agent ID from environment: %s", envAgentID)
		return envAgentID
	}

	// Try to load from file
	agentID := loadAgentIDFromFile()
	if agentID != "" {
		logrus.Infof("Loaded agent ID from file: %s", agentID)
		return agentID
	}

	// Generate new ID and save to file
	agentID = uuid.New().String()
	if err := saveAgentIDToFile(agentID); err != nil {
		logrus.Warnf("Failed to save agent ID to file: %v", err)
	} else {
		logrus.Infof("Generated new agent ID and saved to file: %s", agentID)
	}

	return agentID
}

// loadAgentIDFromFile loads agent ID from the persistence file
func loadAgentIDFromFile() string {
	// Try system path first
	// #nosec G304 -- fixed agent ID path under /var/lib
	if data, err := os.ReadFile(agentIDFile); err == nil {
		var idData struct {
			AgentID string `json:"agent_id"`
		}
		if json.Unmarshal(data, &idData) == nil && idData.AgentID != "" {
			return idData.AgentID
		}
	}

	// Try home directory as fallback
	homeDir, err := os.UserHomeDir()
	if err == nil {
		homePath := filepath.Join(homeDir, agentIDFileHome)
		// #nosec G304 -- path within user home .flotilla directory
		if data, err := os.ReadFile(homePath); err == nil {
			var idData struct {
				AgentID string `json:"agent_id"`
			}
			if json.Unmarshal(data, &idData) == nil && idData.AgentID != "" {
				return idData.AgentID
			}
		}
	}

	return ""
}

// saveAgentIDToFile saves agent ID to the persistence file
func saveAgentIDToFile(agentID string) error {
	idData := struct {
		AgentID string `json:"agent_id"`
	}{
		AgentID: agentID,
	}

	data, err := json.Marshal(idData)
	if err != nil {
		return err
	}

	// Try system path first
	if err := os.MkdirAll(filepath.Dir(agentIDFile), 0o750); err == nil {
		if err := os.WriteFile(agentIDFile, data, 0o600); err == nil {
			return nil
		}
	}

	// Try home directory as fallback
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	homePath := filepath.Join(homeDir, agentIDFileHome)
	if err := os.MkdirAll(filepath.Dir(homePath), 0o750); err != nil {
		return err
	}

	return os.WriteFile(homePath, data, 0o600)
}

// setupLogging configures the logging system
func setupLogging(level, format string) {
	// Set log level
	switch level {
	case "debug":
		logrus.SetLevel(logrus.DebugLevel)
	case "info":
		logrus.SetLevel(logrus.InfoLevel)
	case "warn":
		logrus.SetLevel(logrus.WarnLevel)
	case "error":
		logrus.SetLevel(logrus.ErrorLevel)
	default:
		logrus.SetLevel(logrus.InfoLevel)
	}

	// Set log format
	if format == "json" {
		logrus.SetFormatter(&logrus.JSONFormatter{})
	} else {
		logrus.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
		})
	}
}

// monitorDockerEvents monitors Docker events and sends them to the server
func (a *Agent) monitorDockerEvents(ctx context.Context) {
	// This is a placeholder for Docker event monitoring
	// In a full implementation, this would:
	// 1. Subscribe to Docker events
	// 2. Filter relevant events (container start/stop/die, etc.)
	// 3. Send events to the server via WebSocket
	logrus.Debug("Docker event monitoring started")
	<-ctx.Done()
	logrus.Debug("Docker event monitoring stopped")
}

// handleResponse handles responses from the server
func (a *Agent) handleResponse(response *protocol.Message) {
	logrus.Debugf("Received response: %s", response.ID)
	// Handle response based on command type
	// This would be implemented based on the specific command
}

// handleEvent handles events from the server
func (a *Agent) handleEvent(event *protocol.Message) {
	logrus.Debugf("Received event: %s", event.ID)
	// Handle event based on event type
	// This would be implemented based on the specific event
}

// handleCommand handles commands from the server
func (a *Agent) handleCommand(command *protocol.Message) {
	logrus.Infof("Received command: %s, type: %s", command.ID, command.Type)

	// Parse command
	cmd, err := command.GetCommand()
	if err != nil {
		logrus.Errorf("Failed to parse command: %v", err)
		a.sendErrorResponse(command.ID, fmt.Sprintf("Failed to parse command: %v", err))
		return
	}

	logrus.Debugf("Command action: %s, params: %+v", cmd.Action, cmd.Params)

	// Use the command handler to process the command
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	response, err := a.Handler.HandleCommand(ctx, command)
	if err != nil {
		logrus.Errorf("Failed to handle command: %v", err)
		a.sendErrorResponse(command.ID, fmt.Sprintf("Failed to handle command: %v", err))
		return
	}

	// Send the response back to the server
	a.sendResponse(response)
}

// handleListContainers handles the list_containers command
func (a *Agent) handleListContainers(commandID string) {
	logrus.Infof("Starting to list containers for command %s", commandID)

	containers, err := a.Docker.ContainerList(context.Background(), types.ContainerListOptions{All: true})
	if err != nil {
		logrus.Errorf("Failed to list containers: %v", err)
		a.sendErrorResponse(commandID, fmt.Sprintf("Failed to list containers: %v", err))
		return
	}

	logrus.Infof("Found %d containers", len(containers))

	var containerInfos []map[string]interface{}
	for i, container := range containers {
		logrus.Debugf("Processing container %d: %s", i, container.ID)

		name := ""
		if len(container.Names) > 0 {
			name = container.Names[0][1:] // Remove leading slash
		}
		// Map Docker status to our expected status values
		status := "stopped"
		if container.State == "running" {
			status = "running"
		} else if container.State == "paused" {
			status = "paused"
		} else if container.State == "restarting" {
			status = "restarting"
		} else if container.State == "exited" {
			status = "exited"
		}

		// Ensure all fields are properly formatted for frontend
		containerInfo := map[string]interface{}{
			"id":      container.ID,
			"name":    name,
			"image":   container.Image,
			"status":  status,
			"created": time.Unix(container.Created, 0).Format(time.RFC3339),
		}

		// Validate that all required fields are present and non-empty
		if containerInfo["id"] == "" || containerInfo["name"] == "" || containerInfo["image"] == "" {
			logrus.WithFields(logrus.Fields{
				"id":    containerInfo["id"],
				"name":  containerInfo["name"],
				"image": containerInfo["image"],
			}).Warn("Skipping container with missing required fields")
			continue
		}

		// Ensure status is one of the expected values
		validStatuses := map[string]bool{
			"running":    true,
			"stopped":    true,
			"exited":     true,
			"paused":     true,
			"restarting": true,
		}
		if !validStatuses[status] {
			logrus.Warnf("Invalid container status '%s', defaulting to 'stopped'", status)
			containerInfo["status"] = "stopped"
		}
		containerInfos = append(containerInfos, containerInfo)
	}

	logrus.Infof("Preparing response with %d containers", len(containerInfos))

	response := protocol.NewResponse(commandID, "success", map[string]interface{}{
		"containers": containerInfos,
	}, nil)

	// Send response back to server
	logrus.Infof("Sending response for command %s", commandID)
	a.sendResponse(response)
	logrus.Infof("Successfully sent container list response: %d containers", len(containerInfos))
}

// handleListImages handles the list_images command
func (a *Agent) handleListImages(commandID string) {
	images, err := a.Docker.ImageList(context.Background(), types.ImageListOptions{})
	if err != nil {
		a.sendErrorResponse(commandID, fmt.Sprintf("Failed to list images: %v", err))
		return
	}

	response := protocol.NewResponse(commandID, "success", map[string]interface{}{
		"images": images,
	}, nil)

	// Send response back to server
	a.sendResponse(response)
	logrus.Debugf("Sending image list response: %d images", len(images))
}

// sendResponse sends a response back to the server
func (a *Agent) sendResponse(response *protocol.Message) {
	logrus.Infof("Sending response: ID=%s, Type=%s", response.ID, response.Type)

	if a.Conn == nil {
		logrus.Error("No WebSocket connection available")
		return
	}

	data, err := response.Serialize()
	if err != nil {
		logrus.Errorf("Failed to serialize response: %v", err)
		return
	}

	logrus.Debugf("Serialized response data length: %d bytes", len(data))

	// Lock mutex to prevent concurrent writes to websocket
	a.writeMu.Lock()
	defer a.writeMu.Unlock()

	if err := a.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
		logrus.WithError(err).Warn("Failed to set write deadline for response")
		return
	}
	if err := a.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
		logrus.Errorf("Failed to send response: %v", err)
		return
	}

	logrus.Infof("Successfully sent response: ID=%s", response.ID)
}

// sendErrorResponse sends an error response
func (a *Agent) sendErrorResponse(commandID, errorMsg string) {
	response := protocol.NewResponse(commandID, "error", nil, fmt.Errorf("%s", errorMsg))
	a.sendResponse(response)
	logrus.Errorf("Sending error response: %s", errorMsg)
}

// getContainerCount returns the number of running containers
func (a *Agent) getContainerCount() int {
	containers, err := a.Docker.ContainerList(context.Background(), types.ContainerListOptions{})
	if err != nil {
		return 0
	}
	return len(containers)
}

// getUptime returns the agent uptime in seconds
func (a *Agent) getUptime() int64 {
	return int64(time.Since(a.StartTime).Seconds())
}

// readMessages reads messages from the WebSocket connection
func (a *Agent) readMessages(conn *websocket.Conn, messageCh chan<- *protocol.Message) {
	defer close(messageCh)

	// Set up pong handler
	conn.SetPongHandler(func(string) error {
		if err := conn.SetReadDeadline(time.Now().Add(60 * time.Second)); err != nil {
			logrus.WithError(err).Warn("Failed to extend read deadline after pong")
		}
		return nil
	})

	// Set initial read deadline
	if err := conn.SetReadDeadline(time.Now().Add(60 * time.Second)); err != nil {
		logrus.WithError(err).Warn("Failed to set initial read deadline")
	}

	for {
		_, messageData, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logrus.Errorf("WebSocket read error: %v", err)
			} else {
				logrus.Info("WebSocket connection closed")
			}
			return
		}

		// Update read deadline after successful read
		if err := conn.SetReadDeadline(time.Now().Add(60 * time.Second)); err != nil {
			logrus.WithError(err).Warn("Failed to extend read deadline after message")
		}

		msg, err := protocol.DeserializeMessage(messageData)
		if err != nil {
			logrus.Errorf("Failed to parse message: %v", err)
			continue
		}

		// Only send non-nil messages
		if msg != nil {
			select {
			case messageCh <- msg:
			default:
				logrus.Warn("Message channel full, dropping message")
			}
		}
	}
}

// writeMessages handles writing messages to the WebSocket connection
func (a *Agent) writeMessages(conn *websocket.Conn, writeCh <-chan []byte) {
	ticker := time.NewTicker(54 * time.Second) // Ping period
	defer ticker.Stop()

	for {
		select {
		case message, ok := <-writeCh:
			if err := conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
				logrus.WithError(err).Warn("Failed to set write deadline for outgoing message")
				return
			}
			if !ok {
				if err := conn.WriteMessage(websocket.CloseMessage, []byte{}); err != nil && !errors.Is(err, websocket.ErrCloseSent) {
					logrus.WithError(err).Debug("Failed to send close message")
				}
				return
			}

			if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
				logrus.Errorf("Failed to write message: %v", err)
				return
			}

		case <-ticker.C:
			if err := conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
				logrus.WithError(err).Warn("Failed to set write deadline for ping")
				return
			}
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				logrus.Errorf("Failed to send ping: %v", err)
				return
			}
		}
	}
}

// sendHeartbeat sends a heartbeat to the server
func (a *Agent) sendHeartbeat(conn *websocket.Conn) {
	heartbeat := protocol.NewHeartbeat(
		a.ID,
		a.Name,
		a.Hostname,
		"healthy",
		a.getUptime(),
		a.getContainerCount(),
	)

	data, err := heartbeat.Serialize()
	if err != nil {
		logrus.Errorf("Failed to serialize heartbeat: %v", err)
		return
	}

	// Lock mutex to prevent concurrent writes to websocket
	a.writeMu.Lock()
	defer a.writeMu.Unlock()

	if err := conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
		logrus.WithError(err).Warn("Failed to set write deadline for heartbeat")
		return
	}
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		logrus.Errorf("Failed to send heartbeat: %v", err)
	}
}

// pingPongLoop handles ping/pong to keep the connection alive
func (a *Agent) pingPongLoop(conn *websocket.Conn) {
	ticker := time.NewTicker(30 * time.Second) // Send pings every 30 seconds
	defer ticker.Stop()

	for range ticker.C {
		// Lock mutex to prevent concurrent writes to websocket
		a.writeMu.Lock()
		err := conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		if err == nil {
			err = conn.WriteMessage(websocket.PingMessage, nil)
		}
		a.writeMu.Unlock()

		if err != nil {
			logrus.Errorf("Failed to send ping: %v", err)
			return
		}
	}
}

// WebSocketWrapper wraps the agent's WebSocket connection to implement the WebSocketClient interface
type WebSocketWrapper struct {
	agent *Agent
}

// SendLogEvent sends a log event via the agent's WebSocket connection
func (w *WebSocketWrapper) SendLogEvent(containerID, data, stream string, timestamp time.Time) error {
	if w.agent.Conn == nil {
		return fmt.Errorf("no WebSocket connection available")
	}

	event := protocol.NewEvent("log_data", map[string]interface{}{
		"container_id": containerID,
		"data":         data,
		"timestamp":    timestamp.Format(time.RFC3339),
		"stream":       stream,
	})

	eventData, err := event.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize log event: %v", err)
	}

	// Lock mutex to prevent concurrent writes to websocket
	w.agent.writeMu.Lock()
	defer w.agent.writeMu.Unlock()

	if err := w.agent.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return fmt.Errorf("failed to set log event write deadline: %w", err)
	}
	if err := w.agent.Conn.WriteMessage(websocket.TextMessage, eventData); err != nil {
		return fmt.Errorf("failed to send log event: %w", err)
	}
	return nil
}

// MetricsSenderWrapper wraps the agent's WebSocket connection to implement the MetricsSender interface
type MetricsSenderWrapper struct {
	agent *Agent
}

// SendMetrics sends metrics via the agent's WebSocket connection
func (m *MetricsSenderWrapper) SendMetrics(message *protocol.Message) error {
	if m.agent.Conn == nil {
		return fmt.Errorf("no WebSocket connection available")
	}

	data, err := message.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize metrics message: %v", err)
	}

	// Lock mutex to prevent concurrent writes to websocket
	m.agent.writeMu.Lock()
	defer m.agent.writeMu.Unlock()

	if err := m.agent.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return fmt.Errorf("failed to set metrics write deadline: %w", err)
	}
	if err := m.agent.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
		return fmt.Errorf("failed to send metrics message: %w", err)
	}
	return nil
}
