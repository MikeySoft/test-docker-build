package api

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	gorillawebsocket "github.com/gorilla/websocket"
	"github.com/mikeysoft/flotilla/internal/server/auth"
	"github.com/mikeysoft/flotilla/internal/server/database"
	appLogs "github.com/mikeysoft/flotilla/internal/server/logs"
	"github.com/mikeysoft/flotilla/internal/server/topology"
	serverws "github.com/mikeysoft/flotilla/internal/server/websocket"
	sharedconfig "github.com/mikeysoft/flotilla/internal/shared/config"
	"github.com/mikeysoft/flotilla/internal/shared/protocol"
	"github.com/mikeysoft/flotilla/internal/shared/querydsl"
	"github.com/sirupsen/logrus"
)

const (
	hostIDQuery     = "id = ?"
	hostNotFoundMsg = "Host not found"
	hostNotFoundLog = "Host %s not found: %v"
)

// HostsHandler handles host-related API endpoints
type HostsHandler struct {
	hub      *serverws.Hub
	logs     *appLogs.Manager
	topology *topology.Manager
}

// NewHostsHandler creates a new hosts handler
func NewHostsHandler(hub *serverws.Hub, logs *appLogs.Manager, topologyManager *topology.Manager) *HostsHandler {
	return &HostsHandler{
		hub:      hub,
		logs:     logs,
		topology: topologyManager,
	}
}

