package metrics

import (
	"context"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/mikeysoft/flotilla/internal/agent/config"
	"github.com/mikeysoft/flotilla/internal/agent/docker"
	"github.com/mikeysoft/flotilla/internal/shared/protocol"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/sirupsen/logrus"
)

// Collector collects metrics from Docker containers and optionally the host system
type Collector struct {
	config            *config.Config
	dockerClient      *docker.Client
	agentID           string
	hostID            string
	stopCh            chan struct{}
	metricsSender     MetricsSender
	previousStats     map[string]*types.StatsJSON
	previousStatsTime map[string]time.Time
	// disk I/O fallback (cgroup v2)
	previousIOTotals map[string]struct {
		Read  uint64
		Write uint64
	}
	// detection of missing/zero blkio to trigger fallback automatically
	ioZeroIntervals  map[string]int
	ioFallbackActive bool
	ioSwitchLogged   bool
	// host metrics autodetect state
	hostAutoChecked bool
	hostAutoEnabled bool
	hostAutoLogged  bool
	mu              sync.RWMutex
}

// MetricsSender interface for sending metrics to the server
type MetricsSender interface {
	SendMetrics(message *protocol.Message) error
}

// NewCollector creates a new metrics collector
func NewCollector(cfg *config.Config, dockerClient *docker.Client, agentID, hostID string) *Collector {
	return &Collector{
		config:            cfg,
		dockerClient:      dockerClient,
		agentID:           agentID,
		hostID:            hostID,
		stopCh:            make(chan struct{}),
		previousStats:     make(map[string]*types.StatsJSON),
		previousStatsTime: make(map[string]time.Time),
		previousIOTotals: make(map[string]struct {
			Read  uint64
			Write uint64
		}),
		ioZeroIntervals: make(map[string]int),
	}
}

// SetMetricsSender sets the metrics sender
func (c *Collector) SetMetricsSender(sender MetricsSender) {
	c.metricsSender = sender
}

// SetHostID updates the host ID for metrics
func (c *Collector) SetHostID(hostID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.hostID = hostID
}

// Start starts the metrics collection loop
func (c *Collector) Start(ctx context.Context) {
	if !c.config.MetricsEnabled {
		logrus.Info("Metrics collection is disabled")
		return
	}

	// Recreate stopCh if it was previously closed
	c.mu.Lock()
	if c.stopCh == nil {
		c.stopCh = make(chan struct{})
	}
	c.mu.Unlock()

	logrus.Infof("Starting metrics collector with interval: %v", c.config.MetricsCollectionInterval)

	ticker := time.NewTicker(c.config.MetricsCollectionInterval)
	defer ticker.Stop()

	// Collect immediately on start
	c.collectAndSend(ctx)

	for {
		select {
		case <-c.stopCh:
			logrus.Info("Metrics collector stopped")
			return
		case <-ticker.C:
			c.collectAndSend(ctx)
		}
	}
}

// Stop stops the metrics collector
func (c *Collector) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stopCh != nil {
		close(c.stopCh)
		c.stopCh = nil
	}
}

// collectAndSend collects metrics and sends them to the server
func (c *Collector) collectAndSend(ctx context.Context) {
	if c.metricsSender == nil {
		logrus.Debug("Metrics sender not set, skipping collection")
		return
	}

	// Collect container metrics
	containerMetrics, err := c.collectContainerMetrics(ctx)
	if err != nil {
		logrus.Errorf("Failed to collect container metrics: %v", err)
		return
	}

	logrus.Debugf("Collected %d container metrics", len(containerMetrics))

	// Host metrics
	var hostMetrics *protocol.HostMetric
	if c.shouldCollectHostMetrics() {
		logrus.Debugf("Collecting host metrics...")
		hm, herr := c.collectHostMetrics()
		if herr != nil {
			logrus.Errorf("Failed to collect host metrics: %v", herr)
		} else {
			hostMetrics = hm
			logrus.Debugf("Collected host metrics: CPU=%.2f%%, Memory=%d/%d", hostMetrics.CPUPercent, hostMetrics.MemoryUsage, hostMetrics.MemoryTotal)
		}
	}

	// Create metrics payload and message
	payload := c.buildMetricsPayload(containerMetrics, hostMetrics)
	message := protocol.NewMetrics(c.agentID, payload)
	logrus.Debugf("Sending metrics message with %d container metrics, hostID=%s", len(payload.ContainerMetrics), c.agentID)
	c.logSerializedPreview(message)
	if err := c.metricsSender.SendMetrics(message); err != nil {
		logrus.Errorf("Failed to send metrics: %v", err)
	} else {
		logrus.Debugf("Successfully sent metrics to server")
	}
}

