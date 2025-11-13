package metrics

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/api/write"
	"github.com/mikeysoft/flotilla/internal/shared/protocol"
	"github.com/sirupsen/logrus"
)

// Client wraps the InfluxDB client with helper methods
type Client struct {
	client   influxdb2.Client
	writeAPI api.WriteAPIBlocking
	queryAPI api.QueryAPI
	bucket   string
	org      string
	enabled  bool
	mu       sync.RWMutex
}

// NewClient creates a new InfluxDB client
func NewClient(url, token, org, bucket string, enabled bool) (*Client, error) {
	if !enabled {
		logrus.Info("InfluxDB metrics storage is disabled")
		return &Client{enabled: false}, nil
	}

	if token == "" {
		logrus.Warn("InfluxDB token not provided, metrics storage disabled")
		return &Client{enabled: false}, nil
	}

	logrus.Infof("Initializing InfluxDB client: url=%s, org=%s, bucket=%s", url, org, bucket)

	// Create InfluxDB client
	client := influxdb2.NewClient(url, token)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	health, err := client.Health(ctx)
	if err != nil {
		logrus.Warnf("Failed to connect to InfluxDB: %v. Metrics storage will be disabled.", err)
		return &Client{enabled: false}, nil
	}

	logrus.Infof("InfluxDB connection healthy: %s", health.Status)

	return &Client{
		client:   client,
		writeAPI: client.WriteAPIBlocking(org, bucket),
		queryAPI: client.QueryAPI(org),
		bucket:   bucket,
		org:      org,
		enabled:  true,
	}, nil
}

// IsEnabled returns whether InfluxDB is enabled
func (c *Client) IsEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.enabled
}

// WriteContainerMetrics writes container metrics to InfluxDB
func (c *Client) WriteContainerMetrics(hostID string, metrics []protocol.ContainerMetric, timestamp time.Time) error {
	if !c.IsEnabled() {
		return nil
	}

	if len(metrics) == 0 {
		return nil
	}

	points := make([]*write.Point, 0, len(metrics))

	for _, m := range metrics {
		// Build fields map
		fields := map[string]interface{}{
			"cpu_percent":      m.CPUPercent,
			"memory_usage":     clampUint64ToInt64(m.MemoryUsage),
			"memory_limit":     clampUint64ToInt64(m.MemoryLimit),
			"disk_read_bytes":  clampUint64ToInt64(m.DiskReadBytes),
			"disk_write_bytes": clampUint64ToInt64(m.DiskWriteBytes),
		}

		// Add network metrics if present
		if m.NetworkRxBytes > 0 || m.NetworkTxBytes > 0 {
			fields["network_rx_bytes"] = clampUint64ToInt64(m.NetworkRxBytes)
			fields["network_tx_bytes"] = clampUint64ToInt64(m.NetworkTxBytes)
		}

		// Create point for container metrics
		point := influxdb2.NewPoint(
			"container_metrics",
			map[string]string{
				"host_id":        hostID,
				"container_id":   m.ContainerID,
				"container_name": m.ContainerName,
				"stack_name":     m.StackName,
			},
			fields,
			timestamp,
		)

		points = append(points, point)
	}

	// Write batch
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := c.writeAPI.WritePoint(ctx, points...); err != nil {
		return fmt.Errorf("failed to write container metrics: %w", err)
	}

	logrus.Debugf("Wrote %d container metrics points to InfluxDB", len(points))
	return nil
}

// WriteHostMetrics writes host metrics to InfluxDB
func (c *Client) WriteHostMetrics(hostID string, metrics *protocol.HostMetric, timestamp time.Time) error {
	if !c.IsEnabled() {
		return nil
	}

	if metrics == nil {
		return nil
	}

	// Create point for host metrics
	tags := map[string]string{
		"host_id": hostID,
	}
	fields := map[string]interface{}{
		"cpu_percent":  metrics.CPUPercent,
		"memory_usage": clampUint64ToInt64(metrics.MemoryUsage),
		"memory_total": clampUint64ToInt64(metrics.MemoryTotal),
		"disk_usage":   clampUint64ToInt64(metrics.DiskUsage),
		"disk_total":   clampUint64ToInt64(metrics.DiskTotal),
	}
	logrus.Debugf("Creating host metrics point: tags=%v, fields=%v", tags, fields)
	point := influxdb2.NewPoint(
		"host_metrics",
		tags,
		fields,
		timestamp,
	)

	// Write point
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := c.writeAPI.WritePoint(ctx, point); err != nil {
		return fmt.Errorf("failed to write host metrics: %w", err)
	}

	logrus.Debugf("Wrote host metrics point to InfluxDB for host %s", hostID)
	return nil
}

