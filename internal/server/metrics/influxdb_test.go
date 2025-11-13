package metrics

import (
	"context"
	"testing"
	"time"

	"github.com/influxdata/influxdb-client-go/v2/api/write"
	"github.com/mikeysoft/flotilla/internal/shared/protocol"
)

func TestNewClientDisabled(t *testing.T) {
	client, err := NewClient("http://localhost:8086", "", "org", "bucket", true)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	if client.IsEnabled() {
		t.Fatal("expected client to be disabled when token is empty")
	}
}

func TestWriteMetricsNoOpWhenDisabled(t *testing.T) {
	client := &Client{enabled: false}
	if err := client.WriteContainerMetrics("host", nil, time.Now()); err != nil {
		t.Fatalf("WriteContainerMetrics should not error when disabled, got %v", err)
	}
	if err := client.WriteHostMetrics("host", nil, time.Now()); err != nil {
		t.Fatalf("WriteHostMetrics should not error when disabled, got %v", err)
	}
}

func TestQueryMetricsDisabled(t *testing.T) {
	client := &Client{enabled: false}
	if _, err := client.QueryContainerMetrics(context.Background(), "host", "container", time.Now(), time.Now(), time.Minute); err == nil {
		t.Fatal("expected container metrics query to fail when disabled")
	}
	if _, err := client.QueryHostMetrics(context.Background(), "host", time.Now(), time.Now(), time.Minute); err == nil {
		t.Fatal("expected host metrics query to fail when disabled")
	}
}

func TestCloseWithoutClient(t *testing.T) {
	client := &Client{enabled: false}
	client.Close() // should not panic
}

func TestWriteContainerMetricsBuildsPoints(t *testing.T) {
	called := false
	client := &Client{
		enabled: true,
		writeAPI: &writeAPIStub{writePointFn: func(points ...*write.Point) error {
			called = true
			if len(points) != 1 {
				t.Fatalf("expected single point, got %d", len(points))
			}
			return nil
		}},
	}
	metrics := []protocol.ContainerMetric{{
		ContainerID:    "cid",
		ContainerName:  "name",
		CPUPercent:     10,
		MemoryUsage:    1,
		MemoryLimit:    2,
		DiskReadBytes:  3,
		DiskWriteBytes: 4,
	}}
	if err := client.WriteContainerMetrics("host", metrics, time.Now()); err != nil {
		t.Fatalf("WriteContainerMetrics error: %v", err)
	}
	if !called {
		t.Fatal("expected write point stub to be called")
	}
}

func TestWriteHostMetricsBuildsPoint(t *testing.T) {
	called := false
	client := &Client{
		enabled: true,
		writeAPI: &writeAPIStub{writePointFn: func(points ...*write.Point) error {
			called = true
			return nil
		}},
	}
	metric := &protocol.HostMetric{CPUPercent: 10}
	if err := client.WriteHostMetrics("host", metric, time.Now()); err != nil {
		t.Fatalf("WriteHostMetrics error: %v", err)
	}
	if !called {
		t.Fatal("expected write point stub to be called")
	}
}

type writeAPIStub struct {
	writePointFn func(points ...*write.Point) error
}

func (w *writeAPIStub) WritePoint(_ context.Context, points ...*write.Point) error {
	if w.writePointFn != nil {
		return w.writePointFn(points...)
	}
	return nil
}

func (w *writeAPIStub) WriteRecord(_ context.Context, _ ...string) error {
	return nil
}

func (w *writeAPIStub) EnableBatching() {}

func (w *writeAPIStub) Flush(_ context.Context) error {
	return nil
}