func (h *HostsHandler) addLog(level, source, message string, fields map[string]any) {
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

// DeleteHost removes a host and associated data from the database.
// This will cascade-delete stacks due to GORM constraints and detach any API keys (SET NULL).
func (h *HostsHandler) DeleteHost(c *gin.Context) {
	hostID := c.Param("id")

	// Check if host exists
	var host database.Host
	if err := database.DB.Where(hostIDQuery, hostID).First(&host).Error; err != nil {
		logrus.Errorf(hostNotFoundLog, hostID, err)
		h.addLog("warn", "host", "Attempted to delete unknown host", map[string]any{
			"host_id": hostID,
		})
		c.JSON(http.StatusNotFound, gin.H{"error": hostNotFoundMsg})
		return
	}

	// If an agent is connected, mark as offline and close connection
	if agent, exists := h.hub.GetAgentByHost(hostID); exists {
		// Best-effort close: unregister will update status to offline
		go func(a *serverws.AgentConnection) {
			defer func() { recover() }()
			if err := a.Conn.Close(); err != nil && !errors.Is(err, gorillawebsocket.ErrCloseSent) {
				logrus.WithError(err).Debugf("Failed to close agent connection while deleting host %s", hostID)
			}
		}(agent)
	}

	// Delete host; stacks are CASCADE via model constraints
	if err := database.DB.Delete(&host).Error; err != nil {
		logrus.Errorf("Failed to delete host %s: %v", hostID, err)
		h.addLog("error", "host", "Failed to delete host", map[string]any{
			"host_id":   host.ID.String(),
			"host_name": host.Name,
			"error":     err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete host"})
		return
	}

	if h.topology != nil {
		if err := h.topology.PurgeHost(hostID); err != nil {
			logrus.WithError(err).WithField("host_id", hostID).Warn("failed to purge host topology cache")
		}
	}

	h.addLog("info", "host", "Deleted host", map[string]any{
		"host_id":   host.ID.String(),
		"host_name": host.Name,
	})
	c.Status(http.StatusNoContent)
}

// ListHosts returns a list of all hosts
func (h *HostsHandler) ListHosts(c *gin.Context) {
	var hosts []database.Host

	// Get hosts from database
	if err := database.DB.Find(&hosts).Error; err != nil {
		logrus.Errorf("Failed to list hosts: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve hosts",
		})
		return
	}

	// Update online status based on WebSocket connections
	agents := h.hub.GetAgents()
	for i := range hosts {
		if agent, exists := agents[hosts[i].ID.String()]; exists {
			hosts[i].Status = "online"
			hosts[i].LastSeen = &agent.LastSeen
		} else {
			hosts[i].Status = "offline"
		}
	}

	// Optional server-side filtering via q
	q := strings.TrimSpace(c.Query("q"))
	if q != "" {
		ast, err := querydsl.Parse(q)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid query"})
			return
		}
		filtered := make([]database.Host, 0, len(hosts))
		for _, host := range hosts {
			rec := map[string]any{
				"name":   host.Name,
				"status": host.Status,
				"host":   host.Name,
			}
			if querydsl.EvaluateRecord(ast, rec) {
				filtered = append(filtered, host)
			}
		}
		hosts = filtered
	}

	c.JSON(http.StatusOK, hosts)
}

// GetHost returns details about a specific host
func (h *HostsHandler) GetHost(c *gin.Context) {
	hostID := c.Param("id")

	var host database.Host
	if err := database.DB.Where(hostIDQuery, hostID).First(&host).Error; err != nil {
		logrus.Errorf("Failed to get host %s: %v", hostID, err)
		c.JSON(http.StatusNotFound, gin.H{
			"error": hostNotFoundMsg,
		})
		return
	}

	// Update online status based on WebSocket connection
	if agent, exists := h.hub.GetAgent(hostID); exists {
		host.Status = "online"
		host.LastSeen = &agent.LastSeen
	} else {
		host.Status = "offline"
	}

	c.JSON(http.StatusOK, host)
}

// GetHostInfo queries the agent for docker and host info
func (h *HostsHandler) GetHostInfo(c *gin.Context) {
	hostID := c.Param("id")

	// Ensure host exists
	var host database.Host
	if err := database.DB.Where(hostIDQuery, hostID).First(&host).Error; err != nil {
		logrus.Errorf(hostNotFoundLog, hostID, err)
		c.JSON(http.StatusNotFound, gin.H{"error": hostNotFoundMsg})
		return
	}

	// Find connected agent
	agent, exists := h.hub.GetAgentByHost(hostID)
	if !exists {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Host agent not connected"})
		return
	}

	// Ask agent for info
	command := protocol.NewCommandWithAction("get_docker_info", map[string]any{})
	response, err := h.sendCommandAndWait(agent.ID, command, 10*time.Second)
	if err != nil {
		logrus.Errorf("Failed to get docker info from host %s: %v", hostID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get host info"})
		return
	}

	c.JSON(http.StatusOK, response)
}

// ListContainers returns containers for a specific host
func (h *HostsHandler) ListContainers(c *gin.Context) {
	hostID := c.Param("id")

	// Check if host exists
	var host database.Host
	if err := database.DB.Where(hostIDQuery, hostID).First(&host).Error; err != nil {
		logrus.Errorf(hostNotFoundLog, hostID, err)
		h.addLog("warn", "container", "Attempted container creation on unknown host", map[string]any{
			"host_id": hostID,
		})
		c.JSON(http.StatusNotFound, gin.H{
			"error": hostNotFoundMsg,
		})
		return
	}

	// Check if agent is connected
	agent, exists := h.hub.GetAgentByHost(hostID)
	if !exists {
		h.addLog("error", "container", "Agent not connected for container creation", map[string]any{
			"host_id":   host.ID.String(),
			"host_name": host.Name,
		})
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Host agent not connected",
		})
		return
	}

	// Send command to agent
	command := protocol.NewCommandWithAction("list_containers", map[string]any{
		"all": true,
	})

	// Send command and wait for response
	response, err := h.sendCommandAndWait(agent.ID, command, 15*time.Second)
	if err != nil {
		logrus.Errorf("Failed to get containers from host %s: %v", hostID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve containers",
		})
		return
	}

	// Extract containers from response
	containers, ok := response["containers"].([]interface{})
	if !ok {
		logrus.Errorf("Invalid containers response format from host %s", hostID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Invalid response format from agent",
		})
		return
	}

	// Add host name for filtering consistency
	for i := range containers {
		if m, ok := containers[i].(map[string]any); ok {
			m["host_name"] = host.Name
		}
	}

	// Apply optional filtering
	q := strings.TrimSpace(c.Query("q"))
	if q != "" {
		ast, err := querydsl.Parse(q)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid query"})
			return
		}
		filtered := make([]map[string]any, 0, len(containers))
		for _, it := range containers {
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

	c.JSON(http.StatusOK, containers)
}

