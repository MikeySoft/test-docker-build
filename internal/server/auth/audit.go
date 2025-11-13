package auth

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/mikeysoft/flotilla/internal/server/database"
	"github.com/sirupsen/logrus"
)

// LogAuditEvent logs an audit event
func LogAuditEvent(userID *uuid.UUID, action, entityType string, entityID *uuid.UUID, details interface{}, ipAddress, userAgent string) error {
	if database.DB == nil {
		return nil // Skip logging if database is not available
	}

	var detailsJSON database.JSONB
	if details != nil {
		// Convert to JSONB by unmarshaling into map
		jsonBytes, err := json.Marshal(details)
		if err != nil {
			logrus.Errorf("Failed to marshal audit details: %v", err)
			return err
		}
		if err := json.Unmarshal(jsonBytes, &detailsJSON); err != nil {
			logrus.Errorf("Failed to unmarshal audit details: %v", err)
			return err
		}
	}

	var targetID *string
	if entityID != nil {
		s := entityID.String()
		targetID = &s
	}

	auditLog := database.AuditLog{
		ActorUserID: userID,
		Action:      action,
		TargetType:  &entityType,
		TargetID:    targetID,
		Metadata:    detailsJSON,
		IP:          &ipAddress,
		UserAgent:   &userAgent,
		CreatedAt:   time.Now(),
	}

	if err := database.DB.Create(&auditLog).Error; err != nil {
		logrus.Errorf("Failed to create audit log: %v", err)
		return err
	}

	return nil
}