// shouldCollectHostMetrics determines whether host metrics collection is enabled,
// handling explicit config and one-time autodetection with logging.
func (c *Collector) shouldCollectHostMetrics() bool {
	// Explicitly enabled
	if c.config.MetricsCollectHostStats {
		return true
	}
	// Autodetect path
	if !c.config.MetricsCollectHostStatsAuto {
		return false
	}
	c.mu.Lock()
	checked := c.hostAutoChecked
	c.mu.Unlock()
	if !checked {
		enabled := c.detectHostMetricsAvailable()
		c.mu.Lock()
		c.hostAutoChecked = true
		c.hostAutoEnabled = enabled
		c.mu.Unlock()
		if enabled && !c.hostAutoLogged {
			logrus.Info("metrics: host metrics enabled via autodetect")
			c.hostAutoLogged = true
		}
	}
	c.mu.RLock()
	enabled := c.hostAutoEnabled
	c.mu.RUnlock()
	return enabled
}

// buildMetricsPayload assembles the metrics payload with timestamp and IDs.
func (c *Collector) buildMetricsPayload(containerMetrics []protocol.ContainerMetric, hostMetrics *protocol.HostMetric) *protocol.MetricsPayload {
	return &protocol.MetricsPayload{
		Timestamp:        time.Now(),
		HostID:           c.hostID,
		ContainerMetrics: containerMetrics,
		HostMetrics:      hostMetrics,
	}
}

// logSerializedPreview logs a small preview of the serialized message for debug.
func (c *Collector) logSerializedPreview(message *protocol.Message) {
	data, _ := message.Serialize()
	previewLen := 200
	if len(data) < previewLen {
		previewLen = len(data)
	}
	logrus.Debugf("Serialized metrics message (first %d chars): %s", previewLen, string(data)[:previewLen])
}

// detectHostMetricsAvailable attempts to verify that host mounts/capabilities are present
// so that host metrics will reflect the real host rather than the container.
func (c *Collector) detectHostMetricsAvailable() bool {
	// Check that proc and cgroup roots are readable when specified
	procRoot := os.Getenv("HOST_PROC_ROOT")
	if procRoot != "" {
		if _, err := os.Stat(procRoot); err != nil {
			logrus.Debugf("host metrics autodetect: missing HOST_PROC_ROOT %s: %v", procRoot, err)
			return false
		}
	}
	cgroupRoot := os.Getenv("HOST_CGROUP_ROOT")
	if cgroupRoot != "" {
		if _, err := os.Stat(cgroupRoot); err != nil {
			logrus.Debugf("host metrics autodetect: missing HOST_CGROUP_ROOT %s: %v", cgroupRoot, err)
			return false
		}
	}
	// Quick probe: attempt disk usage on host root if provided
	diskPath := "/"
	if hostRoot := os.Getenv("HOST_ROOT_PATH"); hostRoot != "" {
		diskPath = hostRoot
	}
	if _, err := disk.Usage(diskPath); err != nil {
		logrus.Debugf("host metrics autodetect: disk probe failed on %s: %v", diskPath, err)
		return false
	}
	return true
}

