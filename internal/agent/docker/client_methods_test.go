package docker

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

func TestClientStartStopRestartRemove(t *testing.T) {
	api := &fakeDockerAPI{}
	client := NewClient(api)

	if err := client.StartContainer(context.Background(), "ctr-start"); err != nil {
		t.Fatalf("StartContainer returned error: %v", err)
	}
	if api.startedID != "ctr-start" {
		t.Fatalf("expected start call for ctr-start, got %s", api.startedID)
	}

	timeout := 15
	if err := client.StopContainer(context.Background(), "ctr-stop", &timeout); err != nil {
		t.Fatalf("StopContainer returned error: %v", err)
	}
	if api.stoppedID != "ctr-stop" || api.stopTimeout != timeout {
		t.Fatalf("stop call mismatch: id=%s timeout=%d", api.stoppedID, api.stopTimeout)
	}

	if err := client.RestartContainer(context.Background(), "ctr-restart", &timeout); err != nil {
		t.Fatalf("RestartContainer returned error: %v", err)
	}
	if api.restartID != "ctr-restart" || api.restartTimeout != timeout {
		t.Fatalf("restart call mismatch")
	}

	if err := client.RemoveContainer(context.Background(), "ctr-remove", true); err != nil {
		t.Fatalf("RemoveContainer returned error: %v", err)
	}
	if api.removeID != "ctr-remove" || !api.removeForce {
		t.Fatalf("remove call mismatch")
	}
}

func TestClientListImagesNetworksVolumes(t *testing.T) {
	api := &fakeDockerAPI{
		images: []types.ImageSummary{{ID: "img"}},
		networks: []types.NetworkResource{{
			ID: "net",
		}},
		volumes: &volume.ListResponse{Volumes: []*volume.Volume{{Name: "vol"}}},
	}
	client := NewClient(api)

	images, err := client.ListImages(context.Background())
	if err != nil || len(images) != 1 {
		t.Fatalf("ListImages unexpected result: %v, %v", images, err)
	}
	nets, err := client.ListNetworks(context.Background())
	if err != nil || len(nets) != 1 {
		t.Fatalf("ListNetworks unexpected result: %v, %v", nets, err)
	}
	vols, err := client.ListVolumes(context.Background())
	if err != nil || len(vols) != 1 || vols[0].Name != "vol" {
		t.Fatalf("ListVolumes unexpected result: %v, %v", vols, err)
	}
}

func TestClientInspectAndRemoveResources(t *testing.T) {
	api := &fakeDockerAPI{
		network: types.NetworkResource{Name: "network"},
		volume:  volume.Volume{Name: "volume"},
		imageInspect: types.ImageInspect{
			ID: "image",
		},
		removeImageReport: []types.ImageDeleteResponseItem{{Deleted: "image"}},
	}
	client := NewClient(api)

	if _, err := client.InspectNetwork(context.Background(), "net"); err != nil || api.inspectNetID != "net" {
		t.Fatalf("InspectNetwork failure: %v", err)
	}
	if err := client.RemoveNetwork(context.Background(), "net", false); err != nil || api.removedNetID != "net" {
		t.Fatalf("RemoveNetwork failure: %v", err)
	}
	if _, err := client.InspectVolume(context.Background(), "vol"); err != nil || api.inspectVolName != "vol" {
		t.Fatalf("InspectVolume failure: %v", err)
	}
	if err := client.RemoveVolume(context.Background(), "vol", true); err != nil || api.removeVolName != "vol" || !api.removeVolForce {
		t.Fatalf("RemoveVolume failure: %v", err)
	}
	if _, err := client.RemoveImage(context.Background(), "image", true); err != nil || api.removeImageRef != "image" {
		t.Fatalf("RemoveImage failure: %v", err)
	}
	if _, err := client.InspectImage(context.Background(), "image"); err != nil || api.inspectImageRef != "image" {
		t.Fatalf("InspectImage failure: %v", err)
	}
}

func TestClientListContainersByImageFilters(t *testing.T) {
	api := &fakeDockerAPI{}
	client := NewClient(api)

	if res, err := client.ListContainersByImage(context.Background(), nil); err != nil || res != nil {
		t.Fatalf("expected nil result for nil refs")
	}

	_, _ = client.ListContainersByImage(context.Background(), []string{"img1", ""})
	if len(api.listAncestors) != 1 || api.listAncestors[0] != "img1" {
		t.Fatalf("ancestor filter mismatch: %v", api.listAncestors)
	}
}

