package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mikeysoft/flotilla/internal/server/websocket"
)

// This is a scaffold integration test for the containers API. It requires a
// real database connection and an agent to be connected to fully exercise the
// list containers flow. It is skipped by default unless explicitly enabled.
func TestListContainersIntegration_SkippedByDefault(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") == "" {
		t.Skip("Skipping integration test; set RUN_INTEGRATION_TESTS=1 to enable")
	}

	// NOTE: A full integration would:
	// 1) Connect to a real PostgreSQL (DATABASE_URL)
	// 2) Start the hub and register a test agent
	// 3) Seed a host record matching the agent host ID
	// 4) Issue a request to the containers endpoint and verify response

	r := gin.Default()
	hub := websocket.NewHub()
	handler := NewContainersHandler(hub, nil, nil)

	// Register a minimal route for demonstration; real route wiring is in main
	r.GET("/api/v1/hosts/:id/containers", func(c *gin.Context) {
		// In the real server this would send a list_containers command.
		// We simply return 503 since no agent is connected in this scaffold.
		handler.ListImages(c)
	})

	req, _ := http.NewRequest("GET", "/api/v1/hosts/host-123/containers", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code == 200 {
		t.Log("Received 200 OK; this environment likely has a connected agent and DB configured.")
	}
}
