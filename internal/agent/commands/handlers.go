package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/errdefs"
	"github.com/docker/go-connections/nat"
	"github.com/mikeysoft/flotilla/internal/agent/docker"
	"github.com/mikeysoft/flotilla/internal/shared/protocol"
	"github.com/sirupsen/logrus"
)

// Handler handles Docker commands from the server
type Handler struct {
	dockerClient  *docker.Client
	composeClient *docker.ComposeClient
	wsClient      WebSocketClient
}

const (
	maxConcurrentInspectJobs        = 4
	nameParameterRequiredMsg        = "name parameter required"
	containerIDParameterRequiredMsg = "container_id parameter required"
	imagesParameterArrayMsg         = "images parameter must be an array of strings"
)

var (
	errNameParameterRequired        = errors.New(nameParameterRequiredMsg)
	errContainerIDParameterRequired = errors.New(containerIDParameterRequiredMsg)
)

// handleGetDockerInfo returns docker version and host capacity
func (h *Handler) handleGetDockerInfo(ctx context.Context, commandID string) (*protocol.Message, error) {
	info, err := h.dockerClient.GetSystemInfo(ctx)
	if err != nil {
		return protocol.NewResponse(commandID, "error", nil, err), nil
	}
	return protocol.NewResponse(commandID, "success", map[string]any{
		"docker_version": info.DockerVersion,
		"ncpu":           info.NCPU,
		"mem_total":      info.MemTotal,
		"disk_total":     info.DiskTotal,
		"disk_free":      info.DiskFree,
	}, nil), nil
}

// WebSocketClient interface for sending log events
type WebSocketClient interface {
	SendLogEvent(containerID, data, stream string, timestamp time.Time) error
}

// NewHandler creates a new command handler
func NewHandler(dockerClient *docker.Client) *Handler {
	return &Handler{
		dockerClient:  dockerClient,
		composeClient: docker.NewComposeClient(dockerClient),
		wsClient:      nil, // Will be set later
	}
}

// SetWebSocketClient sets the WebSocket client for sending log events
func (h *Handler) SetWebSocketClient(wsClient WebSocketClient) {
	h.wsClient = wsClient
}

// HandleCommand processes a command and returns a response
func (h *Handler) HandleCommand(ctx context.Context, command *protocol.Message) (*protocol.Message, error) {
	cmd, err := command.GetCommand()
	if err != nil {
		return protocol.NewResponse(command.ID, "error", nil, err), nil
	}

	logrus.Debugf("Handling command: %s", cmd.Action)

	switch cmd.Action {
	case "list_containers":
		return h.handleListContainers(ctx, command.ID, cmd.Params)
	case "get_docker_info":
		return h.handleGetDockerInfo(ctx, command.ID)
	case "get_container":
		return h.handleGetContainer(ctx, command.ID, cmd.Params)
	case "create_container":
		return h.handleCreateContainer(ctx, command.ID, cmd.Params)
	case "start_container":
		return h.handleStartContainer(ctx, command.ID, cmd.Params)
	case "stop_container":
		return h.handleStopContainer(ctx, command.ID, cmd.Params)
	case "restart_container":
		return h.handleRestartContainer(ctx, command.ID, cmd.Params)
	case "remove_container":
		return h.handleRemoveContainer(ctx, command.ID, cmd.Params)
	case "list_images":
		return h.handleListImages(ctx, command.ID, cmd.Params)
	case "list_networks":
		return h.handleListNetworks(ctx, command.ID, cmd.Params)
	case "inspect_networks":
		return h.handleInspectNetworks(ctx, command.ID, cmd.Params)
	case "remove_networks":
		return h.handleRemoveNetworks(ctx, command.ID, cmd.Params)
	case "list_volumes":
		return h.handleListVolumes(ctx, command.ID, cmd.Params)
	case "inspect_volumes":
		return h.handleInspectVolumes(ctx, command.ID, cmd.Params)
	case "remove_volumes":
		return h.handleRemoveVolumes(ctx, command.ID, cmd.Params)
	case "remove_images":
		return h.handleRemoveImages(ctx, command.ID, cmd.Params)
	case "prune_dangling_images":
		return h.handlePruneDanglingImages(ctx, command.ID, cmd.Params)
	case "get_container_logs":
		return h.handleGetContainerLogs(ctx, command.ID, cmd.Params)
	case "stream_container_logs":
		return h.handleStreamContainerLogs(ctx, command.ID, cmd.Params)
	case "get_container_stats":
		return h.handleGetContainerStats(ctx, command.ID, cmd.Params)
	case "deploy_stack":
		return h.handleDeployStack(ctx, command.ID, cmd.Params)
	case "list_stacks":
		return h.handleListStacks(ctx, command.ID, cmd.Params)
	case "get_stack":
		return h.handleGetStack(ctx, command.ID, cmd.Params)
	case "update_stack":
		return h.handleUpdateStack(ctx, command.ID, cmd.Params)
	case "remove_stack":
		return h.handleRemoveStack(ctx, command.ID, cmd.Params)
	case "start_stack":
		return h.handleStartStack(ctx, command.ID, cmd.Params)
	case "stop_stack":
		return h.handleStopStack(ctx, command.ID, cmd.Params)
	case "restart_stack":
		return h.handleRestartStack(ctx, command.ID, cmd.Params)
	case "import_stack":
		return h.handleImportStack(ctx, command.ID, cmd.Params)
	case "get_stack_containers":
		return h.handleGetStackContainers(ctx, command.ID, cmd.Params)
	case "stack_container_action":
		return h.handleStackContainerAction(ctx, command.ID, cmd.Params)
	default:
		return protocol.NewResponse(command.ID, "error", nil, fmt.Errorf("unknown command: %s", cmd.Action)), nil
	}
}

// handleListContainers handles the list_containers command
func (h *Handler) handleListContainers(ctx context.Context, commandID string, params map[string]any) (*protocol.Message, error) {
	all := false
	if allParam, ok := params["all"].(bool); ok {
		all = allParam
	}

	containers, err := h.dockerClient.ListContainers(ctx, all)
	if err != nil {
		return protocol.NewResponse(commandID, "error", nil, err), nil
	}

	// Convert containers to a more friendly format
	containerList := make([]map[string]any, len(containers))
	for i, container := range containers {
		// Normalize status for frontend
		normalizedStatus := normalizeContainerStatus(container.Status, container.State)

		// Extract container name (Docker returns names as an array, take the first one)
		containerName := ""
		if len(container.Names) > 0 {
			// Remove leading slash from Docker name format
			containerName = strings.TrimPrefix(container.Names[0], "/")
		}
		if containerName == "" {
			// Fallback to short ID if no name
			containerName = container.ID[:12]
		}

		containerList[i] = map[string]any{
			"id":      container.ID,
			"name":    containerName,
			"names":   container.Names, // Keep original array for reference
			"image":   container.Image,
			"status":  normalizedStatus,
			"state":   container.State,
			"created": container.Created,
			"ports":   container.Ports,
			"labels":  container.Labels,
		}
	}

	return protocol.NewResponse(commandID, "success", map[string]any{
		"containers": containerList,
	}, nil), nil
}

