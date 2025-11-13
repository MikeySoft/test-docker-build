package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mikeysoft/flotilla/internal/server/database"
	"github.com/sirupsen/logrus"
)

const (
	dbNotInitializedMsg = "database not initialized"
)

// GenerateAPIKey generates a new API key for agent authentication
func GenerateAPIKey(name string, hostID *string) (string, error) {
	if database.DB == nil {
		return "", errors.New(dbNotInitializedMsg)
	}

	// Generate a random API key
	apiKey := generateRandomKey()

	// Convert hostID string to UUID if provided
	var hostUUID *uuid.UUID
	if hostID != nil && *hostID != "" {
		if parsedUUID, err := uuid.Parse(*hostID); err == nil {
			hostUUID = &parsedUUID
		}
	}

	// Create API key record
	hashedKey, err := HashPassword(apiKey)
	if err != nil {
		return "", fmt.Errorf("failed to hash API key: %w", err)
	}
	apiKeyRecord := database.APIKey{
		KeyHash:  hashedKey, // Store securely hashed
		Name:     name,
		HostID:   hostUUID,
		IsActive: true,
	}

	// Save to database
	if err := database.DB.Create(&apiKeyRecord).Error; err != nil {
		return "", fmt.Errorf("failed to create API key: %w", err)
	}

	// Mask API key for logging: show prefix, last 4 chars, and total length
	if len(apiKey) > 12 {
		prefix := apiKey[:8]
		suffix := apiKey[len(apiKey)-4:]
		logrus.Infof("Generated API key for %s: %s...%s (len=%d)", name, prefix, suffix, len(apiKey))
	} else {
		logrus.Infof("Generated API key for %s: <masked - len=%d>", name, len(apiKey))
	}
	return apiKey, nil
}

// ValidateAPIKey validates an API key and returns the associated record
func ValidateAPIKey(apiKey string) (*database.APIKey, error) {
	if database.DB == nil {
		return nil, errors.New(dbNotInitializedMsg)
	}

	// Parse the API key format: FLA_prefix_secret
	parts := strings.Split(apiKey, "_")
	if len(parts) != 3 || parts[0] != "FLA" {
		return nil, fmt.Errorf("invalid API key format")
	}

	prefix := parts[1]
	secret := parts[2]

	// Find API key by prefix
	var apiKeyRecord database.APIKey
	result := database.DB.Where("prefix = ? AND is_active = ?", prefix, true).First(&apiKeyRecord)
	if result.Error != nil {
		return nil, fmt.Errorf("invalid API key")
	}

	// Verify the secret against the stored hash
	ok, err := VerifyPassword(secret, apiKeyRecord.KeyHash)
	if err != nil || !ok {
		return nil, fmt.Errorf("invalid API key")
	}

	// Update last used timestamp
	now := time.Now()
	database.DB.Model(&apiKeyRecord).Update("last_used", &now)

	return &apiKeyRecord, nil
}

// RevokeAPIKey revokes an API key by setting it as inactive (legacy function)
func RevokeAPIKey(apiKey string) error {
	if database.DB == nil {
		return errors.New(dbNotInitializedMsg)
	}

	// Parse the API key format: FLA_prefix_secret
	parts := strings.Split(apiKey, "_")
	if len(parts) != 3 || parts[0] != "FLA" {
		return fmt.Errorf("invalid API key format")
	}

	prefix := parts[1]

	result := database.DB.Model(&database.APIKey{}).
		Where("prefix = ?", prefix).
		Update("is_active", false)

	if result.Error != nil {
		return fmt.Errorf("failed to revoke API key: %w", result.Error)
	}

	logrus.Infof("Revoked API key: %s", apiKey)
	return nil
}

// ListAPIKeys returns all API keys
func ListAPIKeys() ([]database.APIKey, error) {
	if database.DB == nil {
		return nil, errors.New(dbNotInitializedMsg)
	}

	var apiKeys []database.APIKey
	result := database.DB.Find(&apiKeys)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to list API keys: %w", result.Error)
	}

	return apiKeys, nil
}

// generateRandomKey generates a random 32-byte key
func generateRandomKey() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}