func TestClientGetContainerLogsAggregates(t *testing.T) {
	api := &fakeDockerAPI{
		logsReader: io.NopCloser(strings.NewReader("hello world")),
	}
	client := NewClient(api)

	out, err := client.GetContainerLogs(context.Background(), "log-ctr", map[string]any{"Tail": "10"})
	if err != nil {
		t.Fatalf("GetContainerLogs returned error: %v", err)
	}
	if string(out) != "hello world" {
		t.Fatalf("unexpected log output: %s", string(out))
	}
	if api.logsOptions.Tail != "10" {
		t.Fatalf("expected tail=10, got %s", api.logsOptions.Tail)
	}
}

func TestClientGetContainerStatsDecodes(t *testing.T) {
	api := &fakeDockerAPI{
		statsJSON: `{"cpu_stats":{"online_cpus":2}}`,
	}
	client := NewClient(api)

	stats, err := client.GetContainerStats(context.Background(), "id")
	if err != nil || stats == nil {
		t.Fatalf("GetContainerStats error: %v", err)
	}
	if stats.CPUStats.OnlineCPUs != 2 {
		t.Fatalf("expected online CPUS 2, got %d", stats.CPUStats.OnlineCPUs)
	}

	statsJSON, err := client.GetContainerStatsJSON(context.Background(), "id")
	if err != nil || statsJSON == nil {
		t.Fatalf("GetContainerStatsJSON error: %v", err)
	}
	if statsJSON.CPUStats.OnlineCPUs != 2 {
		t.Fatalf("expected online CPUS 2, got %d", statsJSON.CPUStats.OnlineCPUs)
	}
}

func TestClientRunContainerHandlesStartFailure(t *testing.T) {
	api := &fakeDockerAPI{
		createResponse: container.CreateResponse{ID: "new-container"},
		startErr:       assertError("start failed"),
	}
	client := NewClient(api)

	_, err := client.RunContainer(context.Background(), &container.Config{}, &container.HostConfig{}, nil, nil, "runme")
	if err == nil {
		t.Fatalf("expected error from RunContainer")
	}
	if api.removeID != "new-container" || !api.removeForce {
		t.Fatalf("RunContainer should force remove created container")
	}
}

func TestClientPingAndInfo(t *testing.T) {
	api := &fakeDockerAPI{
		infoResult: types.Info{
			ServerVersion: "",
			NCPU:          4,
			MemTotal:      1024,
		},
		versionResult: types.Version{Version: "24.0.0"},
	}
	client := NewClient(api)

	if err := client.Ping(context.Background()); err != nil {
		t.Fatalf("Ping returned error: %v", err)
	}
	sys, err := client.GetSystemInfo(context.Background())
	if err != nil {
		t.Fatalf("GetSystemInfo error: %v", err)
	}
	if sys.DockerVersion != "24.0.0" {
		t.Fatalf("expected version fallback, got %s", sys.DockerVersion)
	}
	if sys.NCPU != 4 {
		t.Fatalf("expected ncpu 4, got %d", sys.NCPU)
	}
}

func TestClientGetEvents(t *testing.T) {
	api := &fakeDockerAPI{}
	client := NewClient(api)

	msgCh, errCh := client.GetEvents(context.Background())
	if msgCh != api.eventsCh || errCh != api.eventsErrCh {
		t.Fatalf("expected channels to match stub")
	}
	if vals := api.eventsFilters.Get("type"); len(vals) != 1 || vals[0] != "container" {
		t.Fatalf("expected events filter for container")
	}
}

type assertError string

func (e assertError) Error() string { return string(e) }

type fakeDockerAPI struct {
	listOptions   types.ContainerListOptions
	listAncestors []string

	startedID   string
	stoppedID   string
	stopTimeout int

	restartID      string
	restartTimeout int

	removeID    string
	removeForce bool

	images   []types.ImageSummary
	networks []types.NetworkResource
	volumes  *volume.ListResponse

	network      types.NetworkResource
	inspectNetID string
	removedNetID string

	volume         volume.Volume
	inspectVolName string
	removeVolName  string
	removeVolForce bool

	removeImageRef    string
	removeImageReport []types.ImageDeleteResponseItem
	inspectImageRef   string
	imageInspect      types.ImageInspect

	logsOptions types.ContainerLogsOptions
	logsReader  io.ReadCloser

	statsJSON string

	createResponse container.CreateResponse
	startErr       error

	infoResult    types.Info
	versionResult types.Version

	eventsCh      <-chan events.Message
	eventsErrCh   <-chan error
	eventsFilters filters.Args

	imagesDeleted []types.ImageDeleteResponseItem
	imageListOpts types.ImageListOptions
}

func (f *fakeDockerAPI) ContainerList(ctx context.Context, opts types.ContainerListOptions) ([]types.Container, error) {
	f.listOptions = opts
	f.listAncestors = opts.Filters.Get("ancestor")
	return nil, nil
}