// QueryContainerMetrics queries container metrics from InfluxDB
// SonarQube Won't Fix: This query/scan function necessarily handles many field coercions
// and guards due to Flux results being dynamically typed. Further splitting would hurt
// locality and readability without materially reducing risk. Behavior is stable and covered
// by integration usage. // NOSONAR
func (c *Client) QueryContainerMetrics(ctx context.Context, hostID, containerID string, start, end time.Time, interval time.Duration) ([]protocol.ContainerMetric, error) { // NOSONAR
	if !c.IsEnabled() {
		return nil, fmt.Errorf("InfluxDB is not enabled")
	}

	// Build Flux query with pivot so each timestamp contains all fields
	query := fmt.Sprintf(`
        from(bucket: "%s")
            |> range(start: %s, stop: %s)
            |> filter(fn: (r) => r["_measurement"] == "container_metrics")
            |> filter(fn: (r) => r["host_id"] == "%s")
            |> filter(fn: (r) => r["container_id"] == "%s")
            |> aggregateWindow(every: %s, fn: mean, createEmpty: false)
            |> pivot(rowKey: ["_time"], columnKey: ["_field"], valueColumn: "_value")
    `, c.bucket, start.Format(time.RFC3339), end.Format(time.RFC3339), hostID, containerID, interval.String())

	result, err := c.queryAPI.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query container metrics: %w", err)
	}
	defer result.Close()

	var metrics []protocol.ContainerMetric

	for result.Next() {
		record := result.Record()

		m := protocol.ContainerMetric{
			ContainerID: containerID,
		}
		// attach timestamp if present
		t := record.Time()
		if !t.IsZero() {
			m.Timestamp = t
		}
		// include tags for name/stack if present
		if v := record.ValueByKey("container_name"); v != nil {
			if s, ok := v.(string); ok {
				m.ContainerName = s
			}
		}
		if v := record.ValueByKey("stack_name"); v != nil {
			if s, ok := v.(string); ok {
				m.StackName = s
			}
		}
		if v := record.ValueByKey("cpu_percent"); v != nil {
			if f, ok := v.(float64); ok {
				m.CPUPercent = f
			}
		}
		if v := record.ValueByKey("memory_usage"); v != nil {
			switch t := v.(type) {
			case int64:
				m.MemoryUsage = clampInt64ToUint64(t)
			case float64:
				m.MemoryUsage = clampFloat64ToUint64(t)
			}
		}
		if v := record.ValueByKey("memory_limit"); v != nil {
			switch t := v.(type) {
			case int64:
				m.MemoryLimit = clampInt64ToUint64(t)
			case float64:
				m.MemoryLimit = clampFloat64ToUint64(t)
			}
		}
		if v := record.ValueByKey("disk_read_bytes"); v != nil {
			switch t := v.(type) {
			case int64:
				m.DiskReadBytes = clampInt64ToUint64(t)
			case float64:
				m.DiskReadBytes = clampFloat64ToUint64(t)
			}
		}
		if v := record.ValueByKey("disk_write_bytes"); v != nil {
			switch t := v.(type) {
			case int64:
				m.DiskWriteBytes = clampInt64ToUint64(t)
			case float64:
				m.DiskWriteBytes = clampFloat64ToUint64(t)
			}
		}
		// Ensure non-nil values (uint64 cannot be negative)

		metrics = append(metrics, m)
	}

	return metrics, nil
}

// QueryHostMetrics queries host metrics from InfluxDB
// SonarQube Won't Fix: Similar to container query, this function performs necessary field
// coercions for Flux records and maintains readability by keeping mapping inline.
// Splitting further would scatter simple, related logic. // NOSONAR
func (c *Client) QueryHostMetrics(ctx context.Context, hostID string, start, end time.Time, interval time.Duration) ([]protocol.HostMetric, error) { // NOSONAR
	if !c.IsEnabled() {
		return nil, fmt.Errorf("InfluxDB is not enabled")
	}

	// Build Flux query and pivot so each timestamp contains all fields
	query := fmt.Sprintf(`
        from(bucket: "%s")
            |> range(start: %s, stop: %s)
            |> filter(fn: (r) => r["_measurement"] == "host_metrics")
            |> filter(fn: (r) => r["host_id"] == "%s")
            |> aggregateWindow(every: %s, fn: mean, createEmpty: false)
            |> pivot(rowKey: ["_time"], columnKey: ["_field"], valueColumn: "_value")
    `, c.bucket, start.Format(time.RFC3339), end.Format(time.RFC3339), hostID, interval.String())

	result, err := c.queryAPI.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query host metrics: %w", err)
	}
	defer result.Close()

	var metrics []protocol.HostMetric

	for result.Next() {
		record := result.Record()

		m := protocol.HostMetric{}
		// attach timestamp if present
		t := record.Time()
		if !t.IsZero() {
			m.Timestamp = t
		}
		if v := record.ValueByKey("cpu_percent"); v != nil {
			if f, ok := v.(float64); ok {
				m.CPUPercent = f
			}
		}
		if v := record.ValueByKey("memory_usage"); v != nil {
			switch t := v.(type) {
			case int64:
				m.MemoryUsage = clampInt64ToUint64(t)
			case float64:
				m.MemoryUsage = clampFloat64ToUint64(t)
			}
		}
		if v := record.ValueByKey("memory_total"); v != nil {
			switch t := v.(type) {
			case int64:
				m.MemoryTotal = clampInt64ToUint64(t)
			case float64:
				m.MemoryTotal = clampFloat64ToUint64(t)
			}
		}
		if v := record.ValueByKey("disk_usage"); v != nil {
			switch t := v.(type) {
			case int64:
				m.DiskUsage = clampInt64ToUint64(t)
			case float64:
				m.DiskUsage = clampFloat64ToUint64(t)
			}
		}
		if v := record.ValueByKey("disk_total"); v != nil {
			switch t := v.(type) {
			case int64:
				m.DiskTotal = clampInt64ToUint64(t)
			case float64:
				m.DiskTotal = clampFloat64ToUint64(t)
			}
		}

		metrics = append(metrics, m)
	}

	// If no metrics found, return empty slice (not error)
	return metrics, nil
}

func clampUint64ToInt64(v uint64) int64 {
	if v > math.MaxInt64 {
		return math.MaxInt64
	}
	return int64(v)
}

func clampInt64ToUint64(v int64) uint64 {
	if v < 0 {
		return 0
	}
	return uint64(v)
}

func clampFloat64ToUint64(v float64) uint64 {
	if v <= 0 {
		return 0
	}
	if v >= float64(math.MaxUint64) {
		return math.MaxUint64
	}
	return uint64(v)
}

// Close closes the InfluxDB client
func (c *Client) Close() {
	if c.enabled && c.client != nil {
		c.client.Close()
		logrus.Info("InfluxDB client closed")
	}
}