// handleGetContainer handles the get_container command
func (h *Handler) handleGetContainer(ctx context.Context, commandID string, params map[string]any) (*protocol.Message, error) {
	containerID, ok := params["container_id"].(string)
	if !ok {
		return protocol.NewResponse(commandID, "error", nil, errContainerIDParameterRequired), nil
	}

	container, err := h.dockerClient.GetContainer(ctx, containerID)
	if err != nil {
		return protocol.NewResponse(commandID, "error", nil, err), nil
	}

	return protocol.NewResponse(commandID, "success", map[string]any{
		"container": container,
	}, nil), nil
}

// handleCreateContainer handles the create_container command
func (h *Handler) handleCreateContainer(ctx context.Context, commandID string, params map[string]any) (*protocol.Message, error) {
	// Parse required parameters
	image, ok := params["image"].(string)
	if !ok {
		return protocol.NewResponse(commandID, "error", nil, fmt.Errorf("image parameter required")), nil
	}

	name, ok := params["name"].(string)
	if !ok {
		return protocol.NewResponse(commandID, "error", nil, errNameParameterRequired), nil
	}

	// Parse optional parameters
	command := ""
	if cmd, ok := params["command"].(string); ok {
		command = cmd
	}

	env := []string{}
	if envVars, ok := params["env"].([]interface{}); ok {
		for _, envVar := range envVars {
			if envStr, ok := envVar.(string); ok {
				env = append(env, envStr)
			}
		}
	}

	ports := map[string]interface{}{}
	if portMappings, ok := params["ports"].(map[string]interface{}); ok {
		ports = portMappings
	}

	volumes := []string{}
	if volumeMappings, ok := params["volumes"].([]interface{}); ok {
		for _, volume := range volumeMappings {
			if volumeStr, ok := volume.(string); ok {
				volumes = append(volumes, volumeStr)
			}
		}
	}

	labels := map[string]string{}
	if labelMap, ok := params["labels"].(map[string]interface{}); ok {
		for k, v := range labelMap {
			if vStr, ok := v.(string); ok {
				labels[k] = vStr
			}
		}
	}

	// Parse restart policy
	restartPolicy := "no"
	if restart, ok := params["restart"].(string); ok {
		restartPolicy = restart
	}

	// Parse auto-start flag
	autoStart := true
	if start, ok := params["auto_start"].(bool); ok {
		autoStart = start
	}

	// Create container configuration
	containerConfig := &container.Config{
		Image:  image,
		Cmd:    strings.Fields(command),
		Env:    env,
		Labels: labels,
	}

	// Create host configuration
	hostConfig := &container.HostConfig{
		RestartPolicy: container.RestartPolicy{
			Name: restartPolicy,
		},
	}

	// Add port bindings
	if len(ports) > 0 {
		portBindings := make(nat.PortMap)
		exposedPorts := make(nat.PortSet)

		for containerPort, hostPort := range ports {
			port, err := nat.NewPort("tcp", containerPort)
			if err != nil {
				// If parsing fails, try with the port as-is
				port = nat.Port(containerPort)
			}
			exposedPorts[port] = struct{}{}
			portBindings[port] = []nat.PortBinding{
				{
					HostPort: fmt.Sprintf("%v", hostPort),
				},
			}
		}

		containerConfig.ExposedPorts = exposedPorts
		hostConfig.PortBindings = portBindings
	}

	// Add volume bindings
	if len(volumes) > 0 {
		hostConfig.Binds = volumes
	}

	// Create the container
	var response *container.CreateResponse
	var err error

	if autoStart {
		response, err = h.dockerClient.RunContainer(ctx, containerConfig, hostConfig, nil, nil, name)
	} else {
		response, err = h.dockerClient.CreateContainer(ctx, containerConfig, hostConfig, nil, nil, name)
	}

	if err != nil {
		return protocol.NewResponse(commandID, "error", nil, err), nil
	}

	return protocol.NewResponse(commandID, "success", map[string]any{
		"message":      "Container created successfully",
		"container_id": response.ID,
		"name":         name,
		"auto_started": autoStart,
	}, nil), nil
}

// handleStartContainer handles the start_container command
func (h *Handler) handleStartContainer(ctx context.Context, commandID string, params map[string]any) (*protocol.Message, error) {
	containerID, ok := params["container_id"].(string)
	if !ok {
		return protocol.NewResponse(commandID, "error", nil, errContainerIDParameterRequired), nil
	}

	err := h.dockerClient.StartContainer(ctx, containerID)
	if err != nil {
		return protocol.NewResponse(commandID, "error", nil, err), nil
	}

	return protocol.NewResponse(commandID, "success", map[string]any{
		"message":      "Container started successfully",
		"container_id": containerID,
	}, nil), nil
}

// handleStopContainer handles the stop_container command
func (h *Handler) handleStopContainer(ctx context.Context, commandID string, params map[string]any) (*protocol.Message, error) {
	containerID, ok := params["container_id"].(string)
	if !ok {
		return protocol.NewResponse(commandID, "error", nil, errContainerIDParameterRequired), nil
	}

	timeout := 30
	if timeoutParam, ok := params["timeout"].(float64); ok {
		timeout = int(timeoutParam)
	}

	err := h.dockerClient.StopContainer(ctx, containerID, &timeout)
	if err != nil {
		return protocol.NewResponse(commandID, "error", nil, err), nil
	}

	return protocol.NewResponse(commandID, "success", map[string]any{
		"message":      "Container stopped successfully",
		"container_id": containerID,
	}, nil), nil
}

// handleRestartContainer handles the restart_container command
func (h *Handler) handleRestartContainer(ctx context.Context, commandID string, params map[string]any) (*protocol.Message, error) {
	containerID, ok := params["container_id"].(string)
	if !ok {
		return protocol.NewResponse(commandID, "error", nil, errContainerIDParameterRequired), nil
	}

	timeout := 30
	if timeoutParam, ok := params["timeout"].(float64); ok {
		timeout = int(timeoutParam)
	}

	err := h.dockerClient.RestartContainer(ctx, containerID, &timeout)
	if err != nil {
		return protocol.NewResponse(commandID, "error", nil, err), nil
	}

	return protocol.NewResponse(commandID, "success", map[string]any{
		"message":      "Container restarted successfully",
		"container_id": containerID,
	}, nil), nil
}

// handleRemoveContainer handles the remove_container command
func (h *Handler) handleRemoveContainer(ctx context.Context, commandID string, params map[string]any) (*protocol.Message, error) {
	containerID, ok := params["container_id"].(string)
	if !ok {
		return protocol.NewResponse(commandID, "error", nil, errContainerIDParameterRequired), nil
	}

	force := false
	if forceParam, ok := params["force"].(bool); ok {
		force = forceParam
	}

	// If container is running and force is not set, stop it first
	if !force {
		container, err := h.dockerClient.GetContainer(ctx, containerID)
		if err == nil && container.State.Running {
			logrus.Infof("Container %s is running, stopping it before removal", containerID)
			err := h.dockerClient.StopContainer(ctx, containerID, nil)
			if err != nil {
				logrus.Warnf("Failed to stop container %s: %v, attempting force removal", containerID, err)
				force = true
			}
		}
	}

	err := h.dockerClient.RemoveContainer(ctx, containerID, force)
	if err != nil {
		return protocol.NewResponse(commandID, "error", nil, err), nil
	}

	return protocol.NewResponse(commandID, "success", map[string]any{
		"message":      "Container removed successfully",
		"container_id": containerID,
	}, nil), nil
}

