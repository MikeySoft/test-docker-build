package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSecurityHeadersMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	handler := SecurityHeadersMiddleware()
	handler(c)

	if got := recorder.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Fatalf("expected X-Frame-Options header, got %q", got)
	}
	if got := recorder.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("expected X-Content-Type-Options header, got %q", got)
	}
}

func TestCORSMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	c.Request = req

	handler := CORSMiddleware()
	handler(c)

	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Fatalf("expected CORS origin header, got %q", got)
	}
}

func TestCORSMiddlewareOptions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	req, _ := http.NewRequest(http.MethodOptions, "/", nil)
	c.Request = req

	handler := CORSMiddleware()
	handler(c)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected OPTIONS request to be short-circuited with 204, got %d", recorder.Code)
	}
}
