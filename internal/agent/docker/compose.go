package docker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

const (
	dockerComposeFileName  = "docker-compose.yml"
	envFileName            = ".env"
	composeProjectLabel    = "com.docker.compose.project"
	flotillaManagedLabel   = "io.flotilla.managed"
	flotillaStackNameLabel = "io.flotilla.stack.name"
	flotillaDeployedLabel  = "io.flotilla.deployed.timestamp"
	composeDirPerm         = 0o750
	composeFilePerm        = 0o600
)

var (
	errDockerComposeOutput    = "Docker compose output: %s"
	errFailedToListContainers = "failed to list containers: %w"
	stackNamePattern          = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
	composeArgPattern         = regexp.MustCompile(`^[A-Za-z0-9/_:@.=+-]+$`)
)

// runCompose tries Docker Compose v2 first ("docker compose"), then falls back to v1 ("docker-compose").
func runCompose(ctx context.Context, workDir string, args ...string) ([]byte, error) {
	if err := validateComposeArgs(args); err != nil {
		return nil, err
	}
	// Try v2: docker compose <args>
	v2Args := append([]string{"compose"}, args...)
	cmdV2 := exec.CommandContext(ctx, "docker", v2Args...)
	cmdV2.Dir = workDir
	cmdV2.Env = os.Environ()
	outV2, errV2 := cmdV2.CombinedOutput()
	if errV2 == nil {
		return outV2, nil
	}

	// Try v1: docker-compose <args>
	cmdV1 := exec.CommandContext(ctx, "docker-compose", args...)
	cmdV1.Dir = workDir
	cmdV1.Env = os.Environ()
	outV1, errV1 := cmdV1.CombinedOutput()
	if errV1 == nil {
		return outV1, nil
	}

	// Both failed: return v1 error, but log both outputs for diagnostics
	if len(outV2) > 0 {
		logrus.Errorf("Docker compose (v2) output: %s", strings.TrimSpace(string(outV2)))
	}
	if len(outV1) > 0 {
		logrus.Errorf("Docker compose (v1) output: %s", strings.TrimSpace(string(outV1)))
	}
	return nil, fmt.Errorf("docker compose failed: v2 error: %w; v1 error: %w", errV2, errV1)
}

// ComposeClient handles Docker Compose operations
type ComposeClient struct {
	dockerClient *Client
	workDir      string
}

func sanitizeStackName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", fmt.Errorf("stack name cannot be empty")
	}
	if !stackNamePattern.MatchString(trimmed) {
		return "", fmt.Errorf("stack name contains invalid characters")
	}
	return trimmed, nil
}

func validateComposeArgs(args []string) error {
	for _, arg := range args {
		if !composeArgPattern.MatchString(arg) {
			return fmt.Errorf("compose argument %q contains invalid characters", arg)
		}
	}
	return nil
}

// NewComposeClient creates a new compose client
func NewComposeClient(dockerClient *Client) *ComposeClient {
	// Create a temporary directory for compose files
	workDir := "/tmp/flotilla-compose"
	if err := os.MkdirAll(workDir, composeDirPerm); err != nil {
		logrus.WithError(err).Fatal("failed to create compose working directory")
	}

	return &ComposeClient{
		dockerClient: dockerClient,
		workDir:      workDir,
	}
}

func (c *ComposeClient) safeStackDir(stackName string) (string, string, error) {
	safeName, err := sanitizeStackName(stackName)
	if err != nil {
		return "", "", err
	}
	return filepath.Join(c.workDir, safeName), safeName, nil
}

// injectFlotillaLabels adds Flotilla management labels to compose file
func injectFlotillaLabels(composeContent, stackName string) (string, error) {
	var config map[string]interface{}
	if err := yaml.Unmarshal([]byte(composeContent), &config); err != nil {
		return "", fmt.Errorf("failed to parse compose file: %w", err)
	}

	// Ensure services section exists
	services, ok := config["services"].(map[string]interface{})
	if !ok {
		// If no services section, return as-is (invalid compose but don't break)
		logrus.Warnf("No services section found in compose file for stack %s", stackName)
		return composeContent, nil
	}

	// Add labels to each service
	timestamp := time.Now().UTC().Format(time.RFC3339)
	for _, service := range services {
		serviceMap, ok := service.(map[string]interface{})
		if !ok {
			continue
		}

		// Initialize labels section if it doesn't exist
		if _, exists := serviceMap["labels"]; !exists {
			serviceMap["labels"] = make(map[string]interface{})
		}

		labels, ok := serviceMap["labels"].(map[string]interface{})
		if !ok {
			// Convert array format to map if needed
			if labelsArr, ok := serviceMap["labels"].([]interface{}); ok {
				labels = make(map[string]interface{})
				for _, label := range labelsArr {
					if labelStr, ok := label.(string); ok {
						parts := strings.SplitN(labelStr, "=", 2)
						if len(parts) == 2 {
							labels[parts[0]] = parts[1]
						}
					}
				}
				serviceMap["labels"] = labels
			} else {
				serviceMap["labels"] = make(map[string]interface{})
				labels = serviceMap["labels"].(map[string]interface{})
			}
		}

		// Add Flotilla labels
		labels[flotillaManagedLabel] = "true"
		labels[flotillaStackNameLabel] = stackName
		labels[flotillaDeployedLabel] = timestamp
	}

	// Marshal back to YAML
	result, err := yaml.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("failed to marshal compose file: %w", err)
	}

	return string(result), nil
}