// handleListImages handles the list_images command
func (h *Handler) handleListImages(ctx context.Context, commandID string, params map[string]any) (*protocol.Message, error) {
	images, err := h.dockerClient.ListImages(ctx)
	if err != nil {
		return protocol.NewResponse(commandID, "error", nil, err), nil
	}

	// Convert images to a more friendly format
	imageList := make([]map[string]any, len(images))
	for i, image := range images {
		shortID := strings.TrimPrefix(image.ID, "sha256:")
		if len(shortID) > 12 {
			shortID = shortID[:12]
		}

		primaryTag := "<none>:<none>"
		if len(image.RepoTags) > 0 {
			primaryTag = image.RepoTags[0]
		}

		status := "active"
		if len(image.RepoTags) == 0 {
			status = "dangling"
		}

		dangling := len(image.RepoTags) == 0
		danglingStr := "false"
		if dangling {
			danglingStr = "true"
		}

		joinedTags := strings.Join(image.RepoTags, ",")
		joinedDigests := strings.Join(image.RepoDigests, ",")

		imageList[i] = map[string]any{
			"id":           image.ID,
			"short_id":     shortID,
			"image":        primaryTag,
			"tag":          primaryTag,
			"repository":   primaryTag,
			"tags":         image.RepoTags,
			"digests":      image.RepoDigests,
			"tags_str":     joinedTags,
			"digests_str":  joinedDigests,
			"status":       status,
			"size":         image.Size,
			"created":      image.Created,
			"labels":       image.Labels,
			"containers":   image.Containers,
			"dangling":     dangling,
			"dangling_str": danglingStr,
			"shared_size":  image.SharedSize,
		}
	}

	return protocol.NewResponse(commandID, "success", map[string]any{
		"images": imageList,
	}, nil), nil
}

// handleListNetworks handles the list_networks command
func (h *Handler) handleListNetworks(ctx context.Context, commandID string, params map[string]any) (*protocol.Message, error) {
	networks, err := h.dockerClient.ListNetworks(ctx)
	if err != nil {
		return protocol.NewResponse(commandID, "error", nil, err), nil
	}

	var containerMeta map[string]containerMeta
	if containers, listErr := h.dockerClient.ListContainers(ctx, true); listErr == nil {
		containerMeta = buildContainerMetadata(containers)
	} else {
		logrus.Debugf("handleListNetworks: unable to list containers for metadata: %v", listErr)
	}

	networkList := make([]map[string]any, len(networks))
	for i, network := range networks {
		networkList[i] = serializeNetworkResource(network, containerMeta)
	}

	return protocol.NewResponse(commandID, "success", map[string]any{
		"networks": networkList,
	}, nil), nil
}

// handleListVolumes handles the list_volumes command
func (h *Handler) handleListVolumes(ctx context.Context, commandID string, params map[string]any) (*protocol.Message, error) {
	volumes, err := h.dockerClient.ListVolumes(ctx)
	if err != nil {
		return protocol.NewResponse(commandID, "error", nil, err), nil
	}

	volumeConsumers := map[string][]map[string]any{}
	if containers, listErr := h.dockerClient.ListContainers(ctx, true); listErr == nil {
		containerMeta := buildContainerMetadata(containers)
		volumeConsumers = collectVolumeConsumers(containers, containerMeta)
	} else {
		logrus.Debugf("handleListVolumes: unable to list containers for metadata: %v", listErr)
	}

	volumeList := make([]map[string]any, len(volumes))
	for i, vol := range volumes {
		volumeList[i] = serializeVolumeResource(vol, volumeConsumers[vol.Name])
	}

	return protocol.NewResponse(commandID, "success", map[string]any{
		"volumes": volumeList,
	}, nil), nil
}

// handleInspectNetworks performs docker network inspect calls in batches.
func (h *Handler) handleInspectNetworks(ctx context.Context, commandID string, params map[string]any) (*protocol.Message, error) {
	ids, err := extractStringSlice(params, "ids")
	if err != nil {
		return protocol.NewResponse(commandID, "error", nil, err), nil
	}
	if len(ids) == 0 {
		return protocol.NewResponse(commandID, "success", map[string]any{
			"networks": []map[string]any{},
		}, nil), nil
	}

	containerMeta := map[string]containerMeta{}
	if containers, listErr := h.dockerClient.ListContainers(ctx, true); listErr == nil {
		containerMeta = buildContainerMetadata(containers)
	} else {
		logrus.Debugf("handleInspectNetworks: unable to list containers for metadata: %v", listErr)
	}

	type inspectResult struct {
		index int
		data  map[string]any
		err   error
		id    string
	}

	results := make([]map[string]any, len(ids))
	errors := make([]map[string]any, 0)

	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrentInspectJobs)
	resultCh := make(chan inspectResult, len(ids))

	for idx, id := range ids {
		wg.Add(1)
		go func(index int, networkID string) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				resultCh <- inspectResult{index: index, err: ctx.Err(), id: networkID}
				return
			}
			defer func() { <-sem }()

			network, inspectErr := h.dockerClient.InspectNetwork(ctx, networkID)
			if inspectErr != nil {
				resultCh <- inspectResult{index: index, err: inspectErr, id: networkID}
				return
			}

			payload := normalizeNetworkInspect(*network, containerMeta)
			resultCh <- inspectResult{index: index, data: payload, id: networkID}
		}(idx, id)
	}

	wg.Wait()
	close(resultCh)

	for res := range resultCh {
		if res.err != nil {
			errors = append(errors, map[string]any{
				"id":    res.id,
				"error": res.err.Error(),
			})
			continue
		}
		results[res.index] = res.data
	}

	response := map[string]any{
		"networks": results,
	}
	if len(errors) > 0 {
		response["errors"] = errors
	}

	return protocol.NewResponse(commandID, "success", response, nil), nil
}

// handleRemoveNetworks removes one or more docker networks on the host.
func (h *Handler) handleRemoveNetworks(ctx context.Context, commandID string, params map[string]any) (*protocol.Message, error) {
	ids, err := extractStringSlice(params, "ids")
	if err != nil {
		return protocol.NewResponse(commandID, "error", nil, err), nil
	}
	if len(ids) == 0 {
		return protocol.NewResponse(commandID, "error", nil, errors.New("ids must not be empty")), nil
	}

	force := false
	if val, ok := params["force"].(bool); ok {
		force = val
	}

	removed := make([]string, 0, len(ids))
	conflicts := make([]protocol.ResourceRemovalConflict, 0)
	unexpectedErrors := make([]protocol.ResourceRemovalError, 0)

	for _, id := range ids {
		if err := h.dockerClient.RemoveNetwork(ctx, id, force); err != nil {
			logrus.WithError(err).Warnf("handleRemoveNetworks: failed to remove network %s", id)
			conflict, removalErr := h.resolveNetworkRemovalError(ctx, id, err)
			if conflict != nil {
				conflicts = append(conflicts, *conflict)
			}
			if removalErr != nil {
				unexpectedErrors = append(unexpectedErrors, *removalErr)
			}
			continue
		}
		removed = append(removed, id)
	}

	payload := map[string]any{
		"removed": removed,
	}
	if len(conflicts) > 0 {
		payload["conflicts"] = conflicts
	}
	if len(unexpectedErrors) > 0 {
		payload["errors"] = unexpectedErrors
	}

	return protocol.NewResponse(commandID, "success", payload, nil), nil
}

