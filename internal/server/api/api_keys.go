package api

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/mikeysoft/flotilla/internal/server/auth"
	"github.com/mikeysoft/flotilla/internal/server/database"
	"github.com/sirupsen/logrus"
)

const (
	userAgentHeader = "User-Agent"
)

// APIKeysHandler handles API key-related endpoints
type APIKeysHandler struct{}

// NewAPIKeysHandler creates a new API keys handler
func NewAPIKeysHandler() *APIKeysHandler {
	return &APIKeysHandler{}
}

// CreateAPIKeyRequest represents the request to create an API key
type CreateAPIKeyRequest struct {
	Name   string `json:"name" binding:"required"`
	HostID string `json:"host_id,omitempty"`
}

// CreateAPIKeyResponse represents the response after creating an API key
type CreateAPIKeyResponse struct {
	APIKey string `json:"api_key"`
	Prefix string `json:"prefix"`
	Name   string `json:"name"`
	HostID string `json:"host_id,omitempty"`
}

// APIKeyResponse represents an API key in responses (without secret)
type APIKeyResponse struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Prefix    string     `json:"prefix"`
	HostID    *string    `json:"host_id,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	LastUsed  *time.Time `json:"last_used,omitempty"`
	IsActive  bool       `json:"is_active"`
}

// CreateAPIKey creates a new API key for agent authentication
func (h *APIKeysHandler) CreateAPIKey(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req CreateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request: " + err.Error(),
		})
		return
	}

	// Get current user ID from context
	userIDStr, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}
	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
		return
	}

	// Generate API key with prefix
	prefix := generateAPIKeyPrefix()
	secret := generateAPIKeySecret()
	fullKey := fmt.Sprintf("FLA_%s_%s", prefix, secret)

	// Hash the secret for storage
	secretHash, err := auth.HashPassword(secret)
	if err != nil {
		logrus.Errorf("Failed to hash API key: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to generate API key",
		})
		return
	}

	// Parse host ID if provided
	var hostUUID *uuid.UUID
	if req.HostID != "" {
		if parsedUUID, err := uuid.Parse(req.HostID); err == nil {
			hostUUID = &parsedUUID
		}
	}

	// Create API key record
	apiKeyRecord := database.APIKey{
		KeyHash:   secretHash,
		Name:      req.Name,
		Prefix:    &prefix,
		HostID:    hostUUID,
		CreatedBy: &userID,
		IsActive:  true,
	}

	// Save to database
	if err := database.DB.Create(&apiKeyRecord).Error; err != nil {
		logrus.Errorf("Failed to create API key: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create API key",
		})
		return
	}

	// Mask the API key in logs: show prefix and last 4 chars, plus length
	masked := "<masked>"
	if len(fullKey) > 12 {
		masked = fullKey[:8] + "..." + fullKey[len(fullKey)-4:]
	}
	logrus.Infof("Generated API key for %s: %s (len=%d)", req.Name, masked, len(fullKey))

	// Audit log API key creation
	if err := auth.LogAuditEvent(&userID, "api_key_created", "api_key", &apiKeyRecord.ID, map[string]interface{}{
		"name":    req.Name,
		"prefix":  prefix,
		"host_id": req.HostID,
	}, c.ClientIP(), c.GetHeader(userAgentHeader)); err != nil {
		logrus.WithError(err).Warn("Failed to record api_key_created audit event")
	}

	c.JSON(http.StatusCreated, CreateAPIKeyResponse{
		APIKey: fullKey,
		Prefix: prefix,
		Name:   req.Name,
		HostID: req.HostID,
	})
}

// ListAPIKeys returns all API keys
func (h *APIKeysHandler) ListAPIKeys(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var apiKeys []database.APIKey
	if err := database.DB.Find(&apiKeys).Error; err != nil {
		logrus.Errorf("Failed to list API keys: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve API keys",
		})
		return
	}

	// Convert to response format (without secrets)
	responses := make([]APIKeyResponse, len(apiKeys))
	for i, key := range apiKeys {
		var hostID *string
		if key.HostID != nil {
			s := key.HostID.String()
			hostID = &s
		}

		// Handle nil prefix (for old API keys created before prefix was added)
		prefix := ""
		if key.Prefix != nil {
			prefix = *key.Prefix
		}

		responses[i] = APIKeyResponse{
			ID:        key.ID.String(),
			Name:      key.Name,
			Prefix:    prefix,
			HostID:    hostID,
			CreatedAt: key.CreatedAt,
			LastUsed:  key.LastUsed,
			IsActive:  key.IsActive,
		}
	}

	c.JSON(http.StatusOK, responses)
}

// RevokeAPIKey revokes an API key by ID
func (h *APIKeysHandler) RevokeAPIKey(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	keyID := c.Param("id")
	if keyID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "API key ID is required",
		})
		return
	}

	// Parse UUID
	keyUUID, err := uuid.Parse(keyID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid API key ID",
		})
		return
	}

	// Update the key to be inactive and set revoked timestamp
	now := time.Now()
	result := database.DB.Model(&database.APIKey{}).
		Where("id = ?", keyUUID).
		Updates(map[string]interface{}{
			"is_active":  false,
			"revoked_at": &now,
		})

	if result.Error != nil {
		logrus.Errorf("Failed to revoke API key: %v", result.Error)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to revoke API key",
		})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "API key not found",
		})
		return
	}

	logrus.Infof("Revoked API key: %s", keyID)

	// Get current user ID for audit log
	userIDStr, exists := c.Get("user_id")
	if exists {
		if userUUID, err := uuid.Parse(userIDStr.(string)); err == nil {
			if err := auth.LogAuditEvent(&userUUID, "api_key_revoked", "api_key", &keyUUID, map[string]interface{}{
				"key_id": keyID,
			}, c.ClientIP(), c.GetHeader(userAgentHeader)); err != nil {
				logrus.WithError(err).Warn("Failed to record api_key_revoked audit event")
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "API key revoked successfully",
	})
}

// Helper functions for API key generation
func generateAPIKeyPrefix() string {
	bytes := make([]byte, 4)
	if _, err := rand.Read(bytes); err != nil {
		logrus.WithError(err).Warn("Failed to generate random API key prefix; using fallback")
		fallback := hex.EncodeToString([]byte(time.Now().Format(time.RFC3339Nano)))
		if len(fallback) < 8 {
			return strings.ToUpper(fallback)
		}
		return strings.ToUpper(fallback[:8])
	}
	return strings.ToUpper(hex.EncodeToString(bytes))
}

func generateAPIKeySecret() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		logrus.WithError(err).Warn("Failed to generate random API key secret; using UUID fallback")
		return strings.ReplaceAll(uuid.New().String(), "-", "")
	}
	return hex.EncodeToString(bytes)
}

// DeleteAPIKeyPermanently permanently deletes an API key
func (h *APIKeysHandler) DeleteAPIKeyPermanently(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	keyID := c.Param("id")
	if keyID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "API key ID is required"})
		return
	}

	// Parse UUID
	keyUUID, err := uuid.Parse(keyID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid API key ID"})
		return
	}

	// Check if API key exists and is revoked
	var key database.APIKey
	if err := database.DB.Where("id = ?", keyUUID).First(&key).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "API key not found"})
		return
	}

	if key.IsActive {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot delete active API key. Revoke it first."})
		return
	}

	// Get user ID from JWT for audit log
	userIDStr, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
		return
	}

	// Audit log
	if err := auth.LogAuditEvent(&userID, "api_key_deleted", "api_key", &keyUUID, map[string]interface{}{
		"key_name":   key.Name,
		"key_prefix": key.Prefix,
	}, c.ClientIP(), c.GetHeader(userAgentHeader)); err != nil {
		logrus.WithError(err).Warn("Failed to record api_key_deleted audit event")
	}

	// Permanently delete the API key
	if err := database.DB.Unscoped().Delete(&key).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete API key"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