// ListAllContainers returns containers from all connected hosts
func (h *HostsHandler) ListAllContainers(c *gin.Context) {
	// Get all connected agents
	agents := h.hub.GetAgents()
	logrus.Infof("ListAllContainers: Found %d connected agents", len(agents))

	if len(agents) == 0 {
		c.JSON(http.StatusOK, []interface{}{})
		return
	}

	var allContainers []map[string]interface{}

	// Iterate through all connected agents
	for agentID, agent := range agents {
		logrus.Infof("ListAllContainers: Processing agent %s with host ID %s", agentID, agent.HostID)

		// Get host information from database
		var host database.Host
		if err := database.DB.Where(hostIDQuery, agent.HostID).First(&host).Error; err != nil {
			logrus.Errorf("Failed to get host %s for agent %s: %v", agent.HostID, agentID, err)
			continue
		}

		logrus.Infof("ListAllContainers: Found host %s (%s) for agent %s", host.ID.String(), host.Name, agentID)

		// Send command to agent to get containers
		command := protocol.NewCommandWithAction("list_containers", map[string]any{
			"all": true,
		})

		// Send command and wait for response
		response, err := h.sendCommandAndWait(agentID, command, 15*time.Second)
		if err != nil {
			logrus.Errorf("Failed to get containers from host %s (agent %s): %v", agent.HostID, agentID, err)
			continue
		}

		// Extract containers from response
		containers, ok := response["containers"].([]interface{})
		if !ok {
			logrus.Errorf("Invalid containers response format from host %s (agent %s)", agent.HostID, agentID)
			continue
		}

		logrus.Infof("ListAllContainers: Found %d containers from agent %s", len(containers), agentID)

		// Add host information to each container
		for _, container := range containers {
			if containerMap, ok := container.(map[string]interface{}); ok {
				containerMap["host_id"] = host.ID.String()
				containerMap["host_name"] = host.Name
				logrus.Debugf("ListAllContainers: Added host info to container %s: host_id=%s, host_name=%s",
					containerMap["name"], containerMap["host_id"], containerMap["host_name"])
				allContainers = append(allContainers, containerMap)
			}
		}
	}

	logrus.Infof("ListAllContainers: Returning %d total containers", len(allContainers))

	// Apply optional filtering
	q := strings.TrimSpace(c.Query("q"))
	if q != "" {
		ast, err := querydsl.Parse(q)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid query"})
			return
		}
		filtered := make([]map[string]any, 0, len(allContainers))
		for _, m := range allContainers {
			if querydsl.EvaluateRecord(ast, m) {
				filtered = append(filtered, m)
			}
		}
		out := make([]interface{}, len(filtered))
		for i := range filtered {
			out[i] = filtered[i]
		}
		c.JSON(http.StatusOK, out)
		return
	}

	c.JSON(http.StatusOK, allContainers)
}