// DeployStack deploys a new stack from a compose file
func (c *ComposeClient) DeployStack(ctx context.Context, stackName, composeContent string, envVars map[string]interface{}) error {
	logrus.Infof("Deploying stack: %s", stackName)

	// Inject Flotilla management labels
	composeWithLabels, err := injectFlotillaLabels(composeContent, stackName)
	if err != nil {
		logrus.Warnf("Failed to inject Flotilla labels: %v, deploying without labels", err)
		composeWithLabels = composeContent
	}

	// Create a temporary directory for this stack
	stackDir, safeName, err := c.safeStackDir(stackName)
	if err != nil {
		return fmt.Errorf("invalid stack name: %w", err)
	}
	if err := os.MkdirAll(stackDir, composeDirPerm); err != nil {
		return fmt.Errorf("failed to create stack directory: %w", err)
	}

	// Write compose file
	composePath := filepath.Join(stackDir, dockerComposeFileName)
	if err := os.WriteFile(composePath, []byte(composeWithLabels), composeFilePerm); err != nil {
		return fmt.Errorf("failed to write compose file: %w", err)
	}

	// Create .env file if env vars are provided
	if len(envVars) > 0 {
		envPath := filepath.Join(stackDir, envFileName)
		envLines := []string{}
		for k, v := range envVars {
			envLines = append(envLines, fmt.Sprintf("%s=%v", k, v))
		}
		if err := os.WriteFile(envPath, []byte(strings.Join(envLines, "\n")), composeFilePerm); err != nil {
			logrus.Warnf("Failed to write .env file: %v", err)
		}
	}

	// Execute compose up
	output, err := runCompose(ctx, stackDir, "-p", safeName, "up", "-d")
	if err != nil {
		logrus.Errorf(errDockerComposeOutput, string(output))
		return fmt.Errorf("failed to deploy stack: %w", err)
	}

	logrus.Infof("Stack deployed successfully: %s", stackName)
	return nil
}

// UpdateStack updates an existing stack
func (c *ComposeClient) UpdateStack(ctx context.Context, stackName, composeContent string, envVars map[string]interface{}) error {
	logrus.Infof("Updating stack: %s", stackName)

	// Inject Flotilla management labels
	composeWithLabels, err := injectFlotillaLabels(composeContent, stackName)
	if err != nil {
		logrus.Warnf("Failed to inject Flotilla labels: %v, updating without labels", err)
		composeWithLabels = composeContent
	}

	// Get the stack directory
	stackDir, safeName, err := c.safeStackDir(stackName)
	if err != nil {
		return fmt.Errorf("invalid stack name: %w", err)
	}

	// Write updated compose file
	composePath := filepath.Join(stackDir, dockerComposeFileName)
	if err := os.WriteFile(composePath, []byte(composeWithLabels), composeFilePerm); err != nil {
		return fmt.Errorf("failed to write compose file: %w", err)
	}

	// Update .env file if env vars are provided
	if len(envVars) > 0 {
		envPath := filepath.Join(stackDir, envFileName)
		envLines := []string{}
		for k, v := range envVars {
			envLines = append(envLines, fmt.Sprintf("%s=%v", k, v))
		}
		if err := os.WriteFile(envPath, []byte(strings.Join(envLines, "\n")), composeFilePerm); err != nil {
			logrus.Warnf("Failed to write .env file: %v", err)
		}
	}

	// Execute compose up with --force-recreate
	output, err := runCompose(ctx, stackDir, "-p", safeName, "up", "-d", "--force-recreate")
	if err != nil {
		logrus.Errorf(errDockerComposeOutput, string(output))
		return fmt.Errorf("failed to update stack: %w", err)
	}

	logrus.Infof("Stack updated successfully: %s", stackName)
	return nil
}

