package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/mikeysoft/flotilla/internal/server/auth"
	appLogs "github.com/mikeysoft/flotilla/internal/server/logs"
)

// LogsHandler exposes application logs over HTTP/WebSocket.
type LogsHandler struct {
	manager  *appLogs.Manager
	upgrader websocket.Upgrader
}

// NewLogsHandler creates a new logs handler.
func NewLogsHandler(manager *appLogs.Manager) *LogsHandler {
	return &LogsHandler{
		manager: manager,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
}

// ListLogs returns recent application logs.
func (h *LogsHandler) ListLogs(c *gin.Context) {
	after := c.Query("after")
	limitStr := c.DefaultQuery("limit", "200")
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit"})
		return
	}

	entries := h.manager.List(after, limit)
	next := ""
	if len(entries) > 0 {
		next = entries[len(entries)-1].ID
	}

	c.JSON(http.StatusOK, gin.H{
		"logs":        entries,
		"next_cursor": next,
	})
}

// StreamLogs upgrades to a WebSocket connection and streams log entries.
func (h *LogsHandler) StreamLogs(c *gin.Context) {
	token := ""
	header := c.GetHeader("Authorization")
	if len(header) >= 8 && header[:7] == "Bearer " {
		token = header[7:]
	}
	if token == "" {
		token = c.Query("token")
	}
	if token == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	if _, err := auth.ParseAccessToken(token); err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	ch, unsubscribe := h.manager.Subscribe()
	defer unsubscribe()

	conn.SetCloseHandler(func(code int, text string) error {
		return nil
	})

	for entry := range ch {
		if err := conn.WriteJSON(entry); err != nil {
			return
		}
	}
}