// handleInspectVolumes performs docker volume inspect calls in batches.
func (h *Handler) handleInspectVolumes(ctx context.Context, commandID string, params map[string]any) (*protocol.Message, error) {
	ids, err := extractStringSlice(params, "ids")
	// Support legacy key "names"
	if err != nil || len(ids) == 0 {
		if fallback, fallbackErr := extractStringSlice(params, "names"); fallbackErr == nil {
			ids = fallback
			err = nil
		}
	}
	if err != nil {
		return protocol.NewResponse(commandID, "error", nil, err), nil
	}
	if len(ids) == 0 {
		return protocol.NewResponse(commandID, "success", map[string]any{
			"volumes": []map[string]any{},
		}, nil), nil
	}

	volumeConsumers := map[string][]map[string]any{}
	if containers, listErr := h.dockerClient.ListContainers(ctx, true); listErr == nil {
		meta := buildContainerMetadata(containers)
		volumeConsumers = collectVolumeConsumers(containers, meta)
	} else {
		logrus.Debugf("handleInspectVolumes: unable to list containers for metadata: %v", listErr)
	}

	type inspectResult struct {
		index int
		data  map[string]any
		err   error
		id    string
	}

	results := make([]map[string]any, len(ids))
	errors := make([]map[string]any, 0)

	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrentInspectJobs)
	resultCh := make(chan inspectResult, len(ids))

	for idx, name := range ids {
		wg.Add(1)
		go func(index int, volumeName string) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				resultCh <- inspectResult{index: index, err: ctx.Err(), id: volumeName}
				return
			}
			defer func() { <-sem }()

			volumeInfo, inspectErr := h.dockerClient.InspectVolume(ctx, volumeName)
			if inspectErr != nil {
				resultCh <- inspectResult{index: index, err: inspectErr, id: volumeName}
				return
			}

			payload := normalizeVolumeInspect(volumeInfo, volumeConsumers[volumeName])
			resultCh <- inspectResult{index: index, data: payload, id: volumeName}
		}(idx, name)
	}

	wg.Wait()
	close(resultCh)

	for res := range resultCh {
		if res.err != nil {
			errors = append(errors, map[string]any{
				"id":    res.id,
				"error": res.err.Error(),
			})
			continue
		}
		results[res.index] = res.data
	}

	response := map[string]any{
		"volumes": results,
	}
	if len(errors) > 0 {
		response["errors"] = errors
	}

	return protocol.NewResponse(commandID, "success", response, nil), nil
}

// handleRemoveVolumes removes one or more docker volumes on the host.
func (h *Handler) handleRemoveVolumes(ctx context.Context, commandID string, params map[string]any) (*protocol.Message, error) {
	names, err := extractStringSlice(params, "names")
	if err != nil {
		return protocol.NewResponse(commandID, "error", nil, err), nil
	}
	if len(names) == 0 {
		return protocol.NewResponse(commandID, "error", nil, errors.New("names must not be empty")), nil
	}

	force := false
	if val, ok := params["force"].(bool); ok {
		force = val
	}

	removed := make([]string, 0, len(names))
	conflicts := make([]protocol.ResourceRemovalConflict, 0)
	unexpectedErrors := make([]protocol.ResourceRemovalError, 0)

	for _, name := range names {
		if err := h.dockerClient.RemoveVolume(ctx, name, force); err != nil {
			logrus.WithError(err).Warnf("handleRemoveVolumes: failed to remove volume %s", name)
			conflict, removalErr := h.resolveVolumeRemovalError(ctx, name, err)
			if conflict != nil {
				conflicts = append(conflicts, *conflict)
			}
			if removalErr != nil {
				unexpectedErrors = append(unexpectedErrors, *removalErr)
			}
			continue
		}
		removed = append(removed, name)
	}

	payload := map[string]any{
		"removed": removed,
	}
	if len(conflicts) > 0 {
		payload["conflicts"] = conflicts
	}
	if len(unexpectedErrors) > 0 {
		payload["errors"] = unexpectedErrors
	}

	return protocol.NewResponse(commandID, "success", payload, nil), nil
}

type containerMeta struct {
	Name    string
	Stack   string
	Service string
}

func buildContainerMetadata(containers []types.Container) map[string]containerMeta {
	meta := make(map[string]containerMeta, len(containers)*2)
	for _, ctr := range containers {
		name := ""
		if len(ctr.Names) > 0 {
			name = strings.TrimPrefix(ctr.Names[0], "/")
		} else if len(ctr.ID) >= 12 {
			name = ctr.ID[:12]
		}

		info := containerMeta{
			Name:    name,
			Stack:   ctr.Labels["com.docker.compose.project"],
			Service: ctr.Labels["com.docker.compose.service"],
		}

		meta[ctr.ID] = info
		if len(ctr.ID) >= 12 {
			meta[ctr.ID[:12]] = info
		}
	}
	return meta
}

func collectVolumeConsumers(containers []types.Container, meta map[string]containerMeta) map[string][]map[string]any {
	consumers := make(map[string][]map[string]any)
	for _, ctr := range containers {
		info := meta[ctr.ID]
		if info.Name == "" {
			if len(ctr.Names) > 0 {
				info.Name = strings.TrimPrefix(ctr.Names[0], "/")
			} else if len(ctr.ID) >= 12 {
				info.Name = ctr.ID[:12]
			}
		}

		for _, mount := range ctr.Mounts {
			if mount.Name == "" {
				continue
			}
			consumer := map[string]any{
				"id":          ctr.ID,
				"name":        info.Name,
				"stack":       info.Stack,
				"service":     info.Service,
				"destination": mount.Destination,
				"mode":        mount.Mode,
				"rw":          mount.RW,
			}
			consumers[mount.Name] = append(consumers[mount.Name], consumer)
		}
	}
	return consumers
}

func serializeNetworkResource(network types.NetworkResource, meta map[string]containerMeta) map[string]any {
	payload := normalizeNetworkInspect(network, meta)
	delete(payload, "raw")
	return payload
}

func serializeVolumeResource(vol *volume.Volume, consumers []map[string]any) map[string]any {
	payload := normalizeVolumeInspect(vol, consumers)
	delete(payload, "raw")
	return payload
}

func normalizeNetworkInspect(network types.NetworkResource, meta map[string]containerMeta) map[string]any {
	attachments := make([]map[string]any, 0, len(network.Containers))
	stackSet := make(map[string]struct{})

	for id, endpoint := range network.Containers {
		info, ok := meta[id]
		if !ok {
			info, ok = meta[strings.TrimPrefix(id, "/")]
		}

		attachment := map[string]any{
			"id":   id,
			"ipv4": endpoint.IPv4Address,
			"ipv6": endpoint.IPv6Address,
			"mac":  endpoint.MacAddress,
		}

		if ok {
			attachment["name"] = info.Name
			if info.Stack != "" {
				attachment["stack"] = info.Stack
				stackSet[info.Stack] = struct{}{}
			}
			if info.Service != "" {
				attachment["service"] = info.Service
			}
		} else if endpoint.Name != "" {
			attachment["name"] = endpoint.Name
		}

		attachments = append(attachments, attachment)
	}

	stacks := make([]string, 0, len(stackSet))
	for stack := range stackSet {
		stacks = append(stacks, stack)
	}
	sort.Strings(stacks)

	payload := map[string]any{
		"id":                network.ID,
		"name":              network.Name,
		"driver":            network.Driver,
		"scope":             network.Scope,
		"internal":          network.Internal,
		"attachable":        network.Attachable,
		"ingress":           network.Ingress,
		"enable_ipv6":       network.EnableIPv6,
		"labels":            network.Labels,
		"options":           network.Options,
		"containers":        len(attachments),
		"ipam":              network.IPAM,
		"config_only":       network.ConfigOnly,
		"config_from":       network.ConfigFrom,
		"containers_detail": attachments,
		"stacks":            stacks,
		"raw":               serializeToMap(network),
		"connected":         len(attachments) > 0,
	}

	if !network.Created.IsZero() {
		payload["created"] = network.Created.Format(time.RFC3339)
	}

	return payload
}

