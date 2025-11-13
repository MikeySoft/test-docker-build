package docker

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

func TestGetSystemInfo(t *testing.T) {
	stub := &stubDockerAPI{
		infoFn: func(ctx context.Context) (types.Info, error) {
			return types.Info{
				ServerVersion: "",
				NCPU:          4,
				MemTotal:      2048,
			}, nil
		},
		serverVersionFn: func(ctx context.Context) (types.Version, error) {
			return types.Version{Version: "25.0.0"}, nil
		},
	}
	client := NewClient(stub)

	info, err := client.GetSystemInfo(context.Background())
	if err != nil {
		t.Fatalf("GetSystemInfo returned error: %v", err)
	}
	if info.DockerVersion != "25.0.0" {
		t.Fatalf("expected version fallback to server version, got %s", info.DockerVersion)
	}
	if info.NCPU != 4 {
		t.Fatalf("expected NCPU 4, got %d", info.NCPU)
	}
	if info.MemTotal != 2048 {
		t.Fatalf("expected MemTotal 2048, got %d", info.MemTotal)
	}
}

func TestListContainersByImage(t *testing.T) {
	called := false
	stub := &stubDockerAPI{
		containerListFn: func(ctx context.Context, opts types.ContainerListOptions) ([]types.Container, error) {
			called = true
			if vals := opts.Filters.Get("ancestor"); len(vals) != 1 || vals[0] != "repo:tag" {
				t.Fatalf("expected ancestor filter to include repo:tag")
			}
			return []types.Container{{ID: "c1"}}, nil
		},
	}
	client := NewClient(stub)

	// Non-empty list should call ContainerList with ancestor filter.
	result, err := client.ListContainersByImage(context.Background(), []string{"repo:tag"})
	if err != nil {
		t.Fatalf("ListContainersByImage error: %v", err)
	}
	if !called {
		t.Fatal("expected ContainerList to be called")
	}
	if len(result) != 1 || result[0].ID != "c1" {
		t.Fatalf("unexpected result: %#v", result)
	}

	// Empty list should short-circuit.
	called = false
	if res, err := client.ListContainersByImage(context.Background(), nil); err != nil || res != nil {
		t.Fatalf("expected nil result for empty refs")
	}
	if called {
		t.Fatal("ContainerList should not be called for empty refs")
	}
}

func TestGetContainerLogs(t *testing.T) {
	var capturedOptions types.ContainerLogsOptions
	stub := &stubDockerAPI{
		containerLogsFn: func(ctx context.Context, id string, options types.ContainerLogsOptions) (io.ReadCloser, error) {
			capturedOptions = options
			return io.NopCloser(strings.NewReader("line1\nline2")), nil
		},
	}

	client := NewClient(stub)
	payload, err := client.GetContainerLogs(context.Background(), "cid", map[string]any{"Tail": "5"})
	if err != nil {
		t.Fatalf("GetContainerLogs error: %v", err)
	}
	if capturedOptions.Tail != "5" {
		t.Fatalf("expected tail option to be forwarded, got %s", capturedOptions.Tail)
	}
	if string(payload) != "line1\nline2" {
		t.Fatalf("unexpected payload: %s", string(payload))
	}
}

func TestGetContainerStats(t *testing.T) {
	statsPayload := types.Stats{
		CPUStats: types.CPUStats{
			CPUUsage: types.CPUUsage{
				TotalUsage: 321,
			},
		},
	}
	payloadBytes, _ := json.Marshal(statsPayload)

	stub := &stubDockerAPI{
		containerStatsFn: func(ctx context.Context, id string, stream bool) (types.ContainerStats, error) {
			return types.ContainerStats{
				Body: io.NopCloser(strings.NewReader(string(payloadBytes))),
			}, nil
		},
	}

	client := NewClient(stub)
	stats, err := client.GetContainerStats(context.Background(), "cid")
	if err != nil {
		t.Fatalf("GetContainerStats returned error: %v", err)
	}
	if stats.CPUStats.CPUUsage.TotalUsage != 321 {
		t.Fatalf("expected total usage 321, got %d", stats.CPUStats.CPUUsage.TotalUsage)
	}
}

func TestGetContainerStatsJSON(t *testing.T) {
	statsPayload := types.StatsJSON{
		Stats: types.Stats{
			CPUStats: types.CPUStats{
				CPUUsage: types.CPUUsage{
					TotalUsage: 512,
				},
			},
		},
	}
	payloadBytes, _ := json.Marshal(statsPayload)

	stub := &stubDockerAPI{
		containerStatsFn: func(ctx context.Context, id string, stream bool) (types.ContainerStats, error) {
			return types.ContainerStats{
				Body: io.NopCloser(strings.NewReader(string(payloadBytes))),
			}, nil
		},
	}

	client := NewClient(stub)
	stats, err := client.GetContainerStatsJSON(context.Background(), "cid")
	if err != nil {
		t.Fatalf("GetContainerStatsJSON returned error: %v", err)
	}
	if stats.CPUStats.CPUUsage.TotalUsage != 512 {
		t.Fatalf("expected total usage 512, got %d", stats.CPUStats.CPUUsage.TotalUsage)
	}
}