func (f *fakeDockerAPI) ContainerInspect(ctx context.Context, id string) (types.ContainerJSON, error) {
	return types.ContainerJSON{}, nil
}

func (f *fakeDockerAPI) ContainerStart(ctx context.Context, id string, opts types.ContainerStartOptions) error {
	f.startedID = id
	if f.startErr != nil {
		return f.startErr
	}
	return nil
}

func (f *fakeDockerAPI) ContainerStop(ctx context.Context, id string, opts container.StopOptions) error {
	f.stoppedID = id
	if opts.Timeout != nil {
		f.stopTimeout = *opts.Timeout
	}
	return nil
}

func (f *fakeDockerAPI) ContainerRestart(ctx context.Context, id string, opts container.StopOptions) error {
	f.restartID = id
	if opts.Timeout != nil {
		f.restartTimeout = *opts.Timeout
	}
	return nil
}

func (f *fakeDockerAPI) ContainerRemove(ctx context.Context, id string, opts types.ContainerRemoveOptions) error {
	f.removeID = id
	f.removeForce = opts.Force
	return nil
}

func (f *fakeDockerAPI) ContainerLogs(ctx context.Context, id string, opts types.ContainerLogsOptions) (io.ReadCloser, error) {
	f.logsOptions = opts
	if f.logsReader != nil {
		return f.logsReader, nil
	}
	return io.NopCloser(strings.NewReader("")), nil
}

func (f *fakeDockerAPI) ContainerStats(ctx context.Context, id string, stream bool) (types.ContainerStats, error) {
	payload := f.statsJSON
	if payload == "" {
		payload = "{}"
	}
	reader := io.NopCloser(strings.NewReader(payload))
	return types.ContainerStats{Body: reader}, nil
}

func (f *fakeDockerAPI) ContainerCreate(ctx context.Context, cfg *container.Config, hostCfg *container.HostConfig, netCfg *network.NetworkingConfig, platform *v1.Platform, name string) (container.CreateResponse, error) {
	return f.createResponse, nil
}

func (f *fakeDockerAPI) ImageList(ctx context.Context, opts types.ImageListOptions) ([]types.ImageSummary, error) {
	f.imageListOpts = opts
	return f.images, nil
}

func (f *fakeDockerAPI) ImageRemove(ctx context.Context, ref string, opts types.ImageRemoveOptions) ([]types.ImageDeleteResponseItem, error) {
	f.removeImageRef = ref
	return f.removeImageReport, nil
}

func (f *fakeDockerAPI) ImageInspectWithRaw(ctx context.Context, ref string) (types.ImageInspect, []byte, error) {
	f.inspectImageRef = ref
	return f.imageInspect, nil, nil
}

func (f *fakeDockerAPI) ImagesPrune(ctx context.Context, args filters.Args) (types.ImagesPruneReport, error) {
	return types.ImagesPruneReport{}, nil
}

func (f *fakeDockerAPI) NetworkList(ctx context.Context, opts types.NetworkListOptions) ([]types.NetworkResource, error) {
	return f.networks, nil
}

func (f *fakeDockerAPI) NetworkInspect(ctx context.Context, id string, opts types.NetworkInspectOptions) (types.NetworkResource, error) {
	f.inspectNetID = id
	return f.network, nil
}

func (f *fakeDockerAPI) NetworkRemove(ctx context.Context, id string) error {
	f.removedNetID = id
	return nil
}

func (f *fakeDockerAPI) VolumeList(ctx context.Context, opts volume.ListOptions) (volume.ListResponse, error) {
	if f.volumes != nil {
		return *f.volumes, nil
	}
	return volume.ListResponse{}, nil
}

func (f *fakeDockerAPI) VolumeInspect(ctx context.Context, name string) (volume.Volume, error) {
	f.inspectVolName = name
	return f.volume, nil
}

func (f *fakeDockerAPI) VolumeRemove(ctx context.Context, name string, force bool) error {
	f.removeVolName = name
	f.removeVolForce = force
	return nil
}

func (f *fakeDockerAPI) Events(ctx context.Context, opts types.EventsOptions) (<-chan events.Message, <-chan error) {
	f.eventsFilters = opts.Filters
	return f.eventsCh, f.eventsErrCh
}

func (f *fakeDockerAPI) Ping(ctx context.Context) (types.Ping, error) {
	return types.Ping{}, nil
}

func (f *fakeDockerAPI) Info(ctx context.Context) (types.Info, error) {
	return f.infoResult, nil
}

func (f *fakeDockerAPI) ServerVersion(ctx context.Context) (types.Version, error) {
	return f.versionResult, nil
}
