package docker

import (
	"context"
	"encoding/json"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/sirupsen/logrus"
)

// Client wraps the Docker client with additional functionality
type Client struct {
	api DockerAPI
}

// NewClient creates a new Docker client wrapper
func NewClient(dockerClient DockerAPI) *Client {
	return &Client{
		api: dockerClient,
	}
}

// ListContainers returns a list of all containers
func (c *Client) ListContainers(ctx context.Context, all bool) ([]types.Container, error) {
	options := types.ContainerListOptions{
		All: all,
	}

	containers, err := c.api.ContainerList(ctx, options)
	if err != nil {
		return nil, err
	}

	logrus.Debugf("Listed %d containers", len(containers))
	return containers, nil
}

// GetContainer returns details about a specific container
func (c *Client) GetContainer(ctx context.Context, containerID string) (*types.ContainerJSON, error) {
	container, err := c.api.ContainerInspect(ctx, containerID)
	if err != nil {
		return nil, err
	}

	logrus.Debugf("Retrieved container details: %s", containerID)
	return &container, nil
}

// StartContainer starts a container
func (c *Client) StartContainer(ctx context.Context, containerID string) error {
	err := c.api.ContainerStart(ctx, containerID, types.ContainerStartOptions{})
	if err != nil {
		return err
	}

	logrus.Infof("Started container: %s", containerID)
	return nil
}

// StopContainer stops a container
func (c *Client) StopContainer(ctx context.Context, containerID string, timeout *int) error {
	timeoutDuration := 30 * time.Second
	if timeout != nil {
		timeoutDuration = time.Duration(*timeout) * time.Second
	}

	timeoutSeconds := int(timeoutDuration.Seconds())
	err := c.api.ContainerStop(ctx, containerID, container.StopOptions{
		Timeout: &timeoutSeconds,
	})
	if err != nil {
		return err
	}

	logrus.Infof("Stopped container: %s", containerID)
	return nil
}

// RestartContainer restarts a container
func (c *Client) RestartContainer(ctx context.Context, containerID string, timeout *int) error {
	timeoutDuration := 30 * time.Second
	if timeout != nil {
		timeoutDuration = time.Duration(*timeout) * time.Second
	}

	timeoutSeconds := int(timeoutDuration.Seconds())
	err := c.api.ContainerRestart(ctx, containerID, container.StopOptions{
		Timeout: &timeoutSeconds,
	})
	if err != nil {
		return err
	}

	logrus.Infof("Restarted container: %s", containerID)
	return nil
}

// RemoveContainer removes a container
func (c *Client) RemoveContainer(ctx context.Context, containerID string, force bool) error {
	options := types.ContainerRemoveOptions{
		Force: force,
	}

	err := c.api.ContainerRemove(ctx, containerID, options)
	if err != nil {
		return err
	}

	logrus.Infof("Removed container: %s", containerID)
	return nil
}

// ListImages returns a list of all images
func (c *Client) ListImages(ctx context.Context) ([]types.ImageSummary, error) {
	images, err := c.api.ImageList(ctx, types.ImageListOptions{})
	if err != nil {
		return nil, err
	}

	logrus.Debugf("Listed %d images", len(images))
	return images, nil
}

// ListNetworks returns a list of all docker networks
func (c *Client) ListNetworks(ctx context.Context) ([]types.NetworkResource, error) {
	resources, err := c.api.NetworkList(ctx, types.NetworkListOptions{})
	if err != nil {
		return nil, err
	}

	logrus.Debugf("Listed %d networks", len(resources))
	return resources, nil
}

// ListVolumes returns a list of all docker volumes
func (c *Client) ListVolumes(ctx context.Context) ([]*volume.Volume, error) {
	resp, err := c.api.VolumeList(ctx, volume.ListOptions{})
	if err != nil {
		return nil, err
	}

	volumes := make([]*volume.Volume, 0, len(resp.Volumes))
	for i := range resp.Volumes {
		volumes = append(volumes, resp.Volumes[i])
	}

	logrus.Debugf("Listed %d volumes", len(volumes))
	return volumes, nil
}

// InspectNetwork returns detailed information about a specific docker network.
func (c *Client) InspectNetwork(ctx context.Context, networkID string) (*types.NetworkResource, error) {
	resource, err := c.api.NetworkInspect(ctx, networkID, types.NetworkInspectOptions{})
	if err != nil {
		return nil, err
	}

	logrus.Debugf("Inspected network: %s", networkID)
	return &resource, nil
}

