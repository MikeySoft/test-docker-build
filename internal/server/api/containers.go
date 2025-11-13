package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mikeysoft/flotilla/internal/server/database"
	appLogs "github.com/mikeysoft/flotilla/internal/server/logs"
	"github.com/mikeysoft/flotilla/internal/server/topology"
	"github.com/mikeysoft/flotilla/internal/server/websocket"
	"github.com/mikeysoft/flotilla/internal/shared/protocol"
	"github.com/mikeysoft/flotilla/internal/shared/querydsl"
	"github.com/sirupsen/logrus"
)

// ContainersHandler handles container-related API endpoints
type ContainersHandler struct {
	hub      *websocket.Hub
	logs     *appLogs.Manager
	topology *topology.Manager
}

// NewContainersHandler creates a new containers handler
func NewContainersHandler(hub *websocket.Hub, logs *appLogs.Manager, topologyManager *topology.Manager) *ContainersHandler {
	return &ContainersHandler{
		hub:      hub,
		logs:     logs,
		topology: topologyManager,
	}
}

func (h *ContainersHandler) addLog(level, source, message string, fields map[string]any) {
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

func toStringSlice(value any) []string {
	switch v := value.(type) {
	case []string:
		return v
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func decodeRemovalConflicts(value any) []protocol.ResourceRemovalConflict {
	if value == nil {
		return nil
	}

	bytes, err := json.Marshal(value)
	if err != nil {
		return nil
	}

	var conflicts []protocol.ResourceRemovalConflict
	if err := json.Unmarshal(bytes, &conflicts); err != nil {
		return nil
	}

	return conflicts
}

func decodeRemovalErrors(value any) []protocol.ResourceRemovalError {
	if value == nil {
		return nil
	}

	bytes, err := json.Marshal(value)
	if err != nil {
		return nil
	}

	var removalErrors []protocol.ResourceRemovalError
	if err := json.Unmarshal(bytes, &removalErrors); err != nil {
		return nil
	}

	return removalErrors
}

// GetContainer returns details about a specific container
func (h *ContainersHandler) GetContainer(c *gin.Context) {
	hostID := c.Param("id")
	containerID := c.Param("container_id")

	// Check if host exists
	var host database.Host
	if err := database.DB.Where("id = ?", hostID).First(&host).Error; err != nil {
		logrus.Errorf("Host %s not found: %v", hostID, err)
		h.addLog("warn", "container", "Attempted to fetch container from unknown host", map[string]any{
			"host_id":      hostID,
			"container_id": containerID,
		})
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Host not found",
		})
		return
	}

	// Check if agent is connected
	agent, exists := h.hub.GetAgent(hostID)
	if !exists {
		h.addLog("error", "container", "Agent not connected while fetching container", map[string]any{
			"host_id":      host.ID.String(),
			"host_name":    host.Name,
			"container_id": containerID,
		})
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Host agent not connected",
		})
		return
	}

	// Send command to agent
	command := protocol.NewCommandWithAction("get_container", map[string]any{
		"container_id": containerID,
	})

	// Send command and wait for response
	response, err := h.sendCommandAndWait(agent.ID, command, 30*time.Second)
	if err != nil {
		logrus.Errorf("Failed to get container %s from host %s: %v", containerID, hostID, err)
		h.addLog("error", "container", "Failed to fetch container", map[string]any{
			"host_id":      host.ID.String(),
			"host_name":    host.Name,
			"container_id": containerID,
			"error":        err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve container",
		})
		return
	}

	h.addLog("info", "container", "Fetched container details", map[string]any{
		"host_id":      host.ID.String(),
		"host_name":    host.Name,
		"container_id": containerID,
	})
	c.JSON(http.StatusOK, response)
}