// RemoveStack removes a stack
func (c *ComposeClient) RemoveStack(ctx context.Context, stackName string) error {
	logrus.Infof("Removing stack: %s", stackName)

	// Get the stack directory
	stackDir, safeName, err := c.safeStackDir(stackName)
	if err != nil {
		return fmt.Errorf("invalid stack name: %w", err)
	}

	// Check if stack directory exists
	if _, err := os.Stat(stackDir); os.IsNotExist(err) {
		logrus.Warnf("Stack directory not found: %s", stackDir)
		// Try to remove anyway using docker-compose with the stack name
	} else {
		// Execute compose down
		output, err := runCompose(ctx, stackDir, "-p", safeName, "down", "-v")
		if err != nil {
			logrus.Errorf(errDockerComposeOutput, string(output))
			return fmt.Errorf("failed to remove stack: %w", err)
		}

		// Remove the stack directory
		if err := os.RemoveAll(stackDir); err != nil {
			logrus.Warnf("Failed to remove stack directory: %v", err)
		}
	}

	logrus.Infof("Stack removed successfully: %s", stackName)
	return nil
}

// ListStacks lists all stacks by inspecting containers with compose labels
func (c *ComposeClient) ListStacks(ctx context.Context) ([]map[string]interface{}, error) {
	logrus.Debug("Listing stacks")

	// List all containers
	containers, err := c.dockerClient.ListContainers(ctx, true)
	if err != nil {
		return nil, fmt.Errorf(errFailedToListContainers, err)
	}

	// Group containers by stack name (using com.docker.compose.project label)
	stackMap := make(map[string][]types.Container)
	for _, container := range containers {
		stackName := "unknown"
		if project, ok := container.Labels[composeProjectLabel]; ok {
			stackName = project
		}

		stackMap[stackName] = append(stackMap[stackName], container)
	}

	// Convert to list format
	stacks := []map[string]interface{}{}
	for stackName, containers := range stackMap {
		// Skip containers without a compose project label
		if stackName == "unknown" {
			continue
		}

		// Count running containers
		runningCount := 0
		for _, container := range containers {
			if container.State == "running" {
				runningCount++
			}
		}

		// Determine stack status
		status := "stopped"
		if runningCount > 0 {
			if runningCount == len(containers) {
				status = "running"
			} else {
				status = "partial"
			}
		}

		// Get stack info from compose file if available
		stackDir, _, err := c.safeStackDir(stackName)
		composeContent := ""
		if err == nil {
			composePath := filepath.Join(stackDir, dockerComposeFileName)
			if _, err := os.Stat(composePath); err == nil {
				content, readErr := os.ReadFile(composePath)
				if readErr == nil {
					composeContent = string(content)
				}
			}
		} else {
			logrus.WithError(err).Debugf("Skipping compose metadata for stack %s", stackName)
		}

		// Check if stack is managed by Flotilla by looking at labels
		managedByFlotilla := false
		for _, container := range containers {
			if managed, ok := container.Labels[flotillaManagedLabel]; ok && managed == "true" {
				managedByFlotilla = true
				break
			}
		}

		// Get creation time from stack directory
		createdAt := ""
		if err == nil {
			if info, statErr := os.Stat(stackDir); statErr == nil {
				createdAt = info.ModTime().Format(time.RFC3339)
			}
		}

		stack := map[string]interface{}{
			"name":                stackName,
			"status":              status,
			"containers":          len(containers),
			"running":             runningCount,
			"compose_content":     composeContent,
			"managed_by_flotilla": managedByFlotilla,
			"created_at":          createdAt,
		}

		stacks = append(stacks, stack)
	}

	logrus.Debugf("Found %d stacks", len(stacks))
	return stacks, nil
}