// ListAllStacks returns stacks from all connected hosts
func (h *HostsHandler) ListAllStacks(c *gin.Context) {
	// Get all connected agents
	agents := h.hub.GetAgents()
	logrus.Infof("ListAllStacks: Found %d connected agents", len(agents))

	if len(agents) == 0 {
		c.JSON(http.StatusOK, []interface{}{})
		return
	}

	var allStacks []map[string]interface{}

	// Iterate through all connected agents
	for agentID, agent := range agents {
		logrus.Infof("ListAllStacks: Processing agent %s with host ID %s", agentID, agent.HostID)

		// Get host information from database
		var host database.Host
		if err := database.DB.Where(hostIDQuery, agent.HostID).First(&host).Error; err != nil {
			logrus.Errorf("Failed to get host %s for agent %s: %v", agent.HostID, agentID, err)
			continue
		}

		logrus.Infof("ListAllStacks: Found host %s (%s) for agent %s", host.ID.String(), host.Name, agentID)

		// Send command to agent to get stacks
		command := protocol.NewCommandWithAction("list_stacks", map[string]any{})

		// Send command and wait for response
		response, err := h.sendCommandAndWait(agentID, command, 15*time.Second)
		if err != nil {
			logrus.Errorf("Failed to get stacks from host %s (agent %s): %v", agent.HostID, agentID, err)
			continue
		}

		// Extract stacks from response
		stacks, ok := response["stacks"].([]interface{})
		if !ok {
			logrus.Errorf("Invalid stacks response format from host %s (agent %s)", agent.HostID, agentID)
			continue
		}

		logrus.Infof("ListAllStacks: Found %d stacks from agent %s", len(stacks), agentID)

		// Add host information to each stack
		for _, stack := range stacks {
			if stackMap, ok := stack.(map[string]interface{}); ok {
				stackMap["host_id"] = host.ID.String()
				stackMap["host_name"] = host.Name
				logrus.Debugf("ListAllStacks: Added host info to stack %s: host_id=%s, host_name=%s",
					stackMap["name"], stackMap["host_id"], stackMap["host_name"])
				allStacks = append(allStacks, stackMap)
			}
		}
	}

	logrus.Infof("ListAllStacks: Returning %d total stacks", len(allStacks))

	// Apply masking or decryption based on reveal_secrets and admin
	reveal := c.Query("reveal_secrets") == "1" || strings.EqualFold(c.Query("reveal_secrets"), "true")
	admin := false
	if reveal {
		admin = userIsAdmin(c)
	}
	for i := range allStacks {
		stackMap := allStacks[i]
		if envVars, ok := stackMap["env_vars"].(map[string]interface{}); ok {
			sensitive, _ := stackMap["env_vars_sensitive"].(bool)
			if sensitive {
				if reveal && admin {
					stackMap["env_vars"] = decryptEnvMapIfSensitive(envVars)
				} else {
					stackMap["env_vars"] = maskEnvMap(envVars)
				}
			}
		}
	}

	// Apply optional filtering
	q := strings.TrimSpace(c.Query("q"))
	if q != "" {
		ast, err := querydsl.Parse(q)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid query"})
			return
		}
		filtered := make([]map[string]any, 0, len(allStacks))
		for _, m := range allStacks {
			if querydsl.EvaluateRecord(ast, m) {
				filtered = append(filtered, m)
			}
		}
		out := make([]interface{}, len(filtered))
		for i := range filtered {
			out[i] = filtered[i]
		}
		c.JSON(http.StatusOK, out)
		return
	}

	c.JSON(http.StatusOK, allStacks)
}