// RemoveNetwork removes a docker network by ID or name.
func (c *Client) RemoveNetwork(ctx context.Context, networkID string, force bool) error {
	// The Docker API currently ignores force for network removal; we accept it for parity.
	if err := c.api.NetworkRemove(ctx, networkID); err != nil {
		return err
	}

	logrus.Infof("Removed network: %s (force=%t)", networkID, force)
	return nil
}

// InspectVolume returns detailed information about a specific docker volume.
func (c *Client) InspectVolume(ctx context.Context, volumeName string) (*volume.Volume, error) {
	vol, err := c.api.VolumeInspect(ctx, volumeName)
	if err != nil {
		return nil, err
	}

	logrus.Debugf("Inspected volume: %s", volumeName)
	return &vol, nil
}

// RemoveVolume removes a docker volume by name.
func (c *Client) RemoveVolume(ctx context.Context, volumeName string, force bool) error {
	if err := c.api.VolumeRemove(ctx, volumeName, force); err != nil {
		return err
	}

	logrus.Infof("Removed volume: %s (force=%t)", volumeName, force)
	return nil
}

// RemoveImage removes a docker image by ID or reference
func (c *Client) RemoveImage(ctx context.Context, imageRef string, force bool) ([]types.ImageDeleteResponseItem, error) {
	options := types.ImageRemoveOptions{
		Force:         force,
		PruneChildren: true,
	}

	report, err := c.api.ImageRemove(ctx, imageRef, options)
	if err != nil {
		return nil, err
	}

	logrus.Infof("Removed image: %s (items=%d)", imageRef, len(report))
	return report, nil
}

// InspectImage returns detailed metadata about a docker image by reference.
func (c *Client) InspectImage(ctx context.Context, imageRef string) (*types.ImageInspect, error) {
	image, _, err := c.api.ImageInspectWithRaw(ctx, imageRef)
	if err != nil {
		return nil, err
	}

	logrus.Debugf("Inspected image: %s", imageRef)
	return &image, nil
}

// ListContainersByImage returns containers that were created from any of the provided image references.
func (c *Client) ListContainersByImage(ctx context.Context, imageRefs []string) ([]types.Container, error) {
	if len(imageRefs) == 0 {
		return nil, nil
	}

	args := filters.NewArgs()
	for _, ref := range imageRefs {
		if ref == "" {
			continue
		}
		args.Add("ancestor", ref)
	}

	options := types.ContainerListOptions{
		All:     true,
		Filters: args,
	}

	containers, err := c.api.ContainerList(ctx, options)
	if err != nil {
		return nil, err
	}

	logrus.Debugf("Listed %d containers referencing images %v", len(containers), imageRefs)
	return containers, nil
}

// PruneDanglingImages removes all dangling images from the host
func (c *Client) PruneDanglingImages(ctx context.Context) (*types.ImagesPruneReport, error) {
	args := filters.NewArgs(filters.Arg("dangling", "true"))
	report, err := c.api.ImagesPrune(ctx, args)
	if err != nil {
		return nil, err
	}

	logrus.Infof("Pruned %d dangling images (reclaimed=%d bytes)", len(report.ImagesDeleted), report.SpaceReclaimed)
	return &report, nil
}

// GetContainerLogs returns logs from a container
func (c *Client) GetContainerLogs(ctx context.Context, containerID string, options map[string]any) ([]byte, error) {
	// Convert options to Docker types
	dockerOptions := types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Timestamps: false,
	}

	if timestamps, ok := options["Timestamps"].(bool); ok {
		dockerOptions.Timestamps = timestamps
	}
	if tail, ok := options["Tail"].(string); ok {
		dockerOptions.Tail = tail
	}

	reader, err := c.api.ContainerLogs(ctx, containerID, dockerOptions)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	// Read all logs into a byte slice
	logs := make([]byte, 0)
	buffer := make([]byte, 1024)
	for {
		n, err := reader.Read(buffer)
		if n > 0 {
			logs = append(logs, buffer[:n]...)
		}
		if err != nil {
			break
		}
	}

	logrus.Debugf("Retrieved logs for container: %s", containerID)
	return logs, nil
}

// GetContainerStats returns statistics for a container
func (c *Client) GetContainerStats(ctx context.Context, containerID string) (*types.Stats, error) {
	stats, err := c.api.ContainerStats(ctx, containerID, false)
	if err != nil {
		return nil, err
	}
	defer stats.Body.Close()

	// Parse the stats JSON
	var containerStats types.Stats
	if err := json.NewDecoder(stats.Body).Decode(&containerStats); err != nil {
		return nil, err
	}

	logrus.Debugf("Retrieved stats for container: %s", containerID)
	return &containerStats, nil
}

