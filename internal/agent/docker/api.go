package docker

import (
	"context"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

// DockerAPI defines the subset of the Docker SDK behaviour used by Flotilla.
type DockerAPI interface {
	ContainerList(ctx context.Context, options types.ContainerListOptions) ([]types.Container, error)
	ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error)
	ContainerStart(ctx context.Context, containerID string, options types.ContainerStartOptions) error
	ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error
	ContainerRestart(ctx context.Context, containerID string, options container.StopOptions) error
	ContainerRemove(ctx context.Context, containerID string, options types.ContainerRemoveOptions) error
	ContainerLogs(ctx context.Context, containerID string, options types.ContainerLogsOptions) (io.ReadCloser, error)
	ContainerStats(ctx context.Context, containerID string, stream bool) (types.ContainerStats, error)
	ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *v1.Platform, containerName string) (container.CreateResponse, error)

	ImageList(ctx context.Context, options types.ImageListOptions) ([]types.ImageSummary, error)
	ImageRemove(ctx context.Context, imageRef string, options types.ImageRemoveOptions) ([]types.ImageDeleteResponseItem, error)
	ImageInspectWithRaw(ctx context.Context, imageRef string) (types.ImageInspect, []byte, error)
	ImagesPrune(ctx context.Context, pruneFilters filters.Args) (types.ImagesPruneReport, error)

	NetworkList(ctx context.Context, options types.NetworkListOptions) ([]types.NetworkResource, error)
	NetworkInspect(ctx context.Context, networkID string, options types.NetworkInspectOptions) (types.NetworkResource, error)
	NetworkRemove(ctx context.Context, networkID string) error

	VolumeList(ctx context.Context, options volume.ListOptions) (volume.ListResponse, error)
	VolumeInspect(ctx context.Context, volumeName string) (volume.Volume, error)
	VolumeRemove(ctx context.Context, volumeName string, force bool) error

	Events(ctx context.Context, options types.EventsOptions) (<-chan events.Message, <-chan error)

	Ping(ctx context.Context) (types.Ping, error)
	Info(ctx context.Context) (types.Info, error)
	ServerVersion(ctx context.Context) (types.Version, error)
}

var _ DockerAPI = (*client.Client)(nil)
