package metrics

import (
	"testing"

	"github.com/docker/docker/api/types"
	agentconfig "github.com/mikeysoft/flotilla/internal/agent/config"
	sharedconfig "github.com/mikeysoft/flotilla/internal/shared/config"
	"github.com/mikeysoft/flotilla/internal/shared/protocol"
)

func newTestCollector() *Collector {
	cfg := &agentconfig.Config{
		AgentConfig: sharedconfig.AgentConfig{
			MetricsEnabled: true,
		},
	}
	return NewCollector(cfg, nil, "agent-1", "host-1")
}

func TestBuildMetricsPayload(t *testing.T) {
	collector := newTestCollector()
	containerMetrics := []protocol.ContainerMetric{{ContainerID: "c1"}}
	hostMetrics := &protocol.HostMetric{CPUPercent: 50.5}

	payload := collector.buildMetricsPayload(containerMetrics, hostMetrics)
	if payload.HostID != "host-1" {
		t.Fatalf("expected host id host-1, got %s", payload.HostID)
	}
	if len(payload.ContainerMetrics) != 1 || payload.ContainerMetrics[0].ContainerID != "c1" {
		t.Fatalf("unexpected container metrics: %#v", payload.ContainerMetrics)
	}
	if payload.HostMetrics.CPUPercent != 50.5 {
		t.Fatalf("unexpected host metrics: %#v", payload.HostMetrics)
	}
}

func TestShouldCollectHostMetrics(t *testing.T) {
	collector := newTestCollector()
	collector.config.MetricsCollectHostStats = true

	if !collector.shouldCollectHostMetrics() {
		t.Fatal("expected host metrics to be collected when explicitly enabled")
	}

	collector = newTestCollector()
	collector.config.MetricsCollectHostStats = false
	collector.config.MetricsCollectHostStatsAuto = false
	if collector.shouldCollectHostMetrics() {
		t.Fatal("expected host metrics disabled when both explicit and auto false")
	}
}

func TestCalculateCPUPercentFirstSample(t *testing.T) {
	collector := newTestCollector()
	stats := &types.StatsJSON{
		Stats: types.Stats{
			CPUStats: types.CPUStats{
				CPUUsage: types.CPUUsage{
					TotalUsage: 200,
				},
				SystemUsage: 400,
				OnlineCPUs:  2,
			},
			PreCPUStats: types.CPUStats{
				CPUUsage: types.CPUUsage{
					TotalUsage: 100,
				},
				SystemUsage: 200,
			},
			MemoryStats: types.MemoryStats{
				Usage: 100,
				Limit: 200,
			},
		},
	}

	cpu := collector.calculateCPUPercent(stats, "c1")
	if cpu == 0 {
		t.Fatal("expected cpu percent > 0 for first sample")
	}
}

func TestResolveCpuCountFallback(t *testing.T) {
	collector := newTestCollector()
	stats := &types.StatsJSON{}
	if got := collector.resolveCpuCount(stats); got != 1 {
		t.Fatalf("expected fallback cpu count 1, got %d", got)
	}
	stats.CPUStats.OnlineCPUs = 4
	if got := collector.resolveCpuCount(stats); got != 4 {
		t.Fatalf("expected cpu count 4, got %d", got)
	}
}