// GetStack retrieves detailed information about a specific stack
func (c *ComposeClient) GetStack(ctx context.Context, stackName string) (map[string]interface{}, error) {
	logrus.Debugf("Getting stack details: %s", stackName)

	// List all containers
	containers, err := c.dockerClient.ListContainers(ctx, true)
	if err != nil {
		return nil, fmt.Errorf(errFailedToListContainers, err)
	}

	// Filter containers by stack name
	stackContainers := []types.Container{}
	for _, container := range containers {
		if project, ok := container.Labels[composeProjectLabel]; ok && project == stackName {
			stackContainers = append(stackContainers, container)
		}
	}

	if len(stackContainers) == 0 {
		return nil, fmt.Errorf("stack not found: %s", stackName)
	}

	// Count running containers
	runningCount := 0
	for _, container := range stackContainers {
		if container.State == "running" {
			runningCount++
		}
	}

	// Determine stack status
	status := "stopped"
	if runningCount > 0 {
		if runningCount == len(stackContainers) {
			status = "running"
		} else {
			status = "partial"
		}
	}

	stackDir, _, err := c.safeStackDir(stackName)
	if err != nil {
		return nil, fmt.Errorf("invalid stack name: %w", err)
	}
	// Get compose file content
	composePath := filepath.Join(stackDir, dockerComposeFileName)
	composeContent := ""
	if _, err := os.Stat(composePath); err == nil {
		content, err := os.ReadFile(composePath)
		if err == nil {
			composeContent = string(content)
		}
	}

	// Get .env file content
	envPath := filepath.Join(stackDir, envFileName)
	envVars := map[string]interface{}{}
	if _, err := os.Stat(envPath); err == nil {
		content, err := os.ReadFile(envPath)
		if err == nil {
			envLines := strings.Split(string(content), "\n")
			for _, line := range envLines {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					envVars[parts[0]] = parts[1]
				}
			}
		}
	}

	// Convert containers to a more friendly format
	containerList := make([]map[string]interface{}, len(stackContainers))
	for i, container := range stackContainers {
		containerName := ""
		if len(container.Names) > 0 {
			containerName = strings.TrimPrefix(container.Names[0], "/")
		}
		if containerName == "" {
			containerName = container.ID[:12]
		}

		containerList[i] = map[string]interface{}{
			"id":     container.ID,
			"name":   containerName,
			"image":  container.Image,
			"status": container.State,
			"ports":  container.Ports,
		}
	}

	return map[string]interface{}{
		"name":            stackName,
		"status":          status,
		"containers":      containerList,
		"compose_content": composeContent,
		"env_vars":        envVars,
	}, nil
}

// StartStack starts a stopped stack
func (c *ComposeClient) StartStack(ctx context.Context, stackName string) error {
	logrus.Infof("Starting stack: %s", stackName)

	stackDir, safeName, err := c.safeStackDir(stackName)
	if err != nil {
		return fmt.Errorf("invalid stack name: %w", err)
	}

	output, err := runCompose(ctx, stackDir, "-p", safeName, "start")
	if err != nil {
		logrus.Errorf(errDockerComposeOutput, string(output))
		return fmt.Errorf("failed to start stack: %w", err)
	}

	logrus.Infof("Stack started successfully: %s", stackName)
	return nil
}

// StopStack stops a running stack
func (c *ComposeClient) StopStack(ctx context.Context, stackName string) error {
	logrus.Infof("Stopping stack: %s", stackName)

	stackDir, safeName, err := c.safeStackDir(stackName)
	if err != nil {
		return fmt.Errorf("invalid stack name: %w", err)
	}

	output, err := runCompose(ctx, stackDir, "-p", safeName, "stop")
	if err != nil {
		logrus.Errorf(errDockerComposeOutput, string(output))
		return fmt.Errorf("failed to stop stack: %w", err)
	}

	logrus.Infof("Stack stopped successfully: %s", stackName)
	return nil
}

// RestartStack restarts a stack
func (c *ComposeClient) RestartStack(ctx context.Context, stackName string) error {
	logrus.Infof("Restarting stack: %s", stackName)

	stackDir, safeName, err := c.safeStackDir(stackName)
	if err != nil {
		return fmt.Errorf("invalid stack name: %w", err)
	}

	output, err := runCompose(ctx, stackDir, "-p", safeName, "restart")
	if err != nil {
		logrus.Errorf(errDockerComposeOutput, string(output))
		return fmt.Errorf("failed to restart stack: %w", err)
	}

	logrus.Infof("Stack restarted successfully: %s", stackName)
	return nil
}

