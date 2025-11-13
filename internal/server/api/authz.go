package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func ensureAdmin(c *gin.Context) bool {
	roleValue, exists := c.Get("role")
	if !exists {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return false
	}
	role, ok := roleValue.(string)
	if !ok || !strings.EqualFold(role, "admin") {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return false
	}
	return true
}

func normalizeRole(role string) string {
	return strings.ToLower(strings.TrimSpace(role))
}

func isValidRole(role string) bool {
	switch role {
	case "admin", "user", "viewer":
		return true
	default:
		return false
	}
}
