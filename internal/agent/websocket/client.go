package websocket

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mikeysoft/flotilla/internal/agent/config"
	"github.com/mikeysoft/flotilla/internal/shared/protocol"
	"github.com/sirupsen/logrus"
)

// Client represents a WebSocket client for connecting to the management server
type Client struct {
	config     *config.Config
	conn       *websocket.Conn
	connected  bool
	mu         sync.RWMutex
	stopCh     chan struct{}
	commandCh  chan *protocol.Message
	responseCh chan *protocol.Message
	eventCh    chan *protocol.Message
}

// NewClient creates a new WebSocket client
func NewClient(cfg *config.Config) *Client {
	return &Client{
		config:     cfg,
		connected:  false,
		stopCh:     make(chan struct{}),
		commandCh:  make(chan *protocol.Message, 100),
		responseCh: make(chan *protocol.Message, 100),
		eventCh:    make(chan *protocol.Message, 100),
	}
}

// Connect establishes a WebSocket connection to the server
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return fmt.Errorf("client already connected")
	}

	// Parse server URL
	u, err := url.Parse(c.config.GetServerURL())
	if err != nil {
		return fmt.Errorf("invalid server URL: %w", err)
	}

	// Add API key to query parameters
	q := u.Query()
	q.Set("api_key", c.config.APIKey)
	u.RawQuery = q.Encode()

	// Set up headers
	headers := http.Header{}
	headers.Set("User-Agent", "flotilla-agent/1.0")

	// Connect to WebSocket
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	if strings.EqualFold(os.Getenv("SKIP_TLS_VERIFY"), "true") {
		logrus.Warn("SKIP_TLS_VERIFY is no longer supported; configure trusted certificates instead")
	}
	if c.config.ServerUseTLS || u.Scheme == "wss" {
		dialer.TLSClientConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
	}

	conn, _, err := dialer.Dial(u.String(), headers)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}

	c.conn = conn
	c.connected = true

	logrus.Infof("Connected to server at %s", u.String())

	// Start goroutines for reading and writing
	go c.readPump()
	go c.writePump()
	go c.heartbeatLoop()

	return nil
}

// Disconnect closes the WebSocket connection
func (c *Client) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return nil
	}

	c.connected = false
	close(c.stopCh)

	if c.conn != nil {
		c.conn.Close()
	}

	logrus.Info("Disconnected from server")
	return nil
}

// IsConnected returns whether the client is connected
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// SendCommand sends a command to the server
func (c *Client) SendCommand(command *protocol.Message) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return fmt.Errorf("not connected to server")
	}

	select {
	case c.commandCh <- command:
		return nil
	default:
		return fmt.Errorf("command channel full")
	}
}

// GetResponses returns the responses channel
func (c *Client) GetResponses() <-chan *protocol.Message {
	return c.responseCh
}

// GetEvents returns the events channel
func (c *Client) GetEvents() <-chan *protocol.Message {
	return c.eventCh
}

// readPump pumps messages from the websocket connection
func (c *Client) readPump() {
	defer func() {
		c.mu.Lock()
		c.connected = false
		c.mu.Unlock()
	}()

	c.conn.SetReadLimit(512)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		select {
		case <-c.stopCh:
			return
		default:
			_, messageData, err := c.conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					logrus.Errorf("WebSocket error: %v", err)
				}
				return
			}

			// Parse the message
			msg, err := protocol.DeserializeMessage(messageData)
			if err != nil {
				logrus.Errorf("Failed to parse message from server: %v", err)
				continue
			}

			// Handle different message types
			switch msg.Type {
			case protocol.MessageTypeCommand:
				c.handleCommand(msg)
			case protocol.MessageTypeResponse:
				c.handleResponse(msg)
			case protocol.MessageTypeEvent:
				c.handleEvent(msg)
			default:
				logrus.Warnf("Unknown message type from server: %s", msg.Type)
			}
		}
	}
}

// writePump pumps messages to the websocket connection
func (c *Client) writePump() {
	ticker := time.NewTicker(54 * time.Second) // Send pings
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case command := <-c.commandCh:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			data, err := command.Serialize()
			if err != nil {
				logrus.Errorf("Failed to serialize command: %v", err)
				continue
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				logrus.Errorf("Failed to write command: %v", err)
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				logrus.Errorf("Failed to write ping: %v", err)
				return
			}
		}
	}
}

// heartbeatLoop sends periodic heartbeat messages
func (c *Client) heartbeatLoop() {
	ticker := time.NewTicker(c.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			if c.IsConnected() {
				heartbeat := protocol.NewHeartbeat(
					c.config.AgentID,
					c.config.AgentName,
					getHostname(),
					"healthy",
					0, // Uptime will be calculated by the agent
					0, // This will be updated with actual container count
				)

				// Send heartbeat directly as a message
				data, err := heartbeat.Serialize()
				if err != nil {
					logrus.Errorf("Failed to serialize heartbeat: %v", err)
					continue
				}

				c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
					logrus.Errorf("Failed to send heartbeat: %v", err)
				}
			}
		}
	}
}

// handleCommand handles a command message from the server
func (c *Client) handleCommand(msg *protocol.Message) {
	logrus.Debugf("Received command from server: %s", msg.ID)

	// For now, just echo back a response
	// In the next phase, this will dispatch to command handlers
	response := protocol.NewResponse(msg.ID, "success", map[string]interface{}{
		"message":    "Command received",
		"command_id": msg.ID,
	}, nil)

	select {
	case c.responseCh <- response:
	default:
		logrus.Warn("Response channel full, dropping response")
	}
}

// handleResponse handles a response message from the server
func (c *Client) handleResponse(msg *protocol.Message) {
	logrus.Debugf("Received response from server: %s", msg.ID)

	select {
	case c.responseCh <- msg:
	default:
		logrus.Warn("Response channel full, dropping response")
	}
}

// handleEvent handles an event message from the server
func (c *Client) handleEvent(msg *protocol.Message) {
	logrus.Debugf("Received event from server: %s", msg.ID)

	select {
	case c.eventCh <- msg:
	default:
		logrus.Warn("Event channel full, dropping event")
	}
}

// SendLogEvent sends a log event to the server
func (c *Client) SendLogEvent(containerID, data, stream string, timestamp time.Time) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected || c.conn == nil {
		return fmt.Errorf("client not connected")
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

	c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return c.conn.WriteMessage(websocket.TextMessage, eventData)
}

// Reconnect attempts to reconnect to the server with exponential backoff
// This method is deprecated - reconnection is now handled by the main agent loop
func (c *Client) Reconnect(ctx context.Context) error {
	return fmt.Errorf("reconnect method is deprecated - use main agent loop for reconnection")
}

// getHostname returns the system hostname
func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return hostname
}
