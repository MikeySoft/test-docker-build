package commands

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	"github.com/mikeysoft/flotilla/internal/agent/docker"
	"github.com/mikeysoft/flotilla/internal/shared/protocol"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

func TestNormalizeContainerStatus(t *testing.T) {
	tests := []struct {
		status string
		state  string
		want   string
	}{
		{"Up 2 minutes", "running", "running"},
		{"Exited (0) 3 hours ago", "exited", "stopped"},
		{"", "paused", "paused"},
		{"", "restarting", "restarting"},
		{"", "dead", "error"},
		{"Foo", "created", "stopped"},
		{"Foo", "", "stopped"},
	}

	for _, tt := range tests {
		if got := normalizeContainerStatus(tt.status, tt.state); got != tt.want {
			t.Fatalf("normalizeContainerStatus(%q, %q) = %s, want %s", tt.status, tt.state, got, tt.want)
		}
	}
}

func TestExtractStringSlice(t *testing.T) {
	params := map[string]any{
		"ids": []any{"one", "two"},
	}
	values, err := extractStringSlice(params, "ids")
	if err != nil {
		t.Fatalf("extractStringSlice returned error: %v", err)
	}
	if len(values) != 2 || values[0] != "one" || values[1] != "two" {
		t.Fatalf("unexpected values: %#v", values)
	}

	params["ids"] = []any{"one", 42}
	if _, err := extractStringSlice(params, "ids"); err == nil {
		t.Fatal("expected error when slice contains non-string")
	}
}

func TestNormalizeStringList(t *testing.T) {
	list, err := normalizeStringList([]any{"a", "", "b"})
	if err != nil {
		t.Fatalf("normalizeStringList error: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected filtered list length 2, got %d", len(list))
	}
	if _, err := normalizeStringList(123); err == nil {
		t.Fatal("expected error for non list input")
	}
}

func TestFilterEmptyStrings(t *testing.T) {
	result := filterEmptyStrings([]string{"a", "", "b"})
	if len(result) != 2 || result[0] != "a" || result[1] != "b" {
		t.Fatalf("unexpected filtered result: %#v", result)
	}
}

func TestSanitizeDetails(t *testing.T) {
	details := map[string]string{
		"a": "value",
		"b": "",
	}
	result := sanitizeDetails(details)
	if len(result) != 1 || result["a"] != "value" {
		t.Fatalf("sanitizeDetails result %#v", result)
	}

	empty := sanitizeDetails(map[string]string{"a": ""})
	if empty != nil {
		t.Fatalf("expected nil for empty map, got %#v", empty)
	}
}

func TestContainerDisplayName(t *testing.T) {
	ctr := types.Container{Names: []string{"/svc"}, ID: "123456789abc"}
	if name := containerDisplayName(ctr); name != "svc" {
		t.Fatalf("containerDisplayName = %s, want svc", name)
	}

	ctr = types.Container{ID: "123456789abc"}
	if name := containerDisplayName(ctr); name != "123456789abc"[:12] {
		t.Fatalf("containerDisplayName fallback = %s", name)
	}
}

func TestBuildContainerMetadata(t *testing.T) {
	containers := []types.Container{
		{
			ID:    "123456789abcdef",
			Names: []string{"/svc"},
			Labels: map[string]string{
				"com.docker.compose.project": "stack",
				"com.docker.compose.service": "api",
			},
		},
	}
	meta := buildContainerMetadata(containers)
	if len(meta) != 2 {
		t.Fatalf("expected metadata to include short and full IDs, got %d", len(meta))
	}
	if info := meta["123456789abcdef"]; info.Stack != "stack" || info.Service != "api" {
		t.Fatalf("metadata not populated correctly: %#v", info)
	}
}