// ListStacks returns stacks for a specific host
func (h *HostsHandler) ListStacks(c *gin.Context) {
	hostID := c.Param("id")

	// Check if host exists
	var host database.Host
	if err := database.DB.Where(hostIDQuery, hostID).First(&host).Error; err != nil {
		logrus.Errorf(hostNotFoundLog, hostID, err)
		h.addLog("warn", "stack", "Attempted stack deploy on unknown host", map[string]any{
			"host_id": hostID,
		})
		c.JSON(http.StatusNotFound, gin.H{
			"error": hostNotFoundMsg,
		})
		return
	}

	// Check if agent is connected
	agent, exists := h.hub.GetAgentByHost(hostID)
	if !exists {
		h.addLog("error", "stack", "Agent not connected for stack deploy", map[string]any{
			"host_id":   host.ID.String(),
			"host_name": host.Name,
		})
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Host agent not connected",
		})
		return
	}

	// Send command to agent
	command := protocol.NewCommandWithAction("list_stacks", map[string]any{})

	// Send command and wait for response
	response, err := h.sendCommandAndWait(agent.ID, command, 15*time.Second)
	if err != nil {
		logrus.Errorf("Failed to get stacks from host %s: %v", hostID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve stacks",
		})
		return
	}

	// Extract stacks from response
	stacks, ok := response["stacks"].([]interface{})
	if !ok {
		logrus.Errorf("Invalid stacks response format from host %s", hostID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Invalid response format from agent",
		})
		return
	}

	// Apply masking or decryption based on reveal_secrets and admin
	reveal2 := c.Query("reveal_secrets") == "1" || strings.EqualFold(c.Query("reveal_secrets"), "true")
	admin2 := false
	if reveal2 {
		admin2 = userIsAdmin(c)
	}
	for i := range stacks {
		if stackMap, ok := stacks[i].(map[string]interface{}); ok {
			if envVars, ok := stackMap["env_vars"].(map[string]interface{}); ok {
				sensitive, _ := stackMap["env_vars_sensitive"].(bool)
				if sensitive {
					if reveal2 && admin2 {
						stackMap["env_vars"] = decryptEnvMapIfSensitive(envVars)
					} else {
						stackMap["env_vars"] = maskEnvMap(envVars)
					}
				}
			}
		}
	}

	// Add host name for filtering consistency and apply optional filtering
	for i := range stacks {
		if m, ok := stacks[i].(map[string]any); ok {
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
		filtered := make([]map[string]any, 0, len(stacks))
		for _, it := range stacks {
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

	c.JSON(http.StatusOK, stacks)
}

// DeployStack deploys a new stack on a host
func (h *HostsHandler) DeployStack(c *gin.Context) {
	hostID := c.Param("id")

	// Check if host exists
	var host database.Host
	if err := database.DB.Where(hostIDQuery, hostID).First(&host).Error; err != nil {
		logrus.Errorf(hostNotFoundLog, hostID, err)
		h.addLog("warn", "stack", "Attempted stack import on unknown host", map[string]any{
			"host_id": hostID,
		})
		c.JSON(http.StatusNotFound, gin.H{
			"error": hostNotFoundMsg,
		})
		return
	}

	// Check if agent is connected
	agent, exists := h.hub.GetAgentByHost(hostID)
	if !exists {
		h.addLog("error", "stack", "Agent not connected for stack import", map[string]any{
			"host_id":   host.ID.String(),
			"host_name": host.Name,
		})
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Host agent not connected",
		})
		return
	}

	// Parse request body
	var requestBody map[string]interface{}
	if err := c.ShouldBindJSON(&requestBody); err != nil {
		h.addLog("warn", "stack", "Invalid stack deploy payload", map[string]any{
			"host_id":   host.ID.String(),
			"host_name": host.Name,
			"error":     err.Error(),
		})
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
		})
		return
	}

	// Send command to agent
	command := protocol.NewCommandWithAction("deploy_stack", requestBody)

	// Send command and wait for response
	response, err := h.sendCommandAndWait(agent.ID, command, 120*time.Second)
	if err != nil {
		logrus.Errorf("Failed to deploy stack on host %s: %v", hostID, err)
		h.addLog("error", "stack", "Failed to deploy stack", map[string]any{
			"host_id":   host.ID.String(),
			"host_name": host.Name,
			"error":     err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to deploy stack",
		})
		return
	}

	stackName := ""
	if name, ok := requestBody["name"].(string); ok {
		stackName = name
	} else if name, ok := response["name"].(string); ok {
		stackName = name
	}
	h.addLog("info", "stack", "Deployed stack", map[string]any{
		"host_id":    host.ID.String(),
		"host_name":  host.Name,
		"stack_name": stackName,
	})
	c.JSON(http.StatusOK, response)
}

// StackAction performs an action on a stack
func (h *HostsHandler) StackAction(c *gin.Context) {
	hostID := c.Param("id")
	stackName := c.Param("stack_name")
	action := c.Param("action")

	// Validate action
	validActions := map[string]bool{
		"start":   true,
		"stop":    true,
		"restart": true,
		"remove":  true,
		"update":  true,
	}

	if !validActions[action] {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid action. Must be one of: start, stop, restart, remove, update",
		})
		h.addLog("warn", "stack", "Invalid stack action requested", map[string]any{
			"host_id":    hostID,
			"stack_name": stackName,
			"action":     action,
		})
		return
	}

	// Check if host exists
	var host database.Host
	if err := database.DB.Where(hostIDQuery, hostID).First(&host).Error; err != nil {
		logrus.Errorf(hostNotFoundLog, hostID, err)
		h.addLog("warn", "stack", "Attempted stack action on unknown host", map[string]any{
			"host_id":    hostID,
			"stack_name": stackName,
			"action":     action,
		})
		c.JSON(http.StatusNotFound, gin.H{
			"error": hostNotFoundMsg,
		})
		return
	}

	// Check if agent is connected
	agent, exists := h.hub.GetAgentByHost(hostID)
	if !exists {
		h.addLog("error", "stack", "Agent not connected for stack action", map[string]any{
			"host_id":    host.ID.String(),
			"host_name":  host.Name,
			"stack_name": stackName,
			"action":     action,
		})
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Host agent not connected",
		})
		return
	}

	// Prepare command parameters
	params := map[string]any{
		"name": stackName,
	}

	// For update action, parse request body
	if action == "update" {
		var requestBody map[string]interface{}
		if err := c.ShouldBindJSON(&requestBody); err != nil {
			h.addLog("warn", "stack", "Invalid stack update payload", map[string]any{
				"host_id":    host.ID.String(),
				"host_name":  host.Name,
				"stack_name": stackName,
				"action":     action,
				"error":      err.Error(),
			})
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid request body",
			})
			return
		}
		// Merge request body into params
		for k, v := range requestBody {
			params[k] = v
		}
	}

	// Send command to agent
	command := protocol.NewCommandWithAction(action+"_stack", params)

	// Send command and wait for response
	timeout := 30 * time.Second
	if action == "remove" || action == "update" {
		timeout = 120 * time.Second // 2 minutes for remove/update
	}
	response, err := h.sendCommandAndWait(agent.ID, command, timeout)
	if err != nil {
		logrus.Errorf("Failed to %s stack %s on host %s: %v", action, stackName, hostID, err)
		h.addLog("error", "stack", "Stack action failed", map[string]any{
			"host_id":    host.ID.String(),
			"host_name":  host.Name,
			"stack_name": stackName,
			"action":     action,
			"error":      err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to perform stack action",
		})
		return
	}

	h.addLog("info", "stack", "Stack action completed", map[string]any{
		"host_id":    host.ID.String(),
		"host_name":  host.Name,
		"stack_name": stackName,
		"action":     action,
	})
	c.JSON(http.StatusOK, response)
}