// CheckDockerCompose checks if docker-compose is available
func (c *ComposeClient) CheckDockerCompose() error {
	// Prefer v2
	cmdV2 := exec.Command("docker", "compose", "version")
	outV2, errV2 := cmdV2.CombinedOutput()
	if errV2 == nil {
		logrus.Debugf("Docker Compose v2: %s", strings.TrimSpace(string(outV2)))
		return nil
	}

	// Fallback to v1
	cmdV1 := exec.Command("docker-compose", "version")
	outV1, errV1 := cmdV1.CombinedOutput()
	if errV1 == nil {
		logrus.Debugf("Docker Compose v1: %s", strings.TrimSpace(string(outV1)))
		return nil
	}

	if len(outV2) > 0 {
		logrus.Debugf("docker compose output: %s", strings.TrimSpace(string(outV2)))
	}
	if len(outV1) > 0 {
		logrus.Debugf("docker-compose output: %s", strings.TrimSpace(string(outV1)))
	}
	return fmt.Errorf("docker compose not available: v2 error: %w; v1 error: %w", errV2, errV1)
}

// ImportStack imports an existing stack into Flotilla management
func (c *ComposeClient) ImportStack(ctx context.Context, stackName, composeContent string, envVars map[string]interface{}) error {
	logrus.Infof("Importing stack: %s", stackName)

	// Verify stack exists by checking containers
	containers, err := c.dockerClient.ListContainers(ctx, true)
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	// Check if stack exists
	stackExists := false
	for _, container := range containers {
		if project, ok := container.Labels[composeProjectLabel]; ok && project == stackName {
			stackExists = true
			break
		}
	}

	if !stackExists {
		return fmt.Errorf("stack '%s' not found - no containers with matching project label", stackName)
	}

	// Create stack directory
	stackDir, _, err := c.safeStackDir(stackName)
	if err != nil {
		return fmt.Errorf("invalid stack name: %w", err)
	}
	if err := os.MkdirAll(stackDir, composeDirPerm); err != nil {
		return fmt.Errorf("failed to create stack directory: %w", err)
	}

	// Write compose file
	composePath := filepath.Join(stackDir, dockerComposeFileName)
	if err := os.WriteFile(composePath, []byte(composeContent), composeFilePerm); err != nil {
		return fmt.Errorf("failed to write compose file: %w", err)
	}

	// Write .env file if env vars are provided
	if len(envVars) > 0 {
		envPath := filepath.Join(stackDir, envFileName)
		envLines := []string{}
		for k, v := range envVars {
			envLines = append(envLines, fmt.Sprintf("%s=%v", k, v))
		}
		if err := os.WriteFile(envPath, []byte(strings.Join(envLines, "\n")), composeFilePerm); err != nil {
			logrus.Warnf("Failed to write .env file: %v", err)
		}
	}

	logrus.Infof("Stack imported successfully: %s", stackName)
	return nil
}

// CleanupStaleStacks removes stacks that no longer have any containers
func (c *ComposeClient) CleanupStaleStacks(ctx context.Context) error {
	logrus.Debug("Cleaning up stale stacks")

	// List all containers
	containers, err := c.dockerClient.ListContainers(ctx, true)
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	// Get all active stack names
	activeStacks := make(map[string]bool)
	for _, container := range containers {
		if project, ok := container.Labels[composeProjectLabel]; ok {
			activeStacks[project] = true
		}
	}

	// Check stack directories
	entries, err := os.ReadDir(c.workDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read work directory: %w", err)
	}

	// Remove directories for inactive stacks
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() && stackNamePattern.MatchString(name) && !activeStacks[name] {
			stackDir := filepath.Join(c.workDir, name)
			logrus.Infof("Removing stale stack directory: %s", stackDir)
			if err := os.RemoveAll(stackDir); err != nil {
				logrus.Warnf("Failed to remove stale stack directory: %v", err)
			}
		}
	}

	return nil
}

// GetStackContainers returns detailed info about containers in a stack
func (c *ComposeClient) GetStackContainers(ctx context.Context, stackName string) ([]map[string]interface{}, error) {
	logrus.Debugf("Getting containers for stack: %s", stackName)

	containers, err := c.dockerClient.ListContainers(ctx, true)
	if err != nil {
		return nil, fmt.Errorf(errFailedToListContainers, err)
	}

	stackContainers := []map[string]interface{}{}
	for _, container := range containers {
		if project, ok := container.Labels[composeProjectLabel]; ok && project == stackName {
			// Extract service name from labels
			serviceName := container.Labels["com.docker.compose.service"]

			stackContainers = append(stackContainers, map[string]interface{}{
				"id":           container.ID,
				"name":         strings.TrimPrefix(container.Names[0], "/"),
				"service_name": serviceName,
				"image":        container.Image,
				"state":        container.State,
				"status":       container.Status,
				"created":      container.Created,
				"ports":        container.Ports,
			})
		}
	}

	logrus.Debugf("Found %d containers for stack %s", len(stackContainers), stackName)
	return stackContainers, nil
}