func TestCollectVolumeConsumers(t *testing.T) {
	containers := []types.Container{
		{
			ID:    "container1",
			Names: []string{"/svc"},
			Mounts: []types.MountPoint{{
				Name:        "data",
				Destination: "/var/lib/data",
				Mode:        "rw",
				RW:          true,
			}},
			Labels: map[string]string{
				"com.docker.compose.project": "stack",
				"com.docker.compose.service": "db",
			},
		},
	}
	meta := buildContainerMetadata(containers)
	consumers := collectVolumeConsumers(containers, meta)
	volConsumers, ok := consumers["data"]
	if !ok || len(volConsumers) != 1 {
		t.Fatalf("expected one consumer for volume 'data'")
	}
	if volConsumers[0]["stack"] != "stack" {
		t.Fatalf("expected stack metadata on consumer")
	}
}

func TestHandleCommandListContainers(t *testing.T) {
	stub := &commandDockerStub{
		containerListFn: func(ctx context.Context, opts types.ContainerListOptions) ([]types.Container, error) {
			if !opts.All {
				t.Fatalf("expected All=true in options")
			}
			return []types.Container{
				{
					ID:     "abc123456789",
					Status: "Up 2 minutes",
					State:  "running",
					Names:  []string{"/web"},
				},
			}, nil
		},
	}

	handler := NewHandler(docker.NewClient(stub))
	resp, err := handler.HandleCommand(context.Background(), protocol.NewCommand("cmd-list", "list_containers", map[string]any{"all": true}))
	if err != nil {
		t.Fatalf("HandleCommand returned error: %v", err)
	}

	payloadStatus, ok := resp.Payload["status"].(string)
	if !ok || payloadStatus != "success" {
		t.Fatalf("expected success status, got %#v", resp.Payload["status"])
	}

	data := resp.Payload["data"].(map[string]any)
	containers := data["containers"].([]map[string]any)
	if len(containers) != 1 {
		t.Fatalf("expected one container in response, got %d", len(containers))
	}
	if containers[0]["name"] != "web" {
		t.Fatalf("expected container name 'web', got %v", containers[0]["name"])
	}
	if containers[0]["status"] != "running" {
		t.Fatalf("expected normalized status 'running', got %v", containers[0]["status"])
	}
}

func TestHandleCommandStartContainer(t *testing.T) {
	started := false
	stub := &commandDockerStub{
		containerStartFn: func(ctx context.Context, id string, opts types.ContainerStartOptions) error {
			if id != "container-1" {
				t.Fatalf("expected container ID 'container-1', got %s", id)
			}
			started = true
			return nil
		},
	}

	handler := NewHandler(docker.NewClient(stub))
	resp, err := handler.HandleCommand(context.Background(), protocol.NewCommand("cmd-start", "start_container", map[string]any{"container_id": "container-1"}))
	if err != nil {
		t.Fatalf("HandleCommand returned error: %v", err)
	}
	if resp.Payload["status"] != "success" {
		t.Fatalf("expected success status, got %#v", resp.Payload["status"])
	}
	if !started {
		t.Fatalf("expected containerStartFn to be called")
	}
}

func TestHandleCommandRemoveContainerStopsRunning(t *testing.T) {
	stopCalled := false
	removeCalled := false
	stub := &commandDockerStub{
		containerInspectFn: func(ctx context.Context, id string) (types.ContainerJSON, error) {
			return types.ContainerJSON{
				ContainerJSONBase: &types.ContainerJSONBase{
					State: &types.ContainerState{Running: true},
				},
			}, nil
		},
		containerStopFn: func(ctx context.Context, id string, opts container.StopOptions) error {
			stopCalled = true
			return nil
		},
		containerRemoveFn: func(ctx context.Context, id string, opts types.ContainerRemoveOptions) error {
			if opts.Force {
				t.Fatalf("expected non-forced removal")
			}
			removeCalled = true
			return nil
		},
	}

	handler := NewHandler(docker.NewClient(stub))
	resp, err := handler.HandleCommand(context.Background(), protocol.NewCommand("cmd-remove", "remove_container", map[string]any{"container_id": "running-ctr"}))
	if err != nil {
		t.Fatalf("HandleCommand returned error: %v", err)
	}
	if resp.Payload["status"] != "success" {
		t.Fatalf("expected success status, got %#v", resp.Payload["status"])
	}
	if !stopCalled {
		t.Fatalf("expected stopContainer to be called before removal")
	}
	if !removeCalled {
		t.Fatalf("expected containerRemove to be called")
	}
}