func normalizeVolumeInspect(vol *volume.Volume, consumers []map[string]any) map[string]any {
	var size, refCount int64
	if vol.UsageData != nil {
		size = vol.UsageData.Size
		refCount = vol.UsageData.RefCount
	}

	stackSet := make(map[string]struct{})
	for _, consumer := range consumers {
		if stack, ok := consumer["stack"].(string); ok && stack != "" {
			stackSet[stack] = struct{}{}
		}
	}

	stacks := make([]string, 0, len(stackSet))
	for stack := range stackSet {
		stacks = append(stacks, stack)
	}
	sort.Strings(stacks)

	payload := map[string]any{
		"name":              vol.Name,
		"driver":            vol.Driver,
		"mountpoint":        vol.Mountpoint,
		"created":           vol.CreatedAt,
		"labels":            vol.Labels,
		"scope":             vol.Scope,
		"status":            vol.Status,
		"options":           vol.Options,
		"size_bytes":        size,
		"ref_count":         refCount,
		"containers_detail": consumers,
		"containers":        len(consumers),
		"stacks":            stacks,
		"raw":               serializeToMap(vol),
	}

	return payload
}

func serializeToMap(input any) map[string]any {
	if input == nil {
		return map[string]any{}
	}

	data, err := json.Marshal(input)
	if err != nil {
		logrus.Debugf("serializeToMap: failed to marshal: %v", err)
		return map[string]any{}
	}

	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		logrus.Debugf("serializeToMap: failed to unmarshal: %v", err)
		return map[string]any{}
	}

	return out
}

func extractStringSlice(params map[string]any, key string) ([]string, error) {
	value, ok := params[key]
	if !ok {
		return nil, nil
	}

	switch v := value.(type) {
	case []string:
		return v, nil
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			str, strOK := item.(string)
			if !strOK {
				return nil, fmt.Errorf("invalid value for %s: must be array of strings", key)
			}
			out = append(out, str)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("invalid value for %s: expected array of strings", key)
	}
}

// handleRemoveImages handles removal of one or more images
func (h *Handler) handleRemoveImages(ctx context.Context, commandID string, params map[string]any) (*protocol.Message, error) {
	rawList, ok := params["images"]
	if !ok {
		return protocol.NewResponse(commandID, "error", nil, fmt.Errorf("images parameter required")), nil
	}

	imageRefs, err := normalizeStringList(rawList)
	if err != nil {
		return protocol.NewResponse(commandID, "error", nil, err), nil
	}

	if len(imageRefs) == 0 {
		return protocol.NewResponse(commandID, "error", nil, fmt.Errorf("images parameter must include at least one image reference")), nil
	}

	force := boolParam(params, "force", false)
	removed, conflicts, removeErrors := h.removeImagesByReference(ctx, imageRefs, force)

	payload := map[string]any{
		"removed": removed,
	}
	if len(conflicts) > 0 {
		payload["conflicts"] = conflicts
	}
	if len(removeErrors) > 0 {
		payload["errors"] = removeErrors
	}

	return protocol.NewResponse(commandID, "success", payload, nil), nil
}

func (h *Handler) removeImagesByReference(ctx context.Context, refs []string, force bool) ([]string, []protocol.ResourceRemovalConflict, []protocol.ResourceRemovalError) {
	removed := make([]string, 0, len(refs))
	conflicts := make([]protocol.ResourceRemovalConflict, 0)
	errorsList := make([]protocol.ResourceRemovalError, 0)

	for _, ref := range refs {
		if ref == "" {
			continue
		}
		if _, err := h.dockerClient.RemoveImage(ctx, ref, force); err != nil {
			logrus.Errorf("Failed to remove image %s: %v", ref, err)
			conflict, removalErr := h.resolveImageRemovalError(ctx, ref, err)
			if conflict != nil {
				conflicts = append(conflicts, *conflict)
			}
			if removalErr != nil {
				errorsList = append(errorsList, *removalErr)
			}
			continue
		}
		removed = append(removed, ref)
	}

	return removed, conflicts, errorsList
}

func (h *Handler) resolveImageRemovalError(ctx context.Context, imageRef string, err error) (*protocol.ResourceRemovalConflict, *protocol.ResourceRemovalError) {
	if !errdefs.IsConflict(err) {
		return nil, &protocol.ResourceRemovalError{
			ResourceType: protocol.ResourceTypeImage,
			ResourceName: imageRef,
			Message:      err.Error(),
		}
	}

	imageInspect, inspectErr := h.dockerClient.InspectImage(ctx, imageRef)
	if inspectErr != nil {
		return nil, &protocol.ResourceRemovalError{
			ResourceType: protocol.ResourceTypeImage,
			ResourceName: imageRef,
			Message:      inspectErr.Error(),
		}
	}

	blockers := make([]protocol.ResourceRemovalBlocker, 0)
	tagCount := 0
	for _, tag := range imageInspect.RepoTags {
		if tag == "" || tag == "<none>:<none>" {
			continue
		}
		tagCount++
		blockers = append(blockers, protocol.ResourceRemovalBlocker{
			Kind: "image_tag",
			Name: tag,
		})
	}

	containerBlockers := make([]protocol.ResourceRemovalBlocker, 0)
	if containers, listErr := h.dockerClient.ListContainersByImage(ctx, []string{imageInspect.ID, imageRef}); listErr == nil {
		for _, ctr := range containers {
			details := map[string]string{
				"state": ctr.State,
			}
			blocker := protocol.ResourceRemovalBlocker{
				Kind:    "container",
				ID:      ctr.ID,
				Name:    containerDisplayName(ctr),
				Stack:   ctr.Labels["com.docker.compose.project"],
				Details: sanitizeDetails(details),
			}
			containerBlockers = append(containerBlockers, blocker)
		}
	} else {
		logrus.Debugf("resolveImageRemovalError: unable to list containers for image %s: %v", imageRef, listErr)
	}

	blockers = append(blockers, containerBlockers...)

	reasonParts := make([]string, 0)
	if tagCount > 0 {
		reasonParts = append(reasonParts, fmt.Sprintf("%d tag(s) still reference the image", tagCount))
	}
	if len(containerBlockers) > 0 {
		reasonParts = append(reasonParts, fmt.Sprintf("%d container(s) currently use the image", len(containerBlockers)))
	}
	if len(reasonParts) == 0 {
		reasonParts = append(reasonParts, "Docker reported a conflict while removing the image")
	}

	resourceName := imageRef
	if resourceName == "" {
		if len(imageInspect.RepoTags) > 0 {
			resourceName = imageInspect.RepoTags[0]
		} else if len(imageInspect.RepoDigests) > 0 {
			resourceName = imageInspect.RepoDigests[0]
		}
	}

	conflict := &protocol.ResourceRemovalConflict{
		ResourceType:   protocol.ResourceTypeImage,
		ResourceID:     imageInspect.ID,
		ResourceName:   resourceName,
		Reason:         strings.Join(reasonParts, "; "),
		Blockers:       blockers,
		ForceSupported: true,
		OriginalError:  err.Error(),
	}

	return conflict, nil
}