func TestRunContainerCleansUpOnStartFailure(t *testing.T) {
	createCalled := false
	removeCalled := false
	stub := &stubDockerAPI{
		containerCreateFn: func(ctx context.Context, cfg *container.Config, hostCfg *container.HostConfig, netCfg *network.NetworkingConfig, platform *v1.Platform, name string) (container.CreateResponse, error) {
			createCalled = true
			return container.CreateResponse{ID: "new-container"}, nil
		},
		containerStartFn: func(ctx context.Context, id string, opts types.ContainerStartOptions) error {
			return errors.New("boom")
		},
		containerRemoveFn: func(ctx context.Context, id string, opts types.ContainerRemoveOptions) error {
			if id != "new-container" {
				t.Fatalf("expected cleanup for new-container, got %s", id)
			}
			removeCalled = true
			return nil
		},
	}

	client := NewClient(stub)
	if _, err := client.RunContainer(context.Background(), &container.Config{Image: "nginx"}, &container.HostConfig{}, nil, nil, "demo"); err == nil {
		t.Fatalf("expected error when start fails")
	}
	if !createCalled || !removeCalled {
		t.Fatalf("expected container to be created and subsequently removed on failure")
	}
}

func TestGetEventsFiltersContainers(t *testing.T) {
	var captured filters.Args
	stub := &stubDockerAPI{
		eventsFn: func(ctx context.Context, opts types.EventsOptions) (<-chan events.Message, <-chan error) {
			captured = opts.Filters
			return nil, nil
		},
	}

	client := NewClient(stub)
	client.GetEvents(context.Background())

	if vals := captured.Get("type"); len(vals) != 1 || vals[0] != "container" {
		t.Fatalf("expected Events filter to request container type, got %v", captured)
	}
}

type stubDockerAPI struct {
	DockerAPI
	infoFn            func(ctx context.Context) (types.Info, error)
	serverVersionFn   func(ctx context.Context) (types.Version, error)
	containerListFn   func(ctx context.Context, options types.ContainerListOptions) ([]types.Container, error)
	containerLogsFn   func(ctx context.Context, id string, options types.ContainerLogsOptions) (io.ReadCloser, error)
	containerStatsFn  func(ctx context.Context, id string, stream bool) (types.ContainerStats, error)
	containerCreateFn func(ctx context.Context, cfg *container.Config, hostCfg *container.HostConfig, netCfg *network.NetworkingConfig, platform *v1.Platform, name string) (container.CreateResponse, error)
	containerStartFn  func(ctx context.Context, id string, opts types.ContainerStartOptions) error
	containerRemoveFn func(ctx context.Context, id string, opts types.ContainerRemoveOptions) error
	eventsFn          func(ctx context.Context, opts types.EventsOptions) (<-chan events.Message, <-chan error)
}

func (s *stubDockerAPI) Info(ctx context.Context) (types.Info, error) {
	if s.infoFn != nil {
		return s.infoFn(ctx)
	}
	return types.Info{}, nil
}

func (s *stubDockerAPI) ServerVersion(ctx context.Context) (types.Version, error) {
	if s.serverVersionFn != nil {
		return s.serverVersionFn(ctx)
	}
	return types.Version{}, nil
}

func (s *stubDockerAPI) ContainerList(ctx context.Context, options types.ContainerListOptions) ([]types.Container, error) {
	if s.containerListFn != nil {
		return s.containerListFn(ctx, options)
	}
	return nil, nil
}

func (s *stubDockerAPI) ContainerLogs(ctx context.Context, id string, options types.ContainerLogsOptions) (io.ReadCloser, error) {
	if s.containerLogsFn != nil {
		return s.containerLogsFn(ctx, id, options)
	}
	return io.NopCloser(strings.NewReader("")), nil
}

func (s *stubDockerAPI) ContainerStats(ctx context.Context, id string, stream bool) (types.ContainerStats, error) {
	if s.containerStatsFn != nil {
		return s.containerStatsFn(ctx, id, stream)
	}
	return types.ContainerStats{}, nil
}

func (s *stubDockerAPI) ContainerCreate(ctx context.Context, cfg *container.Config, hostCfg *container.HostConfig, netCfg *network.NetworkingConfig, platform *v1.Platform, name string) (container.CreateResponse, error) {
	if s.containerCreateFn != nil {
		return s.containerCreateFn(ctx, cfg, hostCfg, netCfg, platform, name)
	}
	return container.CreateResponse{}, nil
}

func (s *stubDockerAPI) ContainerStart(ctx context.Context, id string, opts types.ContainerStartOptions) error {
	if s.containerStartFn != nil {
		return s.containerStartFn(ctx, id, opts)
	}
	return nil
}

func (s *stubDockerAPI) ContainerRemove(ctx context.Context, id string, opts types.ContainerRemoveOptions) error {
	if s.containerRemoveFn != nil {
		return s.containerRemoveFn(ctx, id, opts)
	}
	return nil
}

func (s *stubDockerAPI) Events(ctx context.Context, opts types.EventsOptions) (<-chan events.Message, <-chan error) {
	if s.eventsFn != nil {
		return s.eventsFn(ctx, opts)
	}
	return nil, nil
}