func TestHandleCommandGetDockerInfo(t *testing.T) {
	stub := &commandDockerStub{
		infoFn: func(ctx context.Context) (types.Info, error) {
			return types.Info{
				ServerVersion: "26.0.0",
				NCPU:          8,
				MemTotal:      2048,
			}, nil
		},
		serverVersionFn: func(ctx context.Context) (types.Version, error) {
			return types.Version{Version: "27.0.0"}, nil
		},
	}

	handler := NewHandler(docker.NewClient(stub))
	resp, err := handler.HandleCommand(context.Background(), protocol.NewCommand("cmd-info", "get_docker_info", nil))
	if err != nil {
		t.Fatalf("HandleCommand returned error: %v", err)
	}
	if resp.Payload["status"] != "success" {
		t.Fatalf("expected success status, got %#v", resp.Payload["status"])
	}
	data := resp.Payload["data"].(map[string]any)
	if data["docker_version"] == "" {
		t.Fatalf("expected docker_version in response")
	}
	ncpuVal := data["ncpu"]
	var ncpu int
	switch v := ncpuVal.(type) {
	case int:
		ncpu = v
	case int64:
		ncpu = int(v)
	case float64:
		ncpu = int(v)
	default:
		t.Fatalf("unexpected type for ncpu: %T", ncpuVal)
	}
	if ncpu != 8 {
		t.Fatalf("expected ncpu 8, got %v", ncpuVal)
	}
}

func TestHandleCommandGetContainerLogs(t *testing.T) {
	stub := &commandDockerStub{
		containerLogsFn: func(ctx context.Context, id string, opts types.ContainerLogsOptions) (io.ReadCloser, error) {
			if opts.Tail != "5" {
				t.Fatalf("expected tail=5, got %s", opts.Tail)
			}
			return io.NopCloser(strings.NewReader("line-1\nline-2")), nil
		},
	}

	handler := NewHandler(docker.NewClient(stub))
	resp, err := handler.HandleCommand(context.Background(), protocol.NewCommand("cmd-logs", "get_container_logs", map[string]any{
		"container_id": "log-ctr",
		"tail":         "5",
	}))
	if err != nil {
		t.Fatalf("HandleCommand returned error: %v", err)
	}
	if resp.Payload["status"] != "success" {
		t.Fatalf("expected success status, got %#v", resp.Payload["status"])
	}
	data := resp.Payload["data"].(map[string]any)
	if data["logs"] != "line-1\nline-2" {
		t.Fatalf("unexpected logs payload: %v", data["logs"])
	}
}

func TestHandleCommandRemoveImages(t *testing.T) {
	var removed []string
	stub := &commandDockerStub{
		imageRemoveFn: func(ctx context.Context, ref string, opts types.ImageRemoveOptions) ([]types.ImageDeleteResponseItem, error) {
			removed = append(removed, ref)
			return []types.ImageDeleteResponseItem{{Deleted: ref}}, nil
		},
	}

	handler := NewHandler(docker.NewClient(stub))
	resp, err := handler.HandleCommand(context.Background(), protocol.NewCommand("cmd-remove-img", "remove_images", map[string]any{
		"images": []any{"repo:tag"},
	}))
	if err != nil {
		t.Fatalf("HandleCommand returned error: %v", err)
	}
	data := resp.Payload["data"].(map[string]any)
	list := data["removed"].([]string)
	if len(list) != 1 || removed[0] != "repo:tag" {
		t.Fatalf("expected image removal recorded, got %v", list)
	}
}