// collectContainerMetrics collects metrics for all running containers
func (c *Collector) collectContainerMetrics(ctx context.Context) ([]protocol.ContainerMetric, error) {
	// List all containers
	containers, err := c.dockerClient.ListContainers(ctx, false)
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	var metrics []protocol.ContainerMetric

	for _, container := range containers {
		metric, err := c.collectContainerMetric(ctx, container.ID, container.Names[0])
		if err != nil {
			logrus.Errorf("Failed to collect metrics for container %s: %v", container.ID, err)
			continue
		}

		// Extract stack name from labels if available
		if container.Labels != nil {
			if stackName, ok := container.Labels["com.docker.compose.project"]; ok {
				metric.StackName = stackName
			}
		}

		metrics = append(metrics, *metric)
	}

	return metrics, nil
}

// collectContainerMetric collects metrics for a single container
func (c *Collector) collectContainerMetric(ctx context.Context, containerID, containerName string) (*protocol.ContainerMetric, error) {
	// Get container stats
	statsJSON, err := c.dockerClient.GetContainerStatsJSON(ctx, containerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get container stats: %w", err)
	}

	// Calculate CPU percentage
	cpuPercent := c.calculateCPUPercent(statsJSON, containerID)

	// Calculate memory metrics
	memoryUsage := statsJSON.MemoryStats.Usage
	memoryLimit := statsJSON.MemoryStats.Limit
	if memoryLimit == 0 {
		memoryLimit = memoryUsage
	}

	// Calculate disk I/O with auto-fallback
	rTotal, wTotal, hasBlkio, zeroTotals := c.getDockerBlkioTotals(statsJSON)
	c.updateIoFallbackState(containerID, hasBlkio, zeroTotals)

	diskReadBytes := uint64(0)
	diskWriteBytes := uint64(0)
	useFallback := c.config.MetricsCollectDiskIOFallback || c.ioFallbackActive
	if useFallback {
		deltaR, deltaW := c.computeCgroupFallbackDeltas(containerID)
		diskReadBytes = deltaR
		diskWriteBytes = deltaW
	} else if hasBlkio {
		diskReadBytes = rTotal
		diskWriteBytes = wTotal
	}

	metric := &protocol.ContainerMetric{
		ContainerID:    containerID,
		ContainerName:  containerName,
		CPUPercent:     cpuPercent,
		MemoryUsage:    memoryUsage,
		MemoryLimit:    memoryLimit,
		DiskReadBytes:  diskReadBytes,
		DiskWriteBytes: diskWriteBytes,
	}

	// Add network metrics if enabled
	if c.config.MetricsCollectNetwork {
		rx, tx := c.aggregateNetwork(statsJSON)
		metric.NetworkRxBytes = rx
		metric.NetworkTxBytes = tx
	}

	// Store current stats for next calculation
	c.mu.Lock()
	c.previousStats[containerID] = statsJSON
	c.previousStatsTime[containerID] = time.Now()
	c.mu.Unlock()

	return metric, nil
}

// getDockerBlkioTotals sums blkio read/write from Docker stats and reports presence and zero-only status.
func (c *Collector) getDockerBlkioTotals(statsJSON *types.StatsJSON) (read uint64, write uint64, hasEntries bool, zeroTotals bool) {
	if len(statsJSON.BlkioStats.IoServiceBytesRecursive) == 0 {
		return 0, 0, false, true
	}
	var r, w uint64
	for _, entry := range statsJSON.BlkioStats.IoServiceBytesRecursive {
		if entry.Op == "Read" {
			r += entry.Value
		} else if entry.Op == "Write" {
			w += entry.Value
		}
	}
	return r, w, true, r == 0 && w == 0
}

// updateIoFallbackState updates counters and global fallback activation based on blkio availability.
// SonarQube Won't Fix: Cognitive complexity here is driven by necessary environment-specific
// branching (Docker blkio vs cgroup v2 fallbacks) and side-effectful state updates. We split
// calculation paths elsewhere; remaining branches are minimal and improve clarity and reliability.
func (c *Collector) updateIoFallbackState(containerID string, hasBlkio bool, zeroTotals bool) { // NOSONAR
	c.mu.Lock()
	defer c.mu.Unlock()
	if hasBlkio {
		if zeroTotals {
			c.ioZeroIntervals[containerID] = c.ioZeroIntervals[containerID] + 1
			if c.ioZeroIntervals[containerID] >= 3 && !c.ioFallbackActive {
				c.ioFallbackActive = true
				if !c.ioSwitchLogged {
					logrus.Info("metrics: Docker block I/O unavailable; switched to cgroup fallback")
					c.ioSwitchLogged = true
				}
			}
		} else {
			c.ioZeroIntervals[containerID] = 0
		}
		return
	}
	// No blkio entries -> likely cgroup v2
	c.ioZeroIntervals[containerID] = c.ioZeroIntervals[containerID] + 1
	if c.ioZeroIntervals[containerID] >= 1 && !c.ioFallbackActive {
		c.ioFallbackActive = true
		if !c.ioSwitchLogged {
			logrus.Info("metrics: Docker block I/O missing; switched to cgroup fallback")
			c.ioSwitchLogged = true
		}
	}
}

