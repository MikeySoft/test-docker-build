package websocket

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/mikeysoft/flotilla/internal/server/auth"
	"github.com/sirupsen/logrus"
)

// AgentWebSocketHandler handles WebSocket connections from agents
func (h *Hub) AgentWebSocketHandler(c *gin.Context) {
	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logrus.Errorf("Failed to upgrade WebSocket connection: %v", err)
		return
	}

	// Get API key and host ID from query parameters
	apiKey := strings.TrimSpace(c.Query("api_key"))
	hostID := strings.TrimSpace(c.Query("host_id"))

	if apiKey == "" {
		logrus.Warn("Agent connection rejected: missing API key")
		conn.Close()
		return
	}

	apiKeyRecord, err := auth.ValidateAPIKey(apiKey)
	if err != nil {
		logrus.Warnf("Agent authentication failed: %v", err)
		conn.Close()
		return
	}

	if apiKeyRecord.HostID != nil {
		hostID = apiKeyRecord.HostID.String()
	}

	if hostID == "" {
		hostUUID := uuid.New()
		hostID = hostUUID.String()
		logrus.WithField("host_id", hostID).Info("Generated new host ID for agent using unbound API key")
	}

	agentID := hostID

	logrus.Infof("Agent %s connecting for host %s", agentID, hostID)

	// Register the agent connection (this will start the read/write pumps)
	h.RegisterAgent(conn, agentID, hostID)
}

// UIWebSocketHandler handles WebSocket connections from UI clients
//
// IMPORTANT: This function only creates and registers the connection.
// The actual pump goroutines (readPump/writePump) are started in registerUIConnection()
// to prevent duplicate goroutine creation which causes concurrent write panics.
func (h *Hub) UIWebSocketHandler(c *gin.Context) {
	token := extractUIAccessToken(c.Request)
	if token == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	if _, err := auth.ParseAccessToken(token); err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	// Copy upgrader to customize subprotocols without affecting agent connections.
	uiUpgrader := upgrader
	uiUpgrader.Subprotocols = []string{"flotilla-ui"}

	conn, err := uiUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logrus.Errorf("Failed to upgrade WebSocket connection: %v", err)
		return
	}

	// Generate a unique client ID
	clientID := generateClientID()

	logrus.Infof("UI client %s connecting", clientID)

	// Register the UI client connection (this will start the pumps)
	// DO NOT start pumps here - they are started in registerUIConnection()
	h.RegisterUI(conn, clientID)
}

// generateClientID generates a unique client ID for UI connections
func generateClientID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func extractUIAccessToken(r *http.Request) string {
	if header := r.Header.Get("Authorization"); strings.HasPrefix(header, "Bearer ") {
		return strings.TrimPrefix(header, "Bearer ")
	}

	// Token provided via Sec-WebSocket-Protocol entry auth-<token>.
	if protocols := r.Header.Get("Sec-WebSocket-Protocol"); protocols != "" {
		for _, part := range strings.Split(protocols, ",") {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, "auth-") {
				return strings.TrimPrefix(part, "auth-")
			}
		}
	}

	return ""
}