func TestHandleCommandPruneDanglingImages(t *testing.T) {
	stub := &commandDockerStub{
		imagesPruneFn: func(ctx context.Context, args filters.Args) (types.ImagesPruneReport, error) {
			return types.ImagesPruneReport{
				ImagesDeleted:  []types.ImageDeleteResponseItem{{Deleted: "sha256:deadbeef"}},
				SpaceReclaimed: 4096,
			}, nil
		},
	}

	handler := NewHandler(docker.NewClient(stub))
	resp, err := handler.HandleCommand(context.Background(), protocol.NewCommand("cmd-prune", "prune_dangling_images", map[string]any{}))
	if err != nil {
		t.Fatalf("HandleCommand returned error: %v", err)
	}
	data := resp.Payload["data"].(map[string]any)
	if data["space_reclaimed"].(uint64) != 4096 {
		t.Fatalf("expected reclaimed space 4096, got %v", data["space_reclaimed"])
	}
}

func TestHandleCommandGetContainerStats(t *testing.T) {
	statsPayload := types.Stats{
		CPUStats: types.CPUStats{
			CPUUsage: types.CPUUsage{
				TotalUsage: 888,
			},
		},
	}
	payload, _ := json.Marshal(statsPayload)

	stub := &commandDockerStub{
		containerStatsFn: func(ctx context.Context, id string, stream bool) (types.ContainerStats, error) {
			return types.ContainerStats{Body: io.NopCloser(strings.NewReader(string(payload)))}, nil
		},
	}

	handler := NewHandler(docker.NewClient(stub))
	resp, err := handler.HandleCommand(context.Background(), protocol.NewCommand("cmd-stats", "get_container_stats", map[string]any{
		"container_id": "cid",
	}))
	if err != nil {
		t.Fatalf("HandleCommand returned error: %v", err)
	}
	data := resp.Payload["data"].(map[string]any)
	stats := data["stats"].(*types.Stats)
	if stats.CPUStats.CPUUsage.TotalUsage != 888 {
		t.Fatalf("expected usage 888, got %d", stats.CPUStats.CPUUsage.TotalUsage)
	}
}

func TestHandleCommandStopContainerHonorsTimeout(t *testing.T) {
	stub := &commandDockerStub{
		containerStopFn: func(ctx context.Context, id string, opts container.StopOptions) error {
			if opts.Timeout == nil || *opts.Timeout != 42 {
				t.Fatalf("expected timeout 42, got %+v", opts.Timeout)
			}
			return nil
		},
	}

	handler := NewHandler(docker.NewClient(stub))
	resp, err := handler.HandleCommand(context.Background(), protocol.NewCommand("cmd-stop", "stop_container", map[string]any{
		"container_id": "cid",
		"timeout":      float64(42),
	}))
	if err != nil {
		t.Fatalf("HandleCommand returned error: %v", err)
	}
	if resp.Payload["status"] != "success" {
		t.Fatalf("expected success status, got %#v", resp.Payload["status"])
	}
}

func TestHandleCommandRemoveContainerForceSkipsStop(t *testing.T) {
	stopCalled := false
	stub := &commandDockerStub{
		containerStopFn: func(ctx context.Context, id string, opts container.StopOptions) error {
			stopCalled = true
			return nil
		},
		containerRemoveFn: func(ctx context.Context, id string, opts types.ContainerRemoveOptions) error {
			if !opts.Force {
				t.Fatalf("expected removal with force flag")
			}
			return nil
		},
	}

	handler := NewHandler(docker.NewClient(stub))
	resp, err := handler.HandleCommand(context.Background(), protocol.NewCommand("cmd-remove-force", "remove_container", map[string]any{
		"container_id": "cid",
		"force":        true,
	}))
	if err != nil {
		t.Fatalf("HandleCommand returned error: %v", err)
	}
	if resp.Payload["status"] != "success" {
		t.Fatalf("expected success status, got %#v", resp.Payload["status"])
	}
	if stopCalled {
		t.Fatalf("expected stopContainer not to be invoked when force=true")
	}
}