func (h *Handler) resolveVolumeRemovalError(ctx context.Context, volumeName string, err error) (*protocol.ResourceRemovalConflict, *protocol.ResourceRemovalError) {
	if !errdefs.IsConflict(err) {
		return nil, &protocol.ResourceRemovalError{
			ResourceType: protocol.ResourceTypeVolume,
			ResourceName: volumeName,
			Message:      err.Error(),
		}
	}

	volumeInspect, inspectErr := h.dockerClient.InspectVolume(ctx, volumeName)
	if inspectErr != nil {
		return nil, &protocol.ResourceRemovalError{
			ResourceType: protocol.ResourceTypeVolume,
			ResourceName: volumeName,
			Message:      inspectErr.Error(),
		}
	}

	blockers := make([]protocol.ResourceRemovalBlocker, 0)
	containerCount := 0

	if containers, listErr := h.dockerClient.ListContainers(ctx, true); listErr == nil {
		for _, ctr := range containers {
			mountDetails := map[string]string{}
			matched := false
			for _, mount := range ctr.Mounts {
				if mount.Type != "volume" {
					continue
				}
				if mount.Name == volumeInspect.Name || mount.Name == volumeName {
					matched = true
					if mount.Destination != "" {
						mountDetails["mount_point"] = mount.Destination
					}
					if mount.Driver != "" {
						mountDetails["driver"] = mount.Driver
					}
					break
				}
			}
			if !matched {
				continue
			}

			containerCount++
			blockers = append(blockers, protocol.ResourceRemovalBlocker{
				Kind:    "container_mount",
				ID:      ctr.ID,
				Name:    containerDisplayName(ctr),
				Stack:   ctr.Labels["com.docker.compose.project"],
				Details: sanitizeDetails(mountDetails),
			})
		}
	} else {
		logrus.Debugf("resolveVolumeRemovalError: unable to list containers for volume %s: %v", volumeName, listErr)
	}

	reasonParts := make([]string, 0)
	if containerCount > 0 {
		reasonParts = append(reasonParts, fmt.Sprintf("Volume is currently mounted by %d container(s)", containerCount))
	}
	if volumeInspect.Mountpoint != "" && containerCount == 0 {
		reasonParts = append(reasonParts, "Docker reported the volume is still in use")
	}
	if len(reasonParts) == 0 {
		reasonParts = append(reasonParts, "Docker reported a conflict while removing the volume")
	}

	conflict := &protocol.ResourceRemovalConflict{
		ResourceType:   protocol.ResourceTypeVolume,
		ResourceID:     volumeInspect.Name,
		ResourceName:   volumeInspect.Name,
		Reason:         strings.Join(reasonParts, "; "),
		Blockers:       blockers,
		ForceSupported: true,
		OriginalError:  err.Error(),
	}

	return conflict, nil
}

func (h *Handler) resolveNetworkRemovalError(ctx context.Context, networkID string, err error) (*protocol.ResourceRemovalConflict, *protocol.ResourceRemovalError) {
	if !errdefs.IsConflict(err) {
		return nil, &protocol.ResourceRemovalError{
			ResourceType: protocol.ResourceTypeNetwork,
			ResourceName: networkID,
			Message:      err.Error(),
		}
	}

	networkInspect, inspectErr := h.dockerClient.InspectNetwork(ctx, networkID)
	if inspectErr != nil {
		return nil, &protocol.ResourceRemovalError{
			ResourceType: protocol.ResourceTypeNetwork,
			ResourceName: networkID,
			Message:      inspectErr.Error(),
		}
	}

	blockers := make([]protocol.ResourceRemovalBlocker, 0, len(networkInspect.Containers))
	containerCount := 0

	containerMeta := map[string]containerMeta{}
	if containers, listErr := h.dockerClient.ListContainers(ctx, true); listErr == nil {
		containerMeta = buildContainerMetadata(containers)
	} else {
		logrus.Debugf("resolveNetworkRemovalError: unable to list containers for network %s: %v", networkID, listErr)
	}

	for containerID, endpoint := range networkInspect.Containers {
		containerCount++
		meta := containerMeta[containerID]
		details := map[string]string{
			"ipv4": strings.TrimSuffix(endpoint.IPv4Address, "/"),
			"ipv6": strings.TrimSuffix(endpoint.IPv6Address, "/"),
		}

		name := endpoint.Name
		if name == "" {
			name = meta.Name
		}

		blockers = append(blockers, protocol.ResourceRemovalBlocker{
			Kind:    "container_attachment",
			ID:      containerID,
			Name:    name,
			Stack:   meta.Stack,
			Details: sanitizeDetails(details),
		})
	}

	reasonParts := make([]string, 0)
	if containerCount > 0 {
		reasonParts = append(reasonParts, fmt.Sprintf("Network has %d container attachment(s)", containerCount))
	}
	if len(reasonParts) == 0 {
		reasonParts = append(reasonParts, "Docker reported a conflict while removing the network")
	}

	resourceName := networkInspect.Name
	if resourceName == "" {
		resourceName = networkID
	}

	conflict := &protocol.ResourceRemovalConflict{
		ResourceType:   protocol.ResourceTypeNetwork,
		ResourceID:     networkInspect.ID,
		ResourceName:   resourceName,
		Reason:         strings.Join(reasonParts, "; "),
		Blockers:       blockers,
		ForceSupported: false,
		OriginalError:  err.Error(),
	}

	return conflict, nil
}

func sanitizeDetails(details map[string]string) map[string]string {
	if len(details) == 0 {
		return nil
	}
	for key, value := range details {
		if value == "" {
			delete(details, key)
		}
	}
	if len(details) == 0 {
		return nil
	}
	return details
}

func containerDisplayName(ctr types.Container) string {
	if len(ctr.Names) > 0 {
		name := strings.TrimPrefix(ctr.Names[0], "/")
		if name != "" {
			return name
		}
	}

	if len(ctr.ID) >= 12 {
		return ctr.ID[:12]
	}

	return ctr.ID
}

func boolParam(params map[string]any, key string, defaultValue bool) bool {
	if value, ok := params[key].(bool); ok {
		return value
	}
	return defaultValue
}

func normalizeStringList(value any) ([]string, error) {
	switch list := value.(type) {
	case []string:
		return filterEmptyStrings(list), nil
	case []interface{}:
		result := make([]string, 0, len(list))
		for _, item := range list {
			str, ok := item.(string)
			if !ok {
				return nil, errors.New(imagesParameterArrayMsg)
			}
			if str != "" {
				result = append(result, str)
			}
		}
		return result, nil
	default:
		return nil, errors.New(imagesParameterArrayMsg)
	}
}

func filterEmptyStrings(values []string) []string {
	if len(values) == 0 {
		return values
	}
	result := make([]string, 0, len(values))
	for _, v := range values {
		if v != "" {
			result = append(result, v)
		}
	}
	return result
}