// GetContainerLogs returns logs from a specific container
func (h *ContainersHandler) GetContainerLogs(c *gin.Context) {
	hostID := c.Param("id")
	containerID := c.Param("container_id")

	// Check if host exists
	var host database.Host
	if err := database.DB.Where("id = ?", hostID).First(&host).Error; err != nil {
		logrus.Errorf("Host %s not found: %v", hostID, err)
		h.addLog("warn", "container", "Attempted to fetch container logs from unknown host", map[string]any{
			"host_id":      hostID,
			"container_id": containerID,
		})
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Host not found",
		})
		return
	}

	// Check if agent is connected
	agent, exists := h.hub.GetAgent(hostID)
	if !exists {
		h.addLog("error", "container", "Agent not connected while fetching container logs", map[string]any{
			"host_id":      host.ID.String(),
			"host_name":    host.Name,
			"container_id": containerID,
		})
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Host agent not connected",
		})
		return
	}

	// Parse query parameters
	params := map[string]any{
		"container_id": containerID,
	}

	if follow := c.Query("follow"); follow == "true" {
		params["follow"] = true
	}
	if tail := c.Query("tail"); tail != "" {
		params["tail"] = tail
	}
	if timestamps := c.Query("timestamps"); timestamps == "true" {
		params["timestamps"] = true
	}

	// Send command to agent
	command := protocol.NewCommandWithAction("get_container_logs", params)

	// Send command and wait for response
	response, err := h.sendCommandAndWait(agent.ID, command, 30*time.Second)
	if err != nil {
		logrus.Errorf("Failed to get logs for container %s from host %s: %v", containerID, hostID, err)
		h.addLog("error", "container", "Failed to fetch container logs", map[string]any{
			"host_id":      host.ID.String(),
			"host_name":    host.Name,
			"container_id": containerID,
			"error":        err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve container logs",
		})
		return
	}

	h.addLog("info", "container", "Fetched container logs", map[string]any{
		"host_id":      host.ID.String(),
		"host_name":    host.Name,
		"container_id": containerID,
	})
	c.JSON(http.StatusOK, response)
}

// GetContainerStats returns statistics for a specific container
func (h *ContainersHandler) GetContainerStats(c *gin.Context) {
	hostID := c.Param("id")
	containerID := c.Param("container_id")

	// Check if host exists
	var host database.Host
	if err := database.DB.Where("id = ?", hostID).First(&host).Error; err != nil {
		logrus.Errorf("Host %s not found: %v", hostID, err)
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Host not found",
		})
		return
	}

	// Check if agent is connected
	agent, exists := h.hub.GetAgent(hostID)
	if !exists {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Host agent not connected",
		})
		return
	}

	// Send command to agent
	command := protocol.NewCommandWithAction("get_container_stats", map[string]any{
		"container_id": containerID,
	})

	// Send command and wait for response
	response, err := h.sendCommandAndWait(agent.ID, command, 30*time.Second)
	if err != nil {
		logrus.Errorf("Failed to get stats for container %s from host %s: %v", containerID, hostID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve container stats",
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// ListImages returns images for a specific host
func (h *ContainersHandler) ListImages(c *gin.Context) {
	hostID := c.Param("id")

	// Check if host exists
	var host database.Host
	if err := database.DB.Where("id = ?", hostID).First(&host).Error; err != nil {
		logrus.Errorf("Host %s not found: %v", hostID, err)
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Host not found",
		})
		return
	}

	// Check if agent is connected
	agent, exists := h.hub.GetAgent(hostID)
	if !exists {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Host agent not connected",
		})
		return
	}

	// Send command to agent
	command := protocol.NewCommandWithAction("list_images", map[string]any{})

	// Send command and wait for response
	response, err := h.sendCommandAndWait(agent.ID, command, 30*time.Second)
	if err != nil {
		logrus.Errorf("Failed to get images from host %s: %v", hostID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve images",
		})
		return
	}

	images, ok := response["images"].([]interface{})
	if !ok {
		logrus.Errorf("Invalid images response format from host %s", hostID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Invalid response format from agent",
		})
		return
	}

	for i := range images {
		if m, ok := images[i].(map[string]any); ok {
			m["host_name"] = host.Name
		}
	}

	q := strings.TrimSpace(c.Query("q"))
	if q != "" {
		ast, err := querydsl.Parse(q)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid query"})
			return
		}
		filtered := make([]map[string]any, 0, len(images))
		for _, it := range images {
			if m, ok := it.(map[string]any); ok {
				if querydsl.EvaluateRecord(ast, m) {
					filtered = append(filtered, m)
				}
			}
		}
		out := make([]interface{}, len(filtered))
		for i := range filtered {
			out[i] = filtered[i]
		}
		c.JSON(http.StatusOK, out)
		return
	}

	c.JSON(http.StatusOK, images)
}