func TestHandleCommandListImagesFormatsResponse(t *testing.T) {
	stub := &commandDockerStub{
		imageListFn: func(ctx context.Context, opts types.ImageListOptions) ([]types.ImageSummary, error) {
			return []types.ImageSummary{
				{
					ID:       "sha256:abcdef",
					RepoTags: []string{"nginx:latest"},
					Size:     1234,
					Created:  1700000000,
					Labels:   map[string]string{"env": "test"},
				},
			}, nil
		},
	}

	handler := NewHandler(docker.NewClient(stub))
	resp, err := handler.HandleCommand(context.Background(), protocol.NewCommand("cmd-images", "list_images", map[string]any{}))
	if err != nil {
		t.Fatalf("HandleCommand returned error: %v", err)
	}
	data := resp.Payload["data"].(map[string]any)
	images := data["images"].([]map[string]any)
	if len(images) != 1 {
		t.Fatalf("expected one image entry, got %d", len(images))
	}
	if images[0]["image"] != "nginx:latest" {
		t.Fatalf("expected primary tag nginx:latest, got %v", images[0]["image"])
	}
	if images[0]["short_id"] != "abcdef"[:6] {
		t.Fatalf("expected short id derived from digest")
	}
}

func TestHandleCommandGetContainer(t *testing.T) {
	stub := &commandDockerStub{
		containerInspectFn: func(ctx context.Context, id string) (types.ContainerJSON, error) {
			return types.ContainerJSON{
				ContainerJSONBase: &types.ContainerJSONBase{
					ID:    id,
					State: &types.ContainerState{Status: "running"},
				},
			}, nil
		},
	}

	handler := NewHandler(docker.NewClient(stub))
	resp, err := handler.HandleCommand(context.Background(), protocol.NewCommand("cmd-get", "get_container", map[string]any{
		"container_id": "demo",
	}))
	if err != nil {
		t.Fatalf("HandleCommand returned error: %v", err)
	}
	data := resp.Payload["data"].(map[string]any)
	containerJSON := data["container"].(*types.ContainerJSON)
	if containerJSON.ID != "demo" {
		t.Fatalf("expected container ID demo, got %s", containerJSON.ID)
	}
}

func TestHandleCommandListNetworks(t *testing.T) {
	stub := &commandDockerStub{
		networkListFn: func(ctx context.Context, opts types.NetworkListOptions) ([]types.NetworkResource, error) {
			return []types.NetworkResource{
				{
					ID:     "net1",
					Name:   "bridge",
					Driver: "bridge",
					Containers: map[string]types.EndpointResource{
						"ctr-1": {Name: "svc", IPv4Address: "172.18.0.2/16"},
					},
				},
			}, nil
		},
		containerListFn: func(ctx context.Context, opts types.ContainerListOptions) ([]types.Container, error) {
			return []types.Container{
				{
					ID:    "ctr-1",
					Names: []string{"/svc"},
					Labels: map[string]string{
						"com.docker.compose.project": "stack",
					},
				},
			}, nil
		},
	}

	handler := NewHandler(docker.NewClient(stub))
	resp, err := handler.HandleCommand(context.Background(), protocol.NewCommand("cmd-networks", "list_networks", map[string]any{}))
	if err != nil {
		t.Fatalf("HandleCommand returned error: %v", err)
	}
	data := resp.Payload["data"].(map[string]any)
	networks := data["networks"].([]map[string]any)
	if len(networks) != 1 {
		t.Fatalf("expected one network, got %d", len(networks))
	}
	if networks[0]["containers"].(int) != 1 {
		t.Fatalf("expected network to report one container")
	}
}

func TestHandleCommandInspectNetworks(t *testing.T) {
	inspectCalls := 0
	stub := &commandDockerStub{
		containerListFn: func(ctx context.Context, opts types.ContainerListOptions) ([]types.Container, error) {
			return nil, nil
		},
		networkInspectFn: func(ctx context.Context, id string, opts types.NetworkInspectOptions) (types.NetworkResource, error) {
			inspectCalls++
			return types.NetworkResource{ID: id, Name: "bridge"}, nil
		},
	}

	handler := NewHandler(docker.NewClient(stub))
	resp, err := handler.HandleCommand(context.Background(), protocol.NewCommand("cmd-inspect-net", "inspect_networks", map[string]any{
		"ids": []any{"net1"},
	}))
	if err != nil {
		t.Fatalf("HandleCommand returned error: %v", err)
	}
	data := resp.Payload["data"].(map[string]any)
	networks := data["networks"].([]map[string]any)
	if len(networks) != 1 || inspectCalls != 1 {
		t.Fatalf("expected single inspected network, got %d", len(networks))
	}
}