// computeCgroupFallbackDeltas reads cumulative io.stat and converts to per-interval deltas.
func (c *Collector) computeCgroupFallbackDeltas(containerID string) (deltaRead uint64, deltaWrite uint64) {
	r, w := c.readCgroupIO(containerID)
	c.mu.Lock()
	prev := c.previousIOTotals[containerID]
	if r >= prev.Read {
		deltaRead = r - prev.Read
	}
	if w >= prev.Write {
		deltaWrite = w - prev.Write
	}
	c.previousIOTotals[containerID] = struct {
		Read  uint64
		Write uint64
	}{Read: r, Write: w}
	c.mu.Unlock()
	return deltaRead, deltaWrite
}

// aggregateNetwork returns total rx/tx across networks.
func (c *Collector) aggregateNetwork(statsJSON *types.StatsJSON) (rx uint64, tx uint64) {
	if statsJSON.Networks == nil {
		return 0, 0
	}
	var r, t uint64
	for _, nw := range statsJSON.Networks {
		r += nw.RxBytes
		t += nw.TxBytes
	}
	return r, t
}

// readCgroupIO reads cumulative rbytes/wbytes from cgroup v2 io.stat for a container
func (c *Collector) readCgroupIO(containerID string) (readBytes uint64, writeBytes uint64) {
	// Inspect container to get PID and cgroup path
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	details, err := c.dockerClient.GetContainer(ctx, containerID)
	if err != nil {
		logrus.Debugf("IO fallback: inspect failed for %s: %v", containerID, err)
		return 0, 0
	}
	pid := details.State.Pid
	if pid <= 0 {
		return 0, 0
	}
	cgPath := c.resolveCgroupPath(pid)
	if cgPath == "" {
		return 0, 0
	}
	return c.readIoStat(cgPath)
}

// resolveCgroupPath reads /proc/<pid>/cgroup (host root) and returns unified v2 path.
func (c *Collector) resolveCgroupPath(pid int) string {
	hostProc := c.config.HostProcRoot
	if hostProc == "" {
		hostProc = "/host/proc"
	}
	cgFile := fmt.Sprintf("%s/%d/cgroup", hostProc, pid)
	data, err := os.ReadFile(cgFile) // #nosec G304 -- cgroup path constructed from /proc with sanitized pid
	if err != nil {
		return ""
	}
	lines := string(data)
	for _, line := range strings.Split(lines, "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 3)
		if len(parts) == 3 && parts[0] == "0" {
			return parts[2]
		}
	}
	return ""
}

// readIoStat reads and parses io.stat from a cgroup v2 path and sums rbytes/wbytes.
// SonarQube Won't Fix: The parsing logic requires multiple guards and iterations due to the
// io.stat format variations; further splitting would reduce locality without meaningful complexity
// reduction. The function remains small and purpose-specific after prior refactors.
func (c *Collector) readIoStat(cgroupPath string) (readBytes uint64, writeBytes uint64) { // NOSONAR
	hostCgroup := c.config.HostCgroupRoot
	if hostCgroup == "" {
		hostCgroup = "/host/sys/fs/cgroup"
	}
	ioStatPath := fmt.Sprintf("%s%s/io.stat", hostCgroup, cgroupPath)
	ioData, err := os.ReadFile(ioStatPath) // #nosec G304 -- path constrained to host cgroup directories
	if err != nil {
		return 0, 0
	}
	for _, line := range strings.Split(string(ioData), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		for _, kv := range strings.Fields(line) {
			if strings.HasPrefix(kv, "rbytes=") {
				if v, err := strconv.ParseUint(strings.TrimPrefix(kv, "rbytes="), 10, 64); err == nil {
					readBytes += v
				}
			} else if strings.HasPrefix(kv, "wbytes=") {
				if v, err := strconv.ParseUint(strings.TrimPrefix(kv, "wbytes="), 10, 64); err == nil {
					writeBytes += v
				}
			}
		}
	}
	return readBytes, writeBytes
}