// handlePruneDanglingImages removes all dangling images
func (h *Handler) handlePruneDanglingImages(ctx context.Context, commandID string, params map[string]any) (*protocol.Message, error) {
	report, err := h.dockerClient.PruneDanglingImages(ctx)
	if err != nil {
		return protocol.NewResponse(commandID, "error", nil, err), nil
	}

	removed := make([]string, 0, len(report.ImagesDeleted))
	for _, item := range report.ImagesDeleted {
		if item.Deleted != "" {
			removed = append(removed, item.Deleted)
		} else if item.Untagged != "" {
			removed = append(removed, item.Untagged)
		}
	}

	return protocol.NewResponse(commandID, "success", map[string]any{
		"removed":         removed,
		"space_reclaimed": report.SpaceReclaimed,
	}, nil), nil
}

// handleGetContainerLogs handles the get_container_logs command
func (h *Handler) handleGetContainerLogs(ctx context.Context, commandID string, params map[string]any) (*protocol.Message, error) {
	containerID, ok := params["container_id"].(string)
	if !ok {
		return protocol.NewResponse(commandID, "error", nil, errContainerIDParameterRequired), nil
	}

	// Parse log options
	options := map[string]any{
		"follow":     false,
		"tail":       "100",
		"timestamps": true,
	}

	if follow, ok := params["follow"].(bool); ok {
		options["follow"] = follow
	}
	if tail, ok := params["tail"].(string); ok {
		options["tail"] = tail
	}
	if timestamps, ok := params["timestamps"].(bool); ok {
		options["timestamps"] = timestamps
	}

	// Convert options to Docker types
	dockerOptions := map[string]any{
		"ShowStdout": true,
		"ShowStderr": true,
		"Timestamps": options["timestamps"].(bool),
	}

	if tailStr, ok := options["tail"].(string); ok {
		if tailNum, err := strconv.Atoi(tailStr); err == nil {
			dockerOptions["Tail"] = strconv.Itoa(tailNum)
		}
	}

	logs, err := h.dockerClient.GetContainerLogs(ctx, containerID, dockerOptions)
	if err != nil {
		return protocol.NewResponse(commandID, "error", nil, err), nil
	}

	return protocol.NewResponse(commandID, "success", map[string]any{
		"logs":         string(logs),
		"container_id": containerID,
	}, nil), nil
}

// handleStreamContainerLogs handles the stream_container_logs command
func (h *Handler) handleStreamContainerLogs(ctx context.Context, commandID string, params map[string]any) (*protocol.Message, error) {
	containerID, ok := params["container_id"].(string)
	if !ok {
		return protocol.NewResponse(commandID, "error", nil, errContainerIDParameterRequired), nil
	}

	// Parse log options
	options := docker.LogOptions{
		Follow:     false,
		Tail:       "100",
		Timestamps: true,
	}

	if follow, ok := params["follow"].(bool); ok {
		options.Follow = follow
	}
	if tail, ok := params["tail"].(string); ok {
		options.Tail = tail
	}
	if timestamps, ok := params["timestamps"].(bool); ok {
		options.Timestamps = timestamps
	}
	if since, ok := params["since"].(string); ok {
		options.Since = since
	}
	if until, ok := params["until"].(string); ok {
		options.Until = until
	}

	// Create log streamer
	logStreamer := docker.NewLogStreamer(h.dockerClient.GetDockerClient())

	// Start streaming logs in a goroutine
	go func() {
		streamCtx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Send log chunks via WebSocket to server
		err := logStreamer.StreamLogs(streamCtx, containerID, options, func(chunk docker.LogChunk) error {
			// Send log event via WebSocket if available
			if h.wsClient != nil {
				if err := h.wsClient.SendLogEvent(containerID, chunk.Data, chunk.Stream, chunk.Timestamp); err != nil {
					logrus.Errorf("Failed to send log event: %v", err)
				}
			} else {
				logrus.Debugf("Log chunk for container %s (no WebSocket client): %s", containerID, chunk.Data)
			}

			return nil
		})

		if err != nil {
			logrus.Errorf("Log streaming error for container %s: %v", containerID, err)
		}
	}()

	logrus.Infof("Started log stream for container %s", containerID)

	return protocol.NewResponse(commandID, "success", map[string]any{
		"message":      "Log streaming started",
		"container_id": containerID,
		"stream_id":    fmt.Sprintf("%s-%d", containerID, time.Now().Unix()),
	}, nil), nil
}

// handleGetContainerStats handles the get_container_stats command
func (h *Handler) handleGetContainerStats(ctx context.Context, commandID string, params map[string]any) (*protocol.Message, error) {
	containerID, ok := params["container_id"].(string)
	if !ok {
		return protocol.NewResponse(commandID, "error", nil, errContainerIDParameterRequired), nil
	}

	stats, err := h.dockerClient.GetContainerStats(ctx, containerID)
	if err != nil {
		return protocol.NewResponse(commandID, "error", nil, err), nil
	}

	return protocol.NewResponse(commandID, "success", map[string]any{
		"stats":        stats,
		"container_id": containerID,
	}, nil), nil
}

// normalizeContainerStatus converts Docker status strings to frontend-friendly values
func normalizeContainerStatus(status, state string) string {
	// Docker status can be things like "Up 2 hours", "Exited (0) 2 hours ago", etc.
	// We need to normalize these to simple values the frontend expects

	// Check if container is running
	if strings.HasPrefix(status, "Up") {
		return "running"
	}

	// Check if container is paused
	if state == "paused" {
		return "paused"
	}

	// Check if container is restarting
	if state == "restarting" {
		return "restarting"
	}

	// Check if container exited
	if strings.HasPrefix(status, "Exited") {
		return "stopped"
	}

	// Check if container is created but not started
	if state == "created" {
		return "stopped"
	}

	// Check if container is dead
	if state == "dead" {
		return "error"
	}

	// Default to stopped for unknown states
	return "stopped"
}

// handleDeployStack handles the deploy_stack command
func (h *Handler) handleDeployStack(ctx context.Context, commandID string, params map[string]any) (*protocol.Message, error) {
	name, ok := params["name"].(string)
	if !ok {
		return protocol.NewResponse(commandID, "error", nil, errNameParameterRequired), nil
	}

	compose, ok := params["compose"].(string)
	if !ok {
		return protocol.NewResponse(commandID, "error", nil, fmt.Errorf("compose parameter required")), nil
	}

	envVars := map[string]interface{}{}
	if envVarsParam, ok := params["env_vars"].(map[string]interface{}); ok {
		envVars = envVarsParam
	}

	err := h.composeClient.DeployStack(ctx, name, compose, envVars)
	if err != nil {
		return protocol.NewResponse(commandID, "error", nil, err), nil
	}

	return protocol.NewResponse(commandID, "success", map[string]any{
		"message": fmt.Sprintf("Stack '%s' deployed successfully", name),
		"name":    name,
	}, nil), nil
}

// handleListStacks handles the list_stacks command
func (h *Handler) handleListStacks(ctx context.Context, commandID string, _ map[string]any) (*protocol.Message, error) {
	stacks, err := h.composeClient.ListStacks(ctx)
	if err != nil {
		return protocol.NewResponse(commandID, "error", nil, err), nil
	}

	return protocol.NewResponse(commandID, "success", map[string]any{
		"stacks": stacks,
	}, nil), nil
}