// RemoveImages removes one or more images from a host
func (h *ContainersHandler) RemoveImages(c *gin.Context) {
	hostID := c.Param("id")

	var host database.Host
	if err := database.DB.Where("id = ?", hostID).First(&host).Error; err != nil {
		logrus.Errorf("Host %s not found: %v", hostID, err)
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Host not found",
		})
		return
	}

	agent, exists := h.hub.GetAgent(hostID)
	if !exists {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Host agent not connected",
		})
		return
	}

	var request struct {
		Images []string `json:"images"`
		Force  bool     `json:"force"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	if len(request.Images) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "images array must not be empty"})
		return
	}

	params := map[string]any{
		"images": request.Images,
	}
	if request.Force {
		params["force"] = true
	}

	command := protocol.NewCommandWithAction("remove_images", params)
	response, err := h.sendCommandAndWait(agent.ID, command, 60*time.Second)
	if err != nil {
		logrus.Errorf("Failed to remove images on host %s: %v", hostID, err)
		h.addLog("error", "images", "Failed to remove images", map[string]any{
			"host_id": hostID,
			"images":  request.Images,
			"error":   err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove images"})
		return
	}

	removed := toStringSlice(response["removed"])
	conflicts := decodeRemovalConflicts(response["conflicts"])
	errors := decodeRemovalErrors(response["errors"])

	for _, imageID := range removed {
		h.addLog("info", "images", "Removed Docker image", map[string]any{
			"host_id": hostID,
			"image":   imageID,
			"force":   request.Force,
		})
	}

	for _, conflict := range conflicts {
		imageRef := conflict.ResourceName
		if imageRef == "" {
			imageRef = conflict.ResourceID
		}
		h.addLog("warn", "images", "Image removal conflict", map[string]any{
			"host_id":         hostID,
			"image":           imageRef,
			"resource_id":     conflict.ResourceID,
			"reason":          conflict.Reason,
			"force_supported": conflict.ForceSupported,
			"blockers":        conflict.Blockers,
		})
	}

	for _, removalErr := range errors {
		imageRef := removalErr.ResourceName
		if imageRef == "" {
			imageRef = removalErr.ResourceID
		}
		h.addLog("error", "images", "Image removal failed", map[string]any{
			"host_id":     hostID,
			"image":       imageRef,
			"resource_id": removalErr.ResourceID,
			"error":       removalErr.Message,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"removed":   removed,
		"conflicts": conflicts,
		"errors":    errors,
	})
}

// PruneDanglingImages removes all dangling images from a host
func (h *ContainersHandler) PruneDanglingImages(c *gin.Context) {
	hostID := c.Param("id")

	var host database.Host
	if err := database.DB.Where("id = ?", hostID).First(&host).Error; err != nil {
		logrus.Errorf("Host %s not found: %v", hostID, err)
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Host not found",
		})
		return
	}

	agent, exists := h.hub.GetAgent(hostID)
	if !exists {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Host agent not connected",
		})
		return
	}

	command := protocol.NewCommandWithAction("prune_dangling_images", map[string]any{})
	response, err := h.sendCommandAndWait(agent.ID, command, 120*time.Second)
	if err != nil {
		logrus.Errorf("Failed to prune dangling images on host %s: %v", hostID, err)
		h.addLog("error", "images", "Failed to prune dangling images", map[string]any{
			"host_id": hostID,
			"error":   err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prune dangling images"})
		return
	}

	removed := toStringSlice(response["removed"])
	spaceReclaimed := response["space_reclaimed"]
	h.addLog("info", "images", "Pruned dangling images", map[string]any{
		"host_id":         hostID,
		"removed_count":   len(removed),
		"space_reclaimed": spaceReclaimed,
	})

	c.JSON(http.StatusOK, gin.H{
		"removed":         removed,
		"space_reclaimed": spaceReclaimed,
	})
}

// ListNetworks returns networks for a specific host
func (h *ContainersHandler) ListNetworks(c *gin.Context) {
	hostID := c.Param("id")

	var host database.Host
	if err := database.DB.Where("id = ?", hostID).First(&host).Error; err != nil {
		logrus.Errorf("Host %s not found: %v", hostID, err)
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Host not found",
		})
		return
	}

	agent, exists := h.hub.GetAgent(hostID)
	if !exists {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Host agent not connected",
		})
		return
	}

	command := protocol.NewCommandWithAction("list_networks", map[string]any{})
	response, err := h.sendCommandAndWait(agent.ID, command, 30*time.Second)
	if err != nil {
		logrus.Errorf("Failed to get networks from host %s: %v", hostID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve networks",
		})
		return
	}

	networks, ok := response["networks"].([]interface{})
	if !ok {
		logrus.Errorf("Invalid networks response format from host %s", hostID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Invalid response format from agent",
		})
		return
	}

	for i := range networks {
		if m, ok := networks[i].(map[string]any); ok {
			m["host_name"] = host.Name
		}
	}

	h.applyNetworkTopology(host.ID.String(), networks)

	q := strings.TrimSpace(c.Query("q"))
	if q != "" {
		ast, err := querydsl.Parse(q)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid query"})
			return
		}
		filtered := make([]map[string]any, 0, len(networks))
		for _, it := range networks {
			if m, ok := it.(map[string]any); ok {
				if querydsl.EvaluateRecord(ast, m) {
					filtered = append(filtered, m)
				}
			}
		}
		out := make([]interface{}, len(filtered))
		for i := range filtered {
			out[i] = filtered[i]
		}
		c.JSON(http.StatusOK, out)
		return
	}

	c.JSON(http.StatusOK, networks)
}

// InspectNetwork returns detailed information about a specific network.
func (h *ContainersHandler) InspectNetwork(c *gin.Context) {
	hostID := c.Param("id")
	networkID := c.Param("network_id")

	var host database.Host
	if err := database.DB.Where("id = ?", hostID).First(&host).Error; err != nil {
		logrus.Errorf("Host %s not found: %v", hostID, err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Host not found"})
		return
	}

	agent, exists := h.hub.GetAgent(hostID)
	if !exists {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Host agent not connected"})
		return
	}

	command := protocol.NewCommandWithAction("inspect_networks", map[string]any{
		"ids": []string{networkID},
	})
	response, err := h.sendCommandAndWait(agent.ID, command, 30*time.Second)
	if err != nil {
		logrus.Errorf("Failed to inspect network %s on host %s: %v", networkID, hostID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to inspect network"})
		return
	}

	if errorsField, ok := response["errors"].([]interface{}); ok && len(errorsField) > 0 {
		for _, item := range errorsField {
			if errMap, ok := item.(map[string]any); ok {
				if id, ok := errMap["id"].(string); ok && id == networkID {
					c.JSON(http.StatusNotFound, gin.H{"error": errMap["error"]})
					return
				}
			}
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to inspect network"})
		return
	}

	networks, ok := response["networks"].([]interface{})
	if !ok || len(networks) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Network not found"})
		return
	}

	if payload, ok := networks[0].(map[string]any); ok && payload != nil {
		h.addLog("info", "network", "Inspected Docker network", map[string]any{
			"host_id":    host.ID.String(),
			"host_name":  host.Name,
			"network_id": networkID,
		})
		c.JSON(http.StatusOK, payload)
		return
	}

	c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid response format from agent"})
}

// RemoveNetwork removes a specific network from a host.
func (h *ContainersHandler) RemoveNetwork(c *gin.Context) {
	hostID := c.Param("id")
	networkID := c.Param("network_id")

	var host database.Host
	if err := database.DB.Where("id = ?", hostID).First(&host).Error; err != nil {
		logrus.Errorf("Host %s not found: %v", hostID, err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Host not found"})
		return
	}

	agent, exists := h.hub.GetAgent(hostID)
	if !exists {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Host agent not connected"})
		return
	}

	force := false
	if val := strings.ToLower(strings.TrimSpace(c.Query("force"))); val == "true" || val == "1" || val == "yes" {
		force = true
	}

	params := map[string]any{
		"ids": []string{networkID},
	}
	if force {
		params["force"] = true
	}

	command := protocol.NewCommandWithAction("remove_networks", params)
	response, err := h.sendCommandAndWait(agent.ID, command, 60*time.Second)
	if err != nil {
		logrus.Errorf("Failed to remove network %s on host %s: %v", networkID, hostID, err)
		h.addLog("error", "network", "Failed to remove Docker network", map[string]any{
			"host_id":    host.ID.String(),
			"host_name":  host.Name,
			"network_id": networkID,
			"error":      err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove network"})
		return
	}

	removed := toStringSlice(response["removed"])
	conflicts := decodeRemovalConflicts(response["conflicts"])
	errors := decodeRemovalErrors(response["errors"])

	for _, conflict := range conflicts {
		h.addLog("warn", "network", "Network removal conflict", map[string]any{
			"host_id":         host.ID.String(),
			"host_name":       host.Name,
			"network_id":      conflict.ResourceID,
			"requested_id":    networkID,
			"reason":          conflict.Reason,
			"force_supported": conflict.ForceSupported,
			"blockers":        conflict.Blockers,
		})
		if conflict.ResourceID == networkID || conflict.ResourceName == networkID {
			c.JSON(http.StatusConflict, gin.H{
				"error":    conflict.Reason,
				"conflict": conflict,
			})
			return
		}
	}

	for _, removalErr := range errors {
		h.addLog("error", "network", "Network removal failed", map[string]any{
			"host_id":     host.ID.String(),
			"host_name":   host.Name,
			"network_id":  networkID,
			"resource_id": removalErr.ResourceID,
			"error":       removalErr.Message,
		})
		if removalErr.ResourceID == networkID || removalErr.ResourceName == networkID {
			c.JSON(http.StatusInternalServerError, gin.H{"error": removalErr.Message})
			return
		}
	}

	if len(removed) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Network removal not confirmed"})
		return
	}

	for _, network := range removed {
		h.addLog("info", "network", "Removed Docker network", map[string]any{
			"host_id":    host.ID.String(),
			"host_name":  host.Name,
			"network_id": network,
			"force":      force,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"removed":   removed,
		"conflicts": conflicts,
		"errors":    errors,
	})
}

// ListVolumes returns volumes for a specific host
func (h *ContainersHandler) ListVolumes(c *gin.Context) {
	hostID := c.Param("id")

	var host database.Host
	if err := database.DB.Where("id = ?", hostID).First(&host).Error; err != nil {
		logrus.Errorf("Host %s not found: %v", hostID, err)
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Host not found",
		})
		return
	}

	agent, exists := h.hub.GetAgent(hostID)
	if !exists {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Host agent not connected",
		})
		return
	}

	command := protocol.NewCommandWithAction("list_volumes", map[string]any{})
	response, err := h.sendCommandAndWait(agent.ID, command, 30*time.Second)
	if err != nil {
		logrus.Errorf("Failed to get volumes from host %s: %v", hostID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve volumes",
		})
		return
	}

	volumes, ok := response["volumes"].([]interface{})
	if !ok {
		logrus.Errorf("Invalid volumes response format from host %s", hostID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Invalid response format from agent",
		})
		return
	}

	for i := range volumes {
		if m, ok := volumes[i].(map[string]any); ok {
			m["host_name"] = host.Name
		}
	}

	h.applyVolumeTopology(host.ID.String(), volumes)

	q := strings.TrimSpace(c.Query("q"))
	if q != "" {
		ast, err := querydsl.Parse(q)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid query"})
			return
		}
		filtered := make([]map[string]any, 0, len(volumes))
		for _, it := range volumes {
			if m, ok := it.(map[string]any); ok {
				if querydsl.EvaluateRecord(ast, m) {
					filtered = append(filtered, m)
				}
			}
		}
		out := make([]interface{}, len(filtered))
		for i := range filtered {
			out[i] = filtered[i]
		}
		c.JSON(http.StatusOK, out)
		return
	}

	c.JSON(http.StatusOK, volumes)
}

// InspectVolume returns detailed information about a specific volume.
func (h *ContainersHandler) InspectVolume(c *gin.Context) {
	hostID := c.Param("id")
	volumeName := c.Param("volume_name")

	var host database.Host
	if err := database.DB.Where("id = ?", hostID).First(&host).Error; err != nil {
		logrus.Errorf("Host %s not found: %v", hostID, err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Host not found"})
		return
	}

	agent, exists := h.hub.GetAgent(hostID)
	if !exists {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Host agent not connected"})
		return
	}

	command := protocol.NewCommandWithAction("inspect_volumes", map[string]any{
		"names": []string{volumeName},
	})
	response, err := h.sendCommandAndWait(agent.ID, command, 30*time.Second)
	if err != nil {
		logrus.Errorf("Failed to inspect volume %s on host %s: %v", volumeName, hostID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to inspect volume"})
		return
	}

	if errorsField, ok := response["errors"].([]interface{}); ok && len(errorsField) > 0 {
		for _, item := range errorsField {
			if errMap, ok := item.(map[string]any); ok {
				if name, ok := errMap["name"].(string); ok && name == volumeName {
					c.JSON(http.StatusNotFound, gin.H{"error": errMap["error"]})
					return
				}
			}
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to inspect volume"})
		return
	}

	volumes, ok := response["volumes"].([]interface{})
	if !ok || len(volumes) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Volume not found"})
		return
	}

	if payload, ok := volumes[0].(map[string]any); ok && payload != nil {
		h.addLog("info", "volume", "Inspected Docker volume", map[string]any{
			"host_id":     host.ID.String(),
			"host_name":   host.Name,
			"volume_name": volumeName,
		})
		c.JSON(http.StatusOK, payload)
		return
	}

	c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid response format from agent"})
}

// RemoveVolume removes a specific volume from a host.
func (h *ContainersHandler) RemoveVolume(c *gin.Context) {
	hostID := c.Param("id")
	volumeName := c.Param("volume_name")

	var host database.Host
	if err := database.DB.Where("id = ?", hostID).First(&host).Error; err != nil {
		logrus.Errorf("Host %s not found: %v", hostID, err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Host not found"})
		return
	}

	agent, exists := h.hub.GetAgent(hostID)
	if !exists {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Host agent not connected"})
		return
	}

	force := false
	if val := strings.ToLower(strings.TrimSpace(c.Query("force"))); val == "true" || val == "1" || val == "yes" {
		force = true
	}

	params := map[string]any{
		"names": []string{volumeName},
	}
	if force {
		params["force"] = true
	}

	command := protocol.NewCommandWithAction("remove_volumes", params)
	response, err := h.sendCommandAndWait(agent.ID, command, 60*time.Second)
	if err != nil {
		logrus.Errorf("Failed to remove volume %s on host %s: %v", volumeName, hostID, err)
		h.addLog("error", "volume", "Failed to remove Docker volume", map[string]any{
			"host_id":     host.ID.String(),
			"host_name":   host.Name,
			"volume_name": volumeName,
			"error":       err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove volume"})
		return
	}

	removed := toStringSlice(response["removed"])
	conflicts := decodeRemovalConflicts(response["conflicts"])
	errors := decodeRemovalErrors(response["errors"])

	for _, conflict := range conflicts {
		h.addLog("warn", "volume", "Volume removal conflict", map[string]any{
			"host_id":     host.ID.String(),
			"host_name":   host.Name,
			"volume_name": conflict.ResourceName,
			"resource_id": conflict.ResourceID,
			"reason":      conflict.Reason,
			"blockers":    conflict.Blockers,
		})
		if conflict.ResourceID == volumeName || conflict.ResourceName == volumeName {
			c.JSON(http.StatusConflict, gin.H{
				"error":    conflict.Reason,
				"conflict": conflict,
			})
			return
		}
	}

	for _, removalErr := range errors {
		h.addLog("error", "volume", "Volume removal failed", map[string]any{
			"host_id":     host.ID.String(),
			"host_name":   host.Name,
			"volume_name": volumeName,
			"resource_id": removalErr.ResourceID,
			"error":       removalErr.Message,
		})
		if removalErr.ResourceID == volumeName || removalErr.ResourceName == volumeName {
			c.JSON(http.StatusInternalServerError, gin.H{"error": removalErr.Message})
			return
		}
	}

	if len(removed) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Volume removal not confirmed"})
		return
	}

	for _, vol := range removed {
		h.addLog("info", "volume", "Removed Docker volume", map[string]any{
			"host_id":     host.ID.String(),
			"host_name":   host.Name,
			"volume_name": vol,
			"force":       force,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"removed":   removed,
		"conflicts": conflicts,
		"errors":    errors,
	})
}

// RefreshNetworks triggers a background refresh of network topology for a host.
func (h *ContainersHandler) RefreshNetworks(c *gin.Context) {
	if h.topology == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "topology caching is not enabled"})
		return
	}

	hostID := c.Param("id")
	var host database.Host
	if err := database.DB.Where("id = ?", hostID).First(&host).Error; err != nil {
		logrus.Errorf("Host %s not found: %v", hostID, err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Host not found"})
		return
	}

	var req struct {
		IDs []string `json:"ids"`
	}
	if err := bindOptionalJSON(c, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 90*time.Second)
	defer cancel()

	if err := h.topology.RefreshNetworks(ctx, hostID, req.IDs); err != nil {
		logrus.WithError(err).WithField("host_id", hostID).Warn("failed to refresh networks topology")
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}

	topologyPayload, err := h.serializeNetworkTopology(hostID)
	if err != nil {
		logrus.WithError(err).WithField("host_id", hostID).Warn("failed to load refreshed network topology")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load refreshed topology"})
		return
	}

	h.addLog("info", "topology", "Network topology refreshed", map[string]any{
		"host_id": host.ID.String(),
		"count":   len(topologyPayload),
		"ids":     req.IDs,
	})

	c.JSON(http.StatusOK, gin.H{
		"status":    "refreshed",
		"host_id":   host.ID.String(),
		"topology":  topologyPayload,
		"refreshed": time.Now().UTC().Format(time.RFC3339),
		"requested": req.IDs,
	})
}

// RefreshVolumes triggers a background refresh of volume topology for a host.
func (h *ContainersHandler) RefreshVolumes(c *gin.Context) {
	if h.topology == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "topology caching is not enabled"})
		return
	}

	hostID := c.Param("id")
	var host database.Host
	if err := database.DB.Where("id = ?", hostID).First(&host).Error; err != nil {
		logrus.Errorf("Host %s not found: %v", hostID, err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Host not found"})
		return
	}

	var req struct {
		Names []string `json:"names"`
	}
	if err := bindOptionalJSON(c, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 90*time.Second)
	defer cancel()

	if err := h.topology.RefreshVolumes(ctx, hostID, req.Names); err != nil {
		logrus.WithError(err).WithField("host_id", hostID).Warn("failed to refresh volume topology")
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}

	topologyPayload, err := h.serializeVolumeTopology(hostID)
	if err != nil {
		logrus.WithError(err).WithField("host_id", hostID).Warn("failed to load refreshed volume topology")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load refreshed topology"})
		return
	}

	h.addLog("info", "topology", "Volume topology refreshed", map[string]any{
		"host_id": host.ID.String(),
		"count":   len(topologyPayload),
		"names":   req.Names,
	})

	c.JSON(http.StatusOK, gin.H{
		"status":    "refreshed",
		"host_id":   host.ID.String(),
		"topology":  topologyPayload,
		"refreshed": time.Now().UTC().Format(time.RFC3339),
		"requested": req.Names,
	})
}

// sendCommandAndWait sends a command to an agent and waits for the response
func (h *ContainersHandler) sendCommandAndWait(agentID string, command *protocol.Message, timeout time.Duration) (map[string]any, error) {
	responseCh := h.hub.SubscribeResponse(command.ID)
	defer h.hub.UnsubscribeResponse(command.ID)

	// Send command
	if err := h.hub.SendCommand(agentID, command); err != nil {
		return nil, err
	}

	// Wait for response
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case response := <-responseCh:
			if response == nil || response.AgentID != agentID {
				continue
			}
			if response.Error != nil {
				return nil, response.Error
			}

			if response.Response != nil {
				if responseData, ok := response.Response.Payload["data"].(map[string]any); ok {
					return responseData, nil
				}
				return response.Response.Payload, nil
			}

			return map[string]any{"message": "Command completed"}, nil
		case <-timer.C:
			return nil, protocol.ErrCommandTimeout
		}
	}
}

func (h *ContainersHandler) applyNetworkTopology(hostID string, resources []interface{}) {
	if h.topology == nil || len(resources) == 0 {
		return
	}
	records, err := h.topology.GetNetworkTopology(hostID)
	if err != nil {
		logrus.WithError(err).WithField("host_id", hostID).Warn("failed to load cached network topology")
		return
	}
	for _, item := range resources {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		m["topology_metadata_pending"] = true
		id, _ := m["id"].(string)
		if id == "" {
			continue
		}
		record, ok := records[id]
		if !ok {
			continue
		}
		snapshot := cloneJSONBMap(record.Snapshot)
		applyNetworkSnapshot(m, snapshot)
		m["topology_snapshot"] = snapshot
		m["topology_refreshed_at"] = record.RefreshedAt.Format(time.RFC3339)
		m["topology_metadata_pending"] = false
		m["topology_is_stale"] = h.topology.IsStale(record.RefreshedAt)
	}
}

func (h *ContainersHandler) applyVolumeTopology(hostID string, resources []interface{}) {
	if h.topology == nil || len(resources) == 0 {
		return
	}
	records, err := h.topology.GetVolumeTopology(hostID)
	if err != nil {
		logrus.WithError(err).WithField("host_id", hostID).Warn("failed to load cached volume topology")
		return
	}
	for _, item := range resources {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		m["topology_metadata_pending"] = true
		name, _ := m["name"].(string)
		if name == "" {
			continue
		}
		record, ok := records[name]
		if !ok {
			continue
		}
		snapshot := cloneJSONBMap(record.Snapshot)
		applyVolumeSnapshot(m, snapshot)
		m["topology_snapshot"] = snapshot
		m["topology_refreshed_at"] = record.RefreshedAt.Format(time.RFC3339)
		m["topology_metadata_pending"] = false
		m["topology_is_stale"] = h.topology.IsStale(record.RefreshedAt)
	}
}

func (h *ContainersHandler) serializeNetworkTopology(hostID string) (map[string]any, error) {
	if h.topology == nil {
		return nil, nil
	}
	records, err := h.topology.GetNetworkTopology(hostID)
	if err != nil {
		return nil, err
	}
	result := make(map[string]any, len(records))
	for id, rec := range records {
		result[id] = map[string]any{
			"snapshot":      cloneJSONBMap(rec.Snapshot),
			"refreshed_at":  rec.RefreshedAt.Format(time.RFC3339),
			"is_stale":      h.topology.IsStale(rec.RefreshedAt),
			"host_id":       hostID,
			"resource_type": "network",
		}
	}
	return result, nil
}

func (h *ContainersHandler) serializeVolumeTopology(hostID string) (map[string]any, error) {
	if h.topology == nil {
		return nil, nil
	}
	records, err := h.topology.GetVolumeTopology(hostID)
	if err != nil {
		return nil, err
	}
	result := make(map[string]any, len(records))
	for name, rec := range records {
		result[name] = map[string]any{
			"snapshot":      cloneJSONBMap(rec.Snapshot),
			"refreshed_at":  rec.RefreshedAt.Format(time.RFC3339),
			"is_stale":      h.topology.IsStale(rec.RefreshedAt),
			"host_id":       hostID,
			"resource_type": "volume",
		}
	}
	return result, nil
}

func applyNetworkSnapshot(target map[string]any, snapshot map[string]any) {
	if snapshot == nil {
		return
	}
	if attachments, ok := snapshot["containers_detail"]; ok {
		target["containers_detail"] = attachments
		if _, exists := target["containers"]; !exists {
			target["containers"] = extractSliceLength(attachments)
		}
	}
	if stacks, ok := snapshot["stacks"]; ok {
		target["stacks"] = stacks
	}
	if connected, ok := snapshot["connected"]; ok {
		target["connected"] = connected
	}
}

func applyVolumeSnapshot(target map[string]any, snapshot map[string]any) {
	if snapshot == nil {
		return
	}
	if consumers, ok := snapshot["containers_detail"]; ok {
		target["containers_detail"] = consumers
		if _, exists := target["containers"]; !exists {
			target["containers"] = extractSliceLength(consumers)
		}
	}
	if stacks, ok := snapshot["stacks"]; ok {
		target["stacks"] = stacks
	}
	if refCount, ok := snapshot["ref_count"]; ok {
		target["ref_count"] = refCount
	}
}

func cloneJSONBMap(snapshot database.JSONB) map[string]any {
	if snapshot == nil {
		return nil
	}
	out := make(map[string]any, len(snapshot))
	for k, v := range snapshot {
		out[k] = v
	}
	return out
}

func extractSliceLength(value interface{}) int {
	switch v := value.(type) {
	case []interface{}:
		return len(v)
	case []map[string]any:
		return len(v)
	default:
		return 0
	}
}

func bindOptionalJSON(c *gin.Context, target interface{}) error {
	if c.Request.Body == nil {
		return nil
	}
	if c.Request.ContentLength == 0 {
		return nil
	}
	if err := c.ShouldBindJSON(target); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
	return nil
}
