package api

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/mikeysoft/flotilla/internal/server/dashboard"
	appLogs "github.com/mikeysoft/flotilla/internal/server/logs"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// DashboardHandler provides endpoints for fleet summary and actionable tasks.
type DashboardHandler struct {
	manager *dashboard.Manager
	logs    *appLogs.Manager
}

// NewDashboardHandler constructs a dashboard HTTP handler.
func NewDashboardHandler(manager *dashboard.Manager, logs *appLogs.Manager) *DashboardHandler {
	return &DashboardHandler{
		manager: manager,
		logs:    logs,
	}
}

// GetSummary returns the current dashboard summary.
func (h *DashboardHandler) GetSummary(c *gin.Context) {
	summary, err := h.manager.GetSummary(c.Request.Context())
	if err != nil {
		logrus.WithError(err).Error("failed to load dashboard summary")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load dashboard summary"})
		return
	}
	c.JSON(http.StatusOK, summary)
}

// ListTasks returns dashboard tasks filtered by query parameters.
func (h *DashboardHandler) ListTasks(c *gin.Context) {
	filter := dashboard.TaskFilter{
		Statuses:   splitAndNormalize(c.Query("status")),
		Severities: splitAndNormalize(c.Query("severity")),
		Sources:    splitAndNormalize(c.Query("source")),
	}

	if v := c.Query("limit"); v != "" {
		if limit, err := strconv.Atoi(v); err == nil {
			filter.Limit = limit
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be an integer"})
			return
		}
	}
	if v := c.Query("offset"); v != "" {
		if offset, err := strconv.Atoi(v); err == nil {
			filter.Offset = offset
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "offset must be an integer"})
			return
		}
	}

	tasks, total, err := h.manager.ListTasks(c.Request.Context(), filter)
	if err != nil {
		logrus.WithError(err).Error("failed to list dashboard tasks")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list dashboard tasks"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tasks": tasks,
		"total": total,
	})
}

type createTaskRequest struct {
	Title        string                 `json:"title" binding:"required"`
	Description  string                 `json:"description"`
	Severity     string                 `json:"severity"`
	Category     string                 `json:"category"`
	TaskType     string                 `json:"task_type"`
	Metadata     map[string]interface{} `json:"metadata"`
	HostID       string                 `json:"host_id"`
	StackID      string                 `json:"stack_id"`
	ContainerID  string                 `json:"container_id"`
	DueAt        *time.Time             `json:"due_at"`
	SnoozedUntil *time.Time             `json:"snoozed_until"`
}

// CreateTask adds a new manual task to the dashboard.
func (h *DashboardHandler) CreateTask(c *gin.Context) {
	var req createTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	ctx := c.Request.Context()
	userID := parseUserID(c)

	input := dashboard.ManualTaskInput{
		Title:        req.Title,
		Description:  req.Description,
		Severity:     req.Severity,
		Category:     req.Category,
		TaskType:     req.TaskType,
		Metadata:     req.Metadata,
		DueAt:        req.DueAt,
		SnoozedUntil: req.SnoozedUntil,
		CreatedBy:    userID,
	}

	if req.HostID != "" {
		hostID, err := uuid.Parse(req.HostID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid host_id"})
			return
		}
		input.HostID = &hostID
	}
	if req.StackID != "" {
		stackID, err := uuid.Parse(req.StackID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid stack_id"})
			return
		}
		input.StackID = &stackID
	}
	if req.ContainerID != "" {
		containerID := req.ContainerID
		input.ContainerID = &containerID
	}

	task, err := h.manager.CreateManualTask(ctx, input)
	if err != nil {
		logrus.WithError(err).Error("failed to create dashboard task")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.addLog("info", "dashboard", "Created manual dashboard task", map[string]any{
		"task_id": task.ID.String(),
		"title":   task.Title,
	})

	c.JSON(http.StatusCreated, task)
}

// UpdateTask modifies manual dashboard tasks.
func (h *DashboardHandler) UpdateTask(c *gin.Context) {
	taskID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task id"})
		return
	}

	var payload map[string]interface{}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	input, err := buildUpdateInput(payload)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	task, err := h.manager.UpdateTask(c.Request.Context(), taskID, input)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.addLog("info", "dashboard", "Updated dashboard task", map[string]any{
		"task_id": task.ID.String(),
	})

	c.JSON(http.StatusOK, task)
}