// handleGetStack handles the get_stack command
func (h *Handler) handleGetStack(ctx context.Context, commandID string, params map[string]any) (*protocol.Message, error) {
	name, ok := params["name"].(string)
	if !ok {
		return protocol.NewResponse(commandID, "error", nil, errNameParameterRequired), nil
	}

	stack, err := h.composeClient.GetStack(ctx, name)
	if err != nil {
		return protocol.NewResponse(commandID, "error", nil, err), nil
	}

	return protocol.NewResponse(commandID, "success", map[string]any{
		"stack": stack,
	}, nil), nil
}

// handleUpdateStack handles the update_stack command
func (h *Handler) handleUpdateStack(ctx context.Context, commandID string, params map[string]any) (*protocol.Message, error) {
	name, ok := params["name"].(string)
	if !ok {
		return protocol.NewResponse(commandID, "error", nil, errNameParameterRequired), nil
	}

	compose, ok := params["compose"].(string)
	if !ok {
		return protocol.NewResponse(commandID, "error", nil, fmt.Errorf("compose parameter required")), nil
	}

	envVars := map[string]interface{}{}
	if envVarsParam, ok := params["env_vars"].(map[string]interface{}); ok {
		envVars = envVarsParam
	}

	err := h.composeClient.UpdateStack(ctx, name, compose, envVars)
	if err != nil {
		return protocol.NewResponse(commandID, "error", nil, err), nil
	}

	return protocol.NewResponse(commandID, "success", map[string]any{
		"message": fmt.Sprintf("Stack '%s' updated successfully", name),
		"name":    name,
	}, nil), nil
}

// handleRemoveStack handles the remove_stack command
func (h *Handler) handleRemoveStack(ctx context.Context, commandID string, params map[string]any) (*protocol.Message, error) {
	name, ok := params["name"].(string)
	if !ok {
		return protocol.NewResponse(commandID, "error", nil, errNameParameterRequired), nil
	}

	err := h.composeClient.RemoveStack(ctx, name)
	if err != nil {
		return protocol.NewResponse(commandID, "error", nil, err), nil
	}

	return protocol.NewResponse(commandID, "success", map[string]any{
		"message": fmt.Sprintf("Stack '%s' removed successfully", name),
		"name":    name,
	}, nil), nil
}

// handleStartStack handles the start_stack command
func (h *Handler) handleStartStack(ctx context.Context, commandID string, params map[string]any) (*protocol.Message, error) {
	name, ok := params["name"].(string)
	if !ok {
		return protocol.NewResponse(commandID, "error", nil, errNameParameterRequired), nil
	}

	err := h.composeClient.StartStack(ctx, name)
	if err != nil {
		return protocol.NewResponse(commandID, "error", nil, err), nil
	}

	return protocol.NewResponse(commandID, "success", map[string]any{
		"message": fmt.Sprintf("Stack '%s' started successfully", name),
		"name":    name,
	}, nil), nil
}

// handleStopStack handles the stop_stack command
func (h *Handler) handleStopStack(ctx context.Context, commandID string, params map[string]any) (*protocol.Message, error) {
	name, ok := params["name"].(string)
	if !ok {
		return protocol.NewResponse(commandID, "error", nil, errNameParameterRequired), nil
	}

	err := h.composeClient.StopStack(ctx, name)
	if err != nil {
		return protocol.NewResponse(commandID, "error", nil, err), nil
	}

	return protocol.NewResponse(commandID, "success", map[string]any{
		"message": fmt.Sprintf("Stack '%s' stopped successfully", name),
		"name":    name,
	}, nil), nil
}

// handleRestartStack handles the restart_stack command
func (h *Handler) handleRestartStack(ctx context.Context, commandID string, params map[string]any) (*protocol.Message, error) {
	name, ok := params["name"].(string)
	if !ok {
		return protocol.NewResponse(commandID, "error", nil, errNameParameterRequired), nil
	}

	err := h.composeClient.RestartStack(ctx, name)
	if err != nil {
		return protocol.NewResponse(commandID, "error", nil, err), nil
	}

	return protocol.NewResponse(commandID, "success", map[string]any{
		"message": fmt.Sprintf("Stack '%s' restarted successfully", name),
		"name":    name,
	}, nil), nil
}

// handleImportStack handles the import_stack command
func (h *Handler) handleImportStack(ctx context.Context, commandID string, params map[string]any) (*protocol.Message, error) {
	name, ok := params["name"].(string)
	if !ok {
		return protocol.NewResponse(commandID, "error", nil, errNameParameterRequired), nil
	}

	compose, ok := params["compose"].(string)
	if !ok {
		return protocol.NewResponse(commandID, "error", nil, fmt.Errorf("compose parameter required")), nil
	}

	envVars := map[string]interface{}{}
	if envVarsParam, ok := params["env_vars"].(map[string]interface{}); ok {
		envVars = envVarsParam
	}

	err := h.composeClient.ImportStack(ctx, name, compose, envVars)
	if err != nil {
		return protocol.NewResponse(commandID, "error", nil, err), nil
	}

	return protocol.NewResponse(commandID, "success", map[string]any{
		"message":       fmt.Sprintf("Stack '%s' imported successfully", name),
		"name":          name,
		"imported":      true,
		"env_sensitive": len(envVars) > 0, // Mark as sensitive if env vars were imported
	}, nil), nil
}

// handleGetStackContainers gets containers for a stack
func (h *Handler) handleGetStackContainers(ctx context.Context, commandID string, params map[string]any) (*protocol.Message, error) {
	stackName, ok := params["stack_name"].(string)
	if !ok {
		return protocol.NewResponse(commandID, "error", nil, fmt.Errorf("stack_name parameter required")), nil
	}

	containers, err := h.composeClient.GetStackContainers(ctx, stackName)
	if err != nil {
		return protocol.NewResponse(commandID, "error", nil, err), nil
	}

	return protocol.NewResponse(commandID, "success", map[string]any{
		"containers": containers,
	}, nil), nil
}

// handleStackContainerAction handles start/stop/restart for individual containers
func (h *Handler) handleStackContainerAction(ctx context.Context, commandID string, params map[string]any) (*protocol.Message, error) {
	containerID, ok := params["container_id"].(string)
	if !ok {
		return protocol.NewResponse(commandID, "error", nil, errContainerIDParameterRequired), nil
	}

	action, ok := params["action"].(string)
	if !ok {
		return protocol.NewResponse(commandID, "error", nil, fmt.Errorf("action parameter required")), nil
	}

	// Only allow start, stop, restart - no removal
	switch action {
	case "start":
		err := h.dockerClient.StartContainer(ctx, containerID)
		if err != nil {
			return protocol.NewResponse(commandID, "error", nil, err), nil
		}
	case "stop":
		err := h.dockerClient.StopContainer(ctx, containerID, nil)
		if err != nil {
			return protocol.NewResponse(commandID, "error", nil, err), nil
		}
	case "restart":
		err := h.dockerClient.RestartContainer(ctx, containerID, nil)
		if err != nil {
			return protocol.NewResponse(commandID, "error", nil, err), nil
		}
	default:
		return protocol.NewResponse(commandID, "error", nil, fmt.Errorf("invalid action: %s (allowed: start, stop, restart)", action)), nil
	}

	return protocol.NewResponse(commandID, "success", map[string]any{
		"message": fmt.Sprintf("Container %s %sed successfully", containerID, action),
	}, nil), nil
}