// ImportStack imports an existing stack into Flotilla management
func (h *HostsHandler) ImportStack(c *gin.Context) {
	hostID := c.Param("id")

	// Check if host exists
	var host database.Host
	if err := database.DB.Where(hostIDQuery, hostID).First(&host).Error; err != nil {
		logrus.Errorf(hostNotFoundLog, hostID, err)
		c.JSON(http.StatusNotFound, gin.H{
			"error": hostNotFoundMsg,
		})
		return
	}

	// Check if agent is connected
	agent, exists := h.hub.GetAgentByHost(hostID)
	if !exists {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Host agent not connected",
		})
		return
	}

	// Parse request body
	var requestBody map[string]interface{}
	if err := c.ShouldBindJSON(&requestBody); err != nil {
		h.addLog("warn", "stack", "Invalid stack import payload", map[string]any{
			"host_id":   host.ID.String(),
			"host_name": host.Name,
			"error":     err.Error(),
		})
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
		})
		return
	}

	// Send command to agent
	command := protocol.NewCommandWithAction("import_stack", requestBody)

	// Send command and wait for response
	response, err := h.sendCommandAndWait(agent.ID, command, 60*time.Second)
	if err != nil {
		logrus.Errorf("Failed to import stack on host %s: %v", hostID, err)
		h.addLog("error", "stack", "Failed to import stack", map[string]any{
			"host_id":   host.ID.String(),
			"host_name": host.Name,
			"error":     err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to import stack",
		})
		return
	}

	stackName := ""
	if name, ok := requestBody["name"].(string); ok {
		stackName = name
	} else if name, ok := response["name"].(string); ok {
		stackName = name
	}
	h.addLog("info", "stack", "Imported stack", map[string]any{
		"host_id":    host.ID.String(),
		"host_name":  host.Name,
		"stack_name": stackName,
	})
	c.JSON(http.StatusOK, response)
}