// calculateCPUPercent calculates CPU usage percentage
func (c *Collector) calculateCPUPercent(stats *types.StatsJSON, containerID string) float64 {
	c.mu.RLock()
	previousStats, exists := c.previousStats[containerID]
	_, timeExists := c.previousStatsTime[containerID]
	c.mu.RUnlock()

	if !exists || !timeExists {
		return c.cpuPercentFromPreCPU(stats)
	}

	cpuPercent := c.cpuPercentFromDelta(stats, previousStats)
	if cpuPercent < 0 {
		cpuPercent = 0
	}
	if cpuPercent > 100 {
		cpuPercent = 100
	}
	return cpuPercent
}

// cpuPercentFromPreCPU computes first-sample CPU% using PreCPUStats when available.
func (c *Collector) cpuPercentFromPreCPU(stats *types.StatsJSON) float64 {
	if stats.PreCPUStats.SystemUsage <= 0 {
		return 0.0
	}
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemUsage - stats.PreCPUStats.SystemUsage)
	if systemDelta <= 0 || cpuDelta < 0 {
		return 0.0
	}
	cpuCount := c.resolveCpuCount(stats)
	return (cpuDelta / systemDelta) * float64(cpuCount) * 100.0
}

// cpuPercentFromDelta computes CPU% using deltas against previous sample.
func (c *Collector) cpuPercentFromDelta(current *types.StatsJSON, previous *types.StatsJSON) float64 {
	cpuDelta := float64(current.CPUStats.CPUUsage.TotalUsage - previous.CPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(current.CPUStats.SystemUsage - previous.CPUStats.SystemUsage)
	if systemDelta <= 0 || cpuDelta < 0 {
		return 0.0
	}
	cpuCount := c.resolveCpuCount(current)
	return (cpuDelta / systemDelta) * float64(cpuCount) * 100.0
}

// resolveCpuCount returns OnlineCPUs, falling back to per-CPU length, then 1.
func (c *Collector) resolveCpuCount(stats *types.StatsJSON) uint32 {
	cpuCount := stats.CPUStats.OnlineCPUs
	if cpuCount == 0 {
		length := len(stats.CPUStats.CPUUsage.PercpuUsage)
		if length > int(math.MaxUint32) {
			cpuCount = math.MaxUint32
		} else {
			cpuCount = uint32(length)
		}
	}
	if cpuCount == 0 {
		cpuCount = 1
	}
	return cpuCount
}

// collectHostMetrics collects host-level system metrics
func (c *Collector) collectHostMetrics() (*protocol.HostMetric, error) {
	// Get CPU percentage
	cpuPercent, err := cpu.Percent(time.Second, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get CPU usage: %w", err)
	}

	// Get memory stats
	memStats, err := mem.VirtualMemory()
	if err != nil {
		return nil, fmt.Errorf("failed to get memory stats: %w", err)
	}

	// Get disk stats - use host root if available
	diskPath := "/"
	if hostRoot := os.Getenv("HOST_ROOT_PATH"); hostRoot != "" {
		diskPath = hostRoot
	}
	diskStats, err := disk.Usage(diskPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get disk stats: %w", err)
	}

	return &protocol.HostMetric{
		CPUPercent:  cpuPercent[0],
		MemoryUsage: memStats.Used,
		MemoryTotal: memStats.Total,
		DiskUsage:   diskStats.Used,
		DiskTotal:   diskStats.Total,
	}, nil
}