type statusRequest struct {
	Status string `json:"status" binding:"required"`
}

// UpdateTaskStatus transitions a dashboard task to a new status.
func (h *DashboardHandler) UpdateTaskStatus(c *gin.Context) {
	taskID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task id"})
		return
	}

	var req statusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	task, err := h.manager.UpdateTaskStatus(c.Request.Context(), taskID, req.Status, parseUserID(c))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.addLog("info", "dashboard", "Updated dashboard task status", map[string]any{
		"task_id": task.ID.String(),
		"status":  task.Status,
	})

	c.JSON(http.StatusOK, task)
}

func buildUpdateInput(payload map[string]interface{}) (dashboard.UpdateTaskInput, error) {
	var input dashboard.UpdateTaskInput

	if v, ok := payload["title"]; ok {
		if v == nil {
			return input, errors.New("title cannot be null")
		}
		str, ok := v.(string)
		if !ok {
			return input, errors.New("title must be a string")
		}
		str = strings.TrimSpace(str)
		input.Title = &str
	}

	if v, ok := payload["description"]; ok {
		if v == nil {
			empty := ""
			input.Description = &empty
		} else if str, ok := v.(string); ok {
			str = strings.TrimSpace(str)
			input.Description = &str
		} else {
			return input, errors.New("description must be a string")
		}
	}

	if v, ok := payload["severity"]; ok {
		if v == nil {
			return input, errors.New("severity cannot be null")
		}
		str, ok := v.(string)
		if !ok {
			return input, errors.New("severity must be a string")
		}
		str = strings.TrimSpace(str)
		input.Severity = &str
	}

	if v, ok := payload["category"]; ok {
		if v == nil {
			empty := ""
			input.Category = &empty
		} else if str, ok := v.(string); ok {
			str = strings.TrimSpace(str)
			input.Category = &str
		} else {
			return input, errors.New("category must be a string")
		}
	}

	if v, ok := payload["task_type"]; ok {
		if v == nil {
			empty := ""
			input.TaskType = &empty
		} else if str, ok := v.(string); ok {
			str = strings.TrimSpace(str)
			input.TaskType = &str
		} else {
			return input, errors.New("task_type must be a string")
		}
	}

	if v, ok := payload["metadata"]; ok {
		if v == nil {
			input.Metadata = map[string]interface{}{}
		} else if m, ok := v.(map[string]interface{}); ok {
			input.Metadata = m
		} else {
			return input, errors.New("metadata must be an object")
		}
	}

	if v, ok := payload["due_at"]; ok {
		input.DueAtSet = true
		if v == nil {
			input.DueAt = nil
		} else if str, ok := v.(string); ok {
			str = strings.TrimSpace(str)
			if str == "" {
				input.DueAt = nil
			} else {
				ts, err := time.Parse(time.RFC3339, str)
				if err != nil {
					return input, fmt.Errorf("invalid due_at value: %w", err)
				}
				input.DueAt = &ts
			}
		} else {
			return input, errors.New("due_at must be an RFC3339 string or null")
		}
	}

	if v, ok := payload["snoozed_until"]; ok {
		input.SnoozedUntilSet = true
		if v == nil {
			input.SnoozedUntil = nil
		} else if str, ok := v.(string); ok {
			str = strings.TrimSpace(str)
			if str == "" {
				input.SnoozedUntil = nil
			} else {
				ts, err := time.Parse(time.RFC3339, str)
				if err != nil {
					return input, fmt.Errorf("invalid snoozed_until value: %w", err)
				}
				input.SnoozedUntil = &ts
			}
		} else {
			return input, errors.New("snoozed_until must be an RFC3339 string or null")
		}
	}

	return input, nil
}

func splitAndNormalize(v string) []string {
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.ToLower(strings.TrimSpace(p))
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func parseUserID(c *gin.Context) *uuid.UUID {
	if raw, ok := c.Get("user_id"); ok {
		if str, ok := raw.(string); ok && str != "" {
			if id, err := uuid.Parse(str); err == nil {
				return &id
			}
		}
	}
	return nil
}

func (h *DashboardHandler) addLog(level, source, message string, fields map[string]any) {
	if h.logs == nil {
		return
	}

	entryFields := map[string]any{}
	for k, v := range fields {
		entryFields[k] = v
	}

	h.logs.Add(appLogs.Entry{
		Level:   level,
		Source:  source,
		Message: message,
		Fields:  entryFields,
	})
}