// GetStackContainers returns containers in a specific stack
func (h *HostsHandler) GetStackContainers(c *gin.Context) {
	hostID := c.Param("id")
	stackName := c.Param("stack_name")

	// Check if host exists
	var host database.Host
	if err := database.DB.Where(hostIDQuery, hostID).First(&host).Error; err != nil {
		logrus.Errorf(hostNotFoundLog, hostID, err)
		c.JSON(http.StatusNotFound, gin.H{
			"error": hostNotFoundMsg,
		})
		return
	}

	// Check if agent is connected
	agent, exists := h.hub.GetAgentByHost(hostID)
	if !exists {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Host agent not connected",
		})
		return
	}

	// Send command to agent
	command := protocol.NewCommandWithAction("get_stack_containers", map[string]any{
		"stack_name": stackName,
	})

	// Send command and wait for response
	response, err := h.sendCommandAndWait(agent.ID, command, 30*time.Second)
	if err != nil {
		logrus.Errorf("Failed to get stack containers from host %s: %v", hostID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get stack containers",
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// StackContainerAction performs action on a container within a stack
func (h *HostsHandler) StackContainerAction(c *gin.Context) {
	hostID := c.Param("id")
	stackName := c.Param("stack_name")
	containerID := c.Param("container_id")
	action := c.Param("action") // start, stop, restart

	// Validate action
	if action != "start" && action != "stop" && action != "restart" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid action",
		})
		h.addLog("warn", "stack", "Invalid stack container action requested", map[string]any{
			"host_id":      hostID,
			"stack_name":   stackName,
			"container_id": containerID,
			"action":       action,
		})
		return
	}

	// Check if host exists
	var host database.Host
	if err := database.DB.Where(hostIDQuery, hostID).First(&host).Error; err != nil {
		logrus.Errorf(hostNotFoundLog, hostID, err)
		h.addLog("warn", "stack", "Attempted stack container action on unknown host", map[string]any{
			"host_id":      hostID,
			"stack_name":   stackName,
			"container_id": containerID,
			"action":       action,
		})
		c.JSON(http.StatusNotFound, gin.H{
			"error": hostNotFoundMsg,
		})
		return
	}

	// Check if agent is connected
	agent, exists := h.hub.GetAgentByHost(hostID)
	if !exists {
		h.addLog("error", "stack", "Agent not connected for stack container action", map[string]any{
			"host_id":      host.ID.String(),
			"host_name":    host.Name,
			"stack_name":   stackName,
			"container_id": containerID,
			"action":       action,
		})
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Host agent not connected",
		})
		return
	}

	// Send command to agent
	command := protocol.NewCommandWithAction("stack_container_action", map[string]any{
		"container_id": containerID,
		"action":       action,
	})

	// Send command and wait for response
	response, err := h.sendCommandAndWait(agent.ID, command, 30*time.Second)
	if err != nil {
		logrus.Errorf("Failed to %s container %s in stack %s on host %s: %v", action, containerID, stackName, hostID, err)
		h.addLog("error", "stack", "Stack container action failed", map[string]any{
			"host_id":      host.ID.String(),
			"host_name":    host.Name,
			"stack_name":   stackName,
			"container_id": containerID,
			"action":       action,
			"error":        err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to %s container", action),
		})
		return
	}

	h.addLog("info", "stack", "Stack container action completed", map[string]any{
		"host_id":      host.ID.String(),
		"host_name":    host.Name,
		"stack_name":   stackName,
		"container_id": containerID,
		"action":       action,
	})
	c.JSON(http.StatusOK, response)
}

// CreateContainer creates a new container on a host
func (h *HostsHandler) CreateContainer(c *gin.Context) {
	hostID := c.Param("id")

	// Check if host exists
	var host database.Host
	if err := database.DB.Where(hostIDQuery, hostID).First(&host).Error; err != nil {
		logrus.Errorf(hostNotFoundLog, hostID, err)
		c.JSON(http.StatusNotFound, gin.H{
			"error": hostNotFoundMsg,
		})
		return
	}

	// Check if agent is connected
	agent, exists := h.hub.GetAgentByHost(hostID)
	if !exists {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Host agent not connected",
		})
		return
	}

	// Parse request body
	var requestBody map[string]interface{}
	if err := c.ShouldBindJSON(&requestBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
		})
		return
	}

	// Send command to agent
	command := protocol.NewCommandWithAction("create_container", requestBody)

	// Send command and wait for response
	response, err := h.sendCommandAndWait(agent.ID, command, 60*time.Second)
	if err != nil {
		logrus.Errorf("Failed to create container on host %s: %v", hostID, err)
		h.addLog("error", "container", "Failed to create container", map[string]any{
			"host_id":   host.ID.String(),
			"host_name": host.Name,
			"error":     err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create container",
		})
		return
	}

	containerID, _ := response["container_id"].(string)
	containerName := ""
	if n, ok := response["name"].(string); ok {
		containerName = n
	} else if n, ok := requestBody["name"].(string); ok {
		containerName = n
	}
	h.addLog("info", "container", "Created container", map[string]any{
		"host_id":        host.ID.String(),
		"host_name":      host.Name,
		"container_id":   containerID,
		"container_name": containerName,
	})
	c.JSON(http.StatusOK, response)
}