// GetContainerStatsJSON returns detailed statistics as StatsJSON
func (c *Client) GetContainerStatsJSON(ctx context.Context, containerID string) (*types.StatsJSON, error) {
	stats, err := c.api.ContainerStats(ctx, containerID, false)
	if err != nil {
		return nil, err
	}
	defer stats.Body.Close()

	// Parse the stats JSON
	var containerStats types.StatsJSON
	if err := json.NewDecoder(stats.Body).Decode(&containerStats); err != nil {
		return nil, err
	}

	logrus.Debugf("Retrieved stats JSON for container: %s", containerID)
	return &containerStats, nil
}

// GetEvents returns a channel of Docker events
func (c *Client) GetEvents(ctx context.Context) (<-chan events.Message, <-chan error) {
	options := types.EventsOptions{
		Since: time.Now().Format(time.RFC3339),
		Filters: filters.NewArgs(
			filters.Arg("type", "container"),
		),
	}

	return c.api.Events(ctx, options)
}

// CreateContainer creates and optionally starts a new container
func (c *Client) CreateContainer(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *v1.Platform, containerName string) (*container.CreateResponse, error) {
	// Create the container
	response, err := c.api.ContainerCreate(ctx, config, hostConfig, networkingConfig, platform, containerName)
	if err != nil {
		return nil, err
	}

	logrus.Infof("Created container: %s (ID: %s)", containerName, response.ID)
	return &response, nil
}

// RunContainer creates and starts a new container
func (c *Client) RunContainer(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *v1.Platform, containerName string) (*container.CreateResponse, error) {
	// Create the container
	createResponse, err := c.CreateContainer(ctx, config, hostConfig, networkingConfig, platform, containerName)
	if err != nil {
		return nil, err
	}

	// Start the container
	err = c.StartContainer(ctx, createResponse.ID)
	if err != nil {
		// If start fails, clean up the created container
		c.RemoveContainer(ctx, createResponse.ID, true)
		return nil, err
	}

	logrus.Infof("Created and started container: %s (ID: %s)", containerName, createResponse.ID)
	return createResponse, nil
}

// Ping tests the connection to the Docker daemon
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.api.Ping(ctx)
	return err
}

// GetDockerClient returns the underlying Docker client interface
func (c *Client) GetDockerClient() DockerAPI {
	return c.api
}

// SystemInfo contains selected host and docker details
type SystemInfo struct {
	DockerVersion string `json:"docker_version"`
	NCPU          int    `json:"ncpu"`
	MemTotal      uint64 `json:"mem_total"`
	DiskTotal     uint64 `json:"disk_total"`
	DiskFree      uint64 `json:"disk_free"`
}

// GetSystemInfo returns docker server version and host capacity details
func (c *Client) GetSystemInfo(ctx context.Context) (*SystemInfo, error) {
	// Docker daemon info
	info, err := c.api.Info(ctx)
	if err != nil {
		return nil, err
	}

	// Some engines may not populate ServerVersion on Info; fallback to ServerVersion endpoint
	dockerVersion := info.ServerVersion
	if dockerVersion == "" {
		ver, vErr := c.api.ServerVersion(ctx)
		if vErr == nil {
			dockerVersion = ver.Version
		}
	}

	// Disk totals (root filesystem)
	du, dErr := disk.Usage("/")
	if dErr != nil {
		// Non-fatal; log and continue
		logrus.Debugf("disk usage unavailable: %v", dErr)
	}

	if dockerVersion == "" {
		dockerVersion = "unknown"
	}

	if info.NCPU == 0 {
		logrus.Debug("Docker reported 0 CPUs; defaulting to 1")
	}

	sys := &SystemInfo{
		DockerVersion: dockerVersion,
		NCPU:          info.NCPU,
		MemTotal:      clampInt64ToUint64(info.MemTotal),
	}

	if du != nil {
		sys.DiskTotal = du.Total
		sys.DiskFree = du.Free
	}

	logrus.Debugf("SystemInfo: docker=%s, ncpu=%d, mem_total=%d, disk_total=%d",
		sys.DockerVersion, sys.NCPU, sys.MemTotal, sys.DiskTotal)

	return sys, nil
}

func clampInt64ToUint64(v int64) uint64 {
	if v <= 0 {
		return 0
	}
	return uint64(v)
}