func TestHandleCommandRemoveNetworks(t *testing.T) {
	var removed []string
	stub := &commandDockerStub{
		networkRemoveFn: func(ctx context.Context, id string) error {
			removed = append(removed, id)
			return nil
		},
	}

	handler := NewHandler(docker.NewClient(stub))
	resp, err := handler.HandleCommand(context.Background(), protocol.NewCommand("cmd-remove-net", "remove_networks", map[string]any{
		"ids": []any{"net1", "net2"},
	}))
	if err != nil {
		t.Fatalf("HandleCommand returned error: %v", err)
	}
	if resp.Payload["status"] != "success" || len(removed) != 2 {
		t.Fatalf("expected two networks removed, got %v", removed)
	}
}

func TestHandleCommandListVolumes(t *testing.T) {
	stub := &commandDockerStub{
		volumeListFn: func(ctx context.Context, opts volume.ListOptions) (volume.ListResponse, error) {
			return volume.ListResponse{
				Volumes: []*volume.Volume{
					{Name: "data", Driver: "local"},
				},
			}, nil
		},
		containerListFn: func(ctx context.Context, opts types.ContainerListOptions) ([]types.Container, error) {
			return []types.Container{
				{
					ID:    "ctr",
					Names: []string{"/svc"},
					Mounts: []types.MountPoint{
						{Name: "data", Destination: "/var/lib/data"},
					},
				},
			}, nil
		},
	}

	handler := NewHandler(docker.NewClient(stub))
	resp, err := handler.HandleCommand(context.Background(), protocol.NewCommand("cmd-vols", "list_volumes", map[string]any{}))
	if err != nil {
		t.Fatalf("HandleCommand returned error: %v", err)
	}
	data := resp.Payload["data"].(map[string]any)
	volumes := data["volumes"].([]map[string]any)
	if len(volumes) != 1 || volumes[0]["name"] != "data" {
		t.Fatalf("expected normalized volume entry")
	}
}

func TestHandleCommandInspectVolumes(t *testing.T) {
	stub := &commandDockerStub{
		containerListFn: func(ctx context.Context, opts types.ContainerListOptions) ([]types.Container, error) {
			return nil, nil
		},
		volumeInspectFn: func(ctx context.Context, name string) (volume.Volume, error) {
			return volume.Volume{Name: name, Driver: "local"}, nil
		},
	}

	handler := NewHandler(docker.NewClient(stub))
	resp, err := handler.HandleCommand(context.Background(), protocol.NewCommand("cmd-inspect-vol", "inspect_volumes", map[string]any{
		"ids": []any{"data"},
	}))
	if err != nil {
		t.Fatalf("HandleCommand returned error: %v", err)
	}
	data := resp.Payload["data"].(map[string]any)
	volumes := data["volumes"].([]map[string]any)
	if len(volumes) != 1 || volumes[0]["name"] != "data" {
		t.Fatalf("expected inspected volume info")
	}
}

func TestHandleCommandRemoveVolumes(t *testing.T) {
	var removed []string
	stub := &commandDockerStub{
		volumeRemoveFn: func(ctx context.Context, name string, force bool) error {
			if !force {
				t.Fatalf("expected force flag to be true")
			}
			removed = append(removed, name)
			return nil
		},
	}

	handler := NewHandler(docker.NewClient(stub))
	_, err := handler.HandleCommand(context.Background(), protocol.NewCommand("cmd-remove-vol", "remove_volumes", map[string]any{
		"names": []any{"data"},
		"force": true,
	}))
	if err != nil {
		t.Fatalf("HandleCommand returned error: %v", err)
	}
	if len(removed) != 1 {
		t.Fatalf("expected removal invocation for volume")
	}
}