// ContainerAction performs an action on a container
func (h *HostsHandler) ContainerAction(c *gin.Context) {
	hostID := c.Param("id")
	containerID := c.Param("container_id")
	action := c.Param("action")

	// Validate action
	validActions := map[string]bool{
		"start":   true,
		"stop":    true,
		"restart": true,
		"remove":  true,
	}

	if !validActions[action] {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid action. Must be one of: start, stop, restart, remove",
		})
		h.addLog("warn", "container", "Invalid container action requested", map[string]any{
			"host_id":      hostID,
			"container_id": containerID,
			"action":       action,
		})
		return
	}

	// Check if host exists
	var host database.Host
	if err := database.DB.Where(hostIDQuery, hostID).First(&host).Error; err != nil {
		logrus.Errorf(hostNotFoundLog, hostID, err)
		h.addLog("warn", "container", "Attempted container action on unknown host", map[string]any{
			"host_id":      hostID,
			"container_id": containerID,
			"action":       action,
		})
		c.JSON(http.StatusNotFound, gin.H{
			"error": hostNotFoundMsg,
		})
		return
	}

	// Check if agent is connected
	agent, exists := h.hub.GetAgentByHost(hostID)
	if !exists {
		h.addLog("error", "container", "Agent not connected for container action", map[string]any{
			"host_id":      host.ID.String(),
			"host_name":    host.Name,
			"container_id": containerID,
			"action":       action,
		})
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Host agent not connected",
		})
		return
	}

	// Prepare command parameters
	params := map[string]any{
		"container_id": containerID,
	}
	containerName := strings.TrimSpace(c.Query("name"))
	if containerName != "" {
		params["container_name"] = containerName
	}

	// Add timeout for stop/restart actions
	if action == "stop" || action == "restart" {
		if timeoutStr := c.Query("timeout"); timeoutStr != "" {
			if timeout, err := strconv.Atoi(timeoutStr); err == nil {
				params["timeout"] = timeout
			}
		}
	}

	// Add force parameter for remove action
	if action == "remove" {
		if forceStr := c.Query("force"); forceStr == "true" {
			params["force"] = true
		}
	}

	// Send command to agent
	command := protocol.NewCommandWithAction(action+"_container", params)

	// Send command and wait for response
	// Use longer timeout for stop/restart operations as they can take time
	timeout := 30 * time.Second
	if action == "stop" || action == "restart" {
		timeout = 120 * time.Second // 2 minutes for stop/restart
	}
	response, err := h.sendCommandAndWait(agent.ID, command, timeout)
	if err != nil {
		logrus.Errorf("Failed to %s container %s on host %s: %v", action, containerID, hostID, err)
		h.addLog("error", "container", "Container action failed", map[string]any{
			"host_id":        host.ID.String(),
			"host_name":      host.Name,
			"container_id":   containerID,
			"action":         action,
			"error":          err.Error(),
			"container_name": containerName,
		})
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to perform container action",
		})
		return
	}

	h.addLog("info", "container", "Container action completed", map[string]any{
		"host_id":        host.ID.String(),
		"host_name":      host.Name,
		"container_id":   containerID,
		"action":         action,
		"container_name": containerName,
	})
	c.JSON(http.StatusOK, response)
}

// sendCommandAndWait sends a command to an agent and waits for the response
func (h *HostsHandler) sendCommandAndWait(agentID string, command *protocol.Message, timeout time.Duration) (map[string]any, error) {
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

func userIsAdmin(c *gin.Context) bool {
	header := c.GetHeader("Authorization")
	if len(header) >= 8 && strings.HasPrefix(header, "Bearer ") {
		tok := header[7:]
		if claims, err := auth.ParseAccessToken(tok); err == nil {
			return strings.EqualFold(claims.Role, "admin")
		}
	}
	return false
}

func maskEnvMap(envVars map[string]any) map[string]any {
	masked := make(map[string]any, len(envVars))
	for k := range envVars {
		masked[k] = "****"
	}
	return masked
}

func decryptEnvMapIfSensitive(envVars map[string]any) map[string]any {
	out := make(map[string]any, len(envVars))
	for k, v := range envVars {
		if s, ok := v.(string); ok && s != "" {
			if pt, err := sharedconfig.DecryptValue(s); err == nil {
				out[k] = pt
				continue
			}
		}
		out[k] = v
	}
	return out
}
