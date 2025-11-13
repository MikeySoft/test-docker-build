package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func TestSetupLoggingSetsLevel(t *testing.T) {
	setupLogging("warn", "json")
	if logrus.GetLevel() != logrus.WarnLevel {
		t.Fatalf("expected warn level, got %s", logrus.GetLevel())
	}
}

func TestErrorOnlyLoggerMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(errorOnlyLogger())
	router.GET("/ok", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	router.GET("/err", func(c *gin.Context) {
		c.Status(http.StatusInternalServerError)
	})

	// Successful request should pass through
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/ok", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 response, got %d", w.Code)
	}

	// Error response should still complete
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodGet, "/err", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 response, got %d", w.Code)
	}
}