type commandDockerStub struct {
	containerListFn       func(context.Context, types.ContainerListOptions) ([]types.Container, error)
	containerInspectFn    func(context.Context, string) (types.ContainerJSON, error)
	containerStartFn      func(context.Context, string, types.ContainerStartOptions) error
	containerStopFn       func(context.Context, string, container.StopOptions) error
	containerRestartFn    func(context.Context, string, container.StopOptions) error
	containerRemoveFn     func(context.Context, string, types.ContainerRemoveOptions) error
	containerLogsFn       func(context.Context, string, types.ContainerLogsOptions) (io.ReadCloser, error)
	containerStatsFn      func(context.Context, string, bool) (types.ContainerStats, error)
	containerCreateFn     func(context.Context, *container.Config, *container.HostConfig, *network.NetworkingConfig, *v1.Platform, string) (container.CreateResponse, error)
	imageListFn           func(context.Context, types.ImageListOptions) ([]types.ImageSummary, error)
	imageRemoveFn         func(context.Context, string, types.ImageRemoveOptions) ([]types.ImageDeleteResponseItem, error)
	imageInspectWithRawFn func(context.Context, string) (types.ImageInspect, []byte, error)
	imagesPruneFn         func(context.Context, filters.Args) (types.ImagesPruneReport, error)
	networkListFn         func(context.Context, types.NetworkListOptions) ([]types.NetworkResource, error)
	networkInspectFn      func(context.Context, string, types.NetworkInspectOptions) (types.NetworkResource, error)
	networkRemoveFn       func(context.Context, string) error
	volumeListFn          func(context.Context, volume.ListOptions) (volume.ListResponse, error)
	volumeInspectFn       func(context.Context, string) (volume.Volume, error)
	volumeRemoveFn        func(context.Context, string, bool) error
	eventsFn              func(context.Context, types.EventsOptions) (<-chan events.Message, <-chan error)
	pingFn                func(context.Context) (types.Ping, error)
	infoFn                func(context.Context) (types.Info, error)
	serverVersionFn       func(context.Context) (types.Version, error)
}

func (s *commandDockerStub) ContainerList(ctx context.Context, opts types.ContainerListOptions) ([]types.Container, error) {
	if s.containerListFn != nil {
		return s.containerListFn(ctx, opts)
	}
	return nil, nil
}

func (s *commandDockerStub) ContainerInspect(ctx context.Context, id string) (types.ContainerJSON, error) {
	if s.containerInspectFn != nil {
		return s.containerInspectFn(ctx, id)
	}
	return types.ContainerJSON{}, nil
}

func (s *commandDockerStub) ContainerStart(ctx context.Context, id string, opts types.ContainerStartOptions) error {
	if s.containerStartFn != nil {
		return s.containerStartFn(ctx, id, opts)
	}
	return nil
}

func (s *commandDockerStub) ContainerStop(ctx context.Context, id string, opts container.StopOptions) error {
	if s.containerStopFn != nil {
		return s.containerStopFn(ctx, id, opts)
	}
	return nil
}

func (s *commandDockerStub) ContainerRestart(ctx context.Context, id string, opts container.StopOptions) error {
	if s.containerRestartFn != nil {
		return s.containerRestartFn(ctx, id, opts)
	}
	return nil
}

func (s *commandDockerStub) ContainerRemove(ctx context.Context, id string, opts types.ContainerRemoveOptions) error {
	if s.containerRemoveFn != nil {
		return s.containerRemoveFn(ctx, id, opts)
	}
	return nil
}

func (s *commandDockerStub) ContainerLogs(ctx context.Context, id string, opts types.ContainerLogsOptions) (io.ReadCloser, error) {
	if s.containerLogsFn != nil {
		return s.containerLogsFn(ctx, id, opts)
	}
	return io.NopCloser(strings.NewReader("")), nil
}

func (s *commandDockerStub) ContainerStats(ctx context.Context, id string, stream bool) (types.ContainerStats, error) {
	if s.containerStatsFn != nil {
		return s.containerStatsFn(ctx, id, stream)
	}
	return types.ContainerStats{}, nil
}

