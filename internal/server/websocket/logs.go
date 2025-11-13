package websocket

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/mikeysoft/flotilla/internal/server/auth"
	"github.com/mikeysoft/flotilla/internal/shared/protocol"
	"github.com/sirupsen/logrus"
)

// LogStreamConnection represents a WebSocket connection for log streaming
type LogStreamConnection struct {
	ID           string
	Conn         *websocket.Conn
	Send         chan []byte
	ContainerID  string
	HostID       string
	Hub          *Hub
	PumpsStarted bool
}

// LogStreamHandler handles WebSocket connections for log streaming
func (h *Hub) LogStreamHandler(c *gin.Context) {
	// Validate access JWT from Authorization header or token query param
	token := ""
	header := c.GetHeader("Authorization")
	if len(header) >= 8 && header[:7] == "Bearer " {
		token = header[7:]
	} else {
		token = c.Query("token")
	}
	if token == "" {
		c.AbortWithStatusJSON(401, gin.H{"error": "unauthorized"})
		return
	}
	if _, err := auth.ParseAccessToken(token); err != nil {
		c.AbortWithStatusJSON(401, gin.H{"error": "unauthorized"})
		return
	}

	// Determine expected origin for CSRF protection
	expectedOrigin := "http://" + c.Request.Host
	if c.Request.TLS != nil {
		expectedOrigin = "https://" + c.Request.Host
	}

	// Upgrade HTTP connection to WebSocket
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			if origin == "" {
				return true
			}
			return origin == expectedOrigin
		},
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logrus.Errorf("Failed to upgrade log stream connection: %v", err)
		return
	}

	// Parse path parameters
	hostID := c.Param("host_id")
	containerID := c.Param("container_id")

	// Parse query parameters
	query := c.Request.URL.Query()
	follow := query.Get("follow") == "true"
	tail := query.Get("tail")
	timestamps := query.Get("timestamps") == "true"

	if containerID == "" || hostID == "" {
		logrus.Errorf("Missing required parameters: container_id=%s, host_id=%s", containerID, hostID)
		if err := conn.Close(); err != nil && !errors.Is(err, websocket.ErrCloseSent) {
			logrus.WithError(err).Debug("failed to close invalid log stream connection")
		}
		return
	}

	// Create log stream connection
	logConn := &LogStreamConnection{
		ID:          generateID(),
		Conn:        conn,
		Send:        make(chan []byte, 256),
		ContainerID: containerID,
		HostID:      hostID,
		Hub:         h,
	}

	// Register the connection
	h.registerLogStream <- logConn

	// Start the connection pumps
	go logConn.startPumps()

	// Start log streaming
	timestampsStr := "false"
	if timestamps {
		timestampsStr = "true"
	}
	go logConn.startLogStream(follow, tail, timestampsStr)

	logrus.Infof("Log stream connection established for container %s on host %s", containerID, hostID)
}

// startPumps starts the read and write pumps for the log stream connection
func (c *LogStreamConnection) startPumps() {
	defer func() {
		c.Hub.unregisterLogStream <- c
		if err := c.Conn.Close(); err != nil && !errors.Is(err, websocket.ErrCloseSent) {
			logrus.WithError(err).Debugf("Failed to close log stream connection %s", c.ID)
		}
	}()

	// Set up connection parameters
	c.Conn.SetReadLimit(maxMessageSize)
	if err := c.Conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		logrus.WithError(err).Warnf("Failed to set read deadline for log stream %s", c.ID)
	}
	c.Conn.SetPongHandler(func(string) error {
		if err := c.Conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
			logrus.WithError(err).Warnf("Failed to extend read deadline for log stream %s", c.ID)
		}
		return nil
	})

	// Start write pump
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case message, ok := <-c.Send:
			if err := c.Conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				logrus.WithError(err).Warnf("Failed to set write deadline for log stream %s", c.ID)
				return
			}
			if !ok {
				if err := c.Conn.WriteMessage(websocket.CloseMessage, []byte{}); err != nil && !errors.Is(err, websocket.ErrCloseSent) {
					logrus.WithError(err).Debugf("Failed to send close message for log stream %s", c.ID)
				}
				return
			}

			if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			if err := c.Conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				logrus.WithError(err).Warnf("Failed to set ping deadline for log stream %s", c.ID)
				return
			}
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// startLogStream starts streaming logs from the agent
func (c *LogStreamConnection) startLogStream(follow bool, tail, timestamps string) {
	// For now, this is a placeholder implementation
	// In a real implementation, this would:
	// 1. Send a command to the agent to start log streaming
	// 2. Receive log chunks from the agent
	// 3. Forward them to the UI client

	logrus.Infof("Starting log stream for container %s (follow=%v, tail=%s, timestamps=%v)",
		c.ContainerID, follow, tail, timestamps)

	// Send initial connection message
	initialMessage := map[string]interface{}{
		"type": "log_connected",
		"payload": map[string]interface{}{
			"container_id": c.ContainerID,
			"host_id":      c.HostID,
			"follow":       follow,
			"tail":         tail,
			"timestamps":   timestamps,
		},
	}

	if data, err := json.Marshal(initialMessage); err == nil {
		select {
		case c.Send <- data:
		case <-time.After(5 * time.Second):
			logrus.Warnf("Failed to send initial message to log stream client %s", c.ID)
		}
	}

	// Convert timestamps string to boolean
	timestampsBool := timestamps == "true"

	// Send a command to the agent to start log streaming
	command := protocol.NewCommandWithAction("stream_container_logs", map[string]any{
		"container_id": c.ContainerID,
		"follow":       follow,
		"tail":         tail,
		"timestamps":   timestampsBool,
	})

	// Find the agent for this host
	agent := c.Hub.GetAgentByHostID(c.HostID)
	if agent == nil {
		logrus.Errorf("No agent found for host %s", c.HostID)
		errorMessage := map[string]interface{}{
			"type": "log_error",
			"payload": map[string]interface{}{
				"error": "No agent connected for this host",
			},
		}
		if data, err := json.Marshal(errorMessage); err == nil {
			select {
			case c.Send <- data:
			case <-time.After(5 * time.Second):
			}
		}
		return
	}

	// Send command to agent
	commandData, err := command.Serialize()
	if err != nil {
		logrus.Errorf("Failed to serialize log stream command: %v", err)
		return
	}

	agent.Send <- commandData
	logrus.Infof("Sent log stream command to agent %s for container %s", agent.ID, c.ContainerID)
}

// generateID generates a unique ID for log stream connections
func generateID() string {
	return uuid.New().String()
}