func (s *commandDockerStub) ContainerCreate(ctx context.Context, cfg *container.Config, hostCfg *container.HostConfig, netCfg *network.NetworkingConfig, platform *v1.Platform, name string) (container.CreateResponse, error) {
	if s.containerCreateFn != nil {
		return s.containerCreateFn(ctx, cfg, hostCfg, netCfg, platform, name)
	}
	return container.CreateResponse{}, nil
}

func (s *commandDockerStub) ImageList(ctx context.Context, opts types.ImageListOptions) ([]types.ImageSummary, error) {
	if s.imageListFn != nil {
		return s.imageListFn(ctx, opts)
	}
	return nil, nil
}

func (s *commandDockerStub) ImageRemove(ctx context.Context, ref string, opts types.ImageRemoveOptions) ([]types.ImageDeleteResponseItem, error) {
	if s.imageRemoveFn != nil {
		return s.imageRemoveFn(ctx, ref, opts)
	}
	return nil, nil
}

func (s *commandDockerStub) ImageInspectWithRaw(ctx context.Context, ref string) (types.ImageInspect, []byte, error) {
	if s.imageInspectWithRawFn != nil {
		return s.imageInspectWithRawFn(ctx, ref)
	}
	return types.ImageInspect{}, nil, nil
}

func (s *commandDockerStub) ImagesPrune(ctx context.Context, args filters.Args) (types.ImagesPruneReport, error) {
	if s.imagesPruneFn != nil {
		return s.imagesPruneFn(ctx, args)
	}
	return types.ImagesPruneReport{}, nil
}

func (s *commandDockerStub) NetworkList(ctx context.Context, opts types.NetworkListOptions) ([]types.NetworkResource, error) {
	if s.networkListFn != nil {
		return s.networkListFn(ctx, opts)
	}
	return nil, nil
}

func (s *commandDockerStub) NetworkInspect(ctx context.Context, id string, opts types.NetworkInspectOptions) (types.NetworkResource, error) {
	if s.networkInspectFn != nil {
		return s.networkInspectFn(ctx, id, opts)
	}
	return types.NetworkResource{}, nil
}

func (s *commandDockerStub) NetworkRemove(ctx context.Context, id string) error {
	if s.networkRemoveFn != nil {
		return s.networkRemoveFn(ctx, id)
	}
	return nil
}

func (s *commandDockerStub) VolumeList(ctx context.Context, opts volume.ListOptions) (volume.ListResponse, error) {
	if s.volumeListFn != nil {
		return s.volumeListFn(ctx, opts)
	}
	return volume.ListResponse{}, nil
}

func (s *commandDockerStub) VolumeInspect(ctx context.Context, name string) (volume.Volume, error) {
	if s.volumeInspectFn != nil {
		return s.volumeInspectFn(ctx, name)
	}
	return volume.Volume{}, nil
}

func (s *commandDockerStub) VolumeRemove(ctx context.Context, name string, force bool) error {
	if s.volumeRemoveFn != nil {
		return s.volumeRemoveFn(ctx, name, force)
	}
	return nil
}

func (s *commandDockerStub) Events(ctx context.Context, opts types.EventsOptions) (<-chan events.Message, <-chan error) {
	if s.eventsFn != nil {
		return s.eventsFn(ctx, opts)
	}
	return nil, nil
}

func (s *commandDockerStub) Ping(ctx context.Context) (types.Ping, error) {
	if s.pingFn != nil {
		return s.pingFn(ctx)
	}
	return types.Ping{}, nil
}

func (s *commandDockerStub) Info(ctx context.Context) (types.Info, error) {
	if s.infoFn != nil {
		return s.infoFn(ctx)
	}
	return types.Info{}, nil
}

func (s *commandDockerStub) ServerVersion(ctx context.Context) (types.Version, error) {
	if s.serverVersionFn != nil {
		return s.serverVersionFn(ctx)
	}
	return types.Version{}, nil
}
