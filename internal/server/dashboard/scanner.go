package dashboard

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/mikeysoft/flotilla/internal/server/database"
	"github.com/mikeysoft/flotilla/internal/server/metrics"
	"github.com/mikeysoft/flotilla/internal/server/topology"
	"github.com/mikeysoft/flotilla/internal/server/websocket"
	"github.com/mikeysoft/flotilla/internal/shared/protocol"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

const (
	defaultScanInterval          = 30 * time.Second
	defaultDiskWarningPercent    = 15.0
	defaultDiskCriticalPercent   = 5.0
	defaultMemoryWarningPercent  = 15.0
	defaultMemoryCriticalPercent = 5.0
	defaultOfflineCriticalAfter  = 5 * time.Minute
	commandTimeout               = 20 * time.Second
)

// ScannerOptions configures dashboard background scanning behaviour.
type ScannerOptions struct {
	Interval              time.Duration
	DiskWarningPercent    float64
	DiskCriticalPercent   float64
	MemoryWarningPercent  float64
	MemoryCriticalPercent float64
	OfflineCriticalAfter  time.Duration
}

// Scanner periodically evaluates fleet state to populate summary metrics and system tasks.
type Scanner struct {
	db       *gorm.DB
	hub      *websocket.Hub
	manager  *Manager
	topology *topology.Manager
	metrics  *metrics.Client
	opts     ScannerOptions
	started  uint32
}

// NewScanner constructs a new dashboard scanner with sane defaults.
func NewScanner(db *gorm.DB, hub *websocket.Hub, manager *Manager, topologyManager *topology.Manager, metricsClient *metrics.Client, opts *ScannerOptions) *Scanner {
	options := ScannerOptions{
		Interval:              defaultScanInterval,
		DiskWarningPercent:    defaultDiskWarningPercent,
		DiskCriticalPercent:   defaultDiskCriticalPercent,
		MemoryWarningPercent:  defaultMemoryWarningPercent,
		MemoryCriticalPercent: defaultMemoryCriticalPercent,
		OfflineCriticalAfter:  defaultOfflineCriticalAfter,
	}
	if opts != nil {
		if opts.Interval > 0 {
			options.Interval = opts.Interval
		}
		if opts.DiskWarningPercent > 0 {
			options.DiskWarningPercent = opts.DiskWarningPercent
		}
		if opts.DiskCriticalPercent > 0 {
			options.DiskCriticalPercent = opts.DiskCriticalPercent
		}
		if opts.MemoryWarningPercent > 0 {
			options.MemoryWarningPercent = opts.MemoryWarningPercent
		}
		if opts.MemoryCriticalPercent > 0 {
			options.MemoryCriticalPercent = opts.MemoryCriticalPercent
		}
		if opts.OfflineCriticalAfter > 0 {
			options.OfflineCriticalAfter = opts.OfflineCriticalAfter
		}
	}

	return &Scanner{
		db:       db,
		hub:      hub,
		manager:  manager,
		topology: topologyManager,
		metrics:  metricsClient,
		opts:     options,
	}
}

// Start launches the background scan loop. Subsequent calls are ignored.
func (s *Scanner) Start(ctx context.Context) {
	if s == nil || s.manager == nil || s.hub == nil || s.db == nil {
		logrus.Warn("dashboard scanner not started (missing dependencies)")
		return
	}
	if !atomic.CompareAndSwapUint32(&s.started, 0, 1) {
		return
	}

	go func() {
		if err := s.scan(ctx); err != nil && !errors.Is(err, context.Canceled) {
			logrus.WithError(err).Warn("initial dashboard scan failed")
		}

		ticker := time.NewTicker(s.opts.Interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.scan(ctx); err != nil && !errors.Is(err, context.Canceled) {
					logrus.WithError(err).Warn("dashboard scan failed")
				}
			}
		}
	}()
}

func (s *Scanner) scan(ctx context.Context) error {
	hosts, err := s.loadHosts(ctx)
	if err != nil {
		return err
	}

	if len(hosts) == 0 {
		s.manager.UpdateSummary(Summary{
			UpdatedAt: time.Now().UTC(),
		})
		return nil
	}

	agents := s.hub.GetAgents()
	connectedHosts := make(map[string]struct{}, len(agents))
	for _, agent := range agents {
		if agent.HostID != "" {
			connectedHosts[agent.HostID] = struct{}{}
		}
	}

	hostByID := make(map[string]database.Host, len(hosts))
	summary := Summary{
		UpdatedAt: time.Now().UTC(),
	}

	for i := range hosts {
		host := hosts[i]
		hostByID[host.ID.String()] = host
		summary.HostsTotal++

		status := strings.ToLower(strings.TrimSpace(host.Status))
		if status == "error" {
			summary.HostsError++
		}

		_, connected := connectedHosts[host.ID.String()]
		if connected {
			summary.HostsOnline++
			fingerprint := fmt.Sprintf("host_offline:%s", host.ID.String())
			if err := s.manager.ResolveTaskByFingerprint(ctx, fingerprint, StatusResolved); err != nil {
				logrus.WithError(err).WithField("host_id", host.ID).Warn("failed to resolve offline task")
			}
		} else {
			if err := s.ensureHostOfflineTask(ctx, host); err != nil {
				logrus.WithError(err).WithField("host_id", host.ID).Warn("failed to upsert host offline task")
			}
		}
	}

	for _, agent := range agents {
		host, ok := hostByID[agent.HostID]
		if !ok {
			continue
		}
		if err := s.processAgent(ctx, agent, host, &summary); err != nil && !errors.Is(err, context.Canceled) {
			logrus.WithError(err).WithField("host_id", agent.HostID).Warn("dashboard agent scan failed")
		}
	}

	summary.HostsOffline = summary.HostsTotal - summary.HostsOnline
	if summary.HostsOffline < 0 {
		summary.HostsOffline = 0
	}

	s.manager.UpdateSummary(summary)
	return nil
}

func (s *Scanner) processAgent(ctx context.Context, agent *websocket.AgentConnection, host database.Host, summary *Summary) error {
	hostID := host.ID
	hostIDPtr := uuidPtr(hostID)

	stacks, err := s.fetchStacks(ctx, agent.ID)
	if err != nil && !errors.Is(err, protocol.ErrCommandTimeout) {
		logrus.WithError(err).WithField("host_id", agent.HostID).Debug("failed to fetch stacks for dashboard scan")
	} else if len(stacks) > 0 {
		summary.StacksTotal += len(stacks)
		active := s.evaluateStacks(ctx, host, stacks, hostIDPtr)
		s.resolveMissingStackTasks(ctx, hostID, active)
	}

	containers, err := s.fetchContainers(ctx, agent.ID)
	if err != nil && !errors.Is(err, protocol.ErrCommandTimeout) {
		logrus.WithError(err).WithField("host_id", agent.HostID).Debug("failed to fetch containers for dashboard scan")
	} else {
		summary.ContainersTotal += len(containers)
	}

	if info, err := s.fetchHostInfo(ctx, agent.ID); err == nil {
		if err := s.evaluateDiskUsage(ctx, host, info, hostIDPtr); err != nil {
			logrus.WithError(err).WithField("host_id", agent.HostID).Debug("disk evaluation failed")
		}
	} else if !errors.Is(err, protocol.ErrCommandTimeout) {
		logrus.WithError(err).WithField("host_id", agent.HostID).Debug("failed to fetch host info for dashboard scan")
	}

	if err := s.evaluateMemoryUsage(ctx, host, hostIDPtr); err != nil {
		logrus.WithError(err).WithField("host_id", agent.HostID).Debug("memory evaluation failed")
	}

	return nil
}

func (s *Scanner) ensureHostOfflineTask(ctx context.Context, host database.Host) error {
	hostID := host.ID
	lastSeen := host.LastSeen
	now := time.Now()
	severity := SeverityWarning
	if lastSeen == nil || now.Sub(lastSeen.UTC()) >= s.opts.OfflineCriticalAfter {
		severity = SeverityCritical
	}

	description := "Agent has not checked in recently."
	if lastSeen != nil {
		description = fmt.Sprintf("Agent last seen %s ago (%s).", humanizeDuration(now.Sub(lastSeen.UTC())), lastSeen.UTC().Format(time.RFC3339))
	}

	fingerprint := fmt.Sprintf("host_offline:%s", hostID.String())
	_, err := s.manager.UpsertSystemTask(ctx, SystemTaskInput{
		Fingerprint: fingerprint,
		Title:       fmt.Sprintf("Host %s is offline", strings.TrimSpace(host.Name)),
		Description: description,
		Severity:    severity,
		Status:      StatusOpen,
		Category:    "host",
		TaskType:    "host_offline",
		Metadata: map[string]interface{}{
			"host_id":   hostID.String(),
			"last_seen": safeTimeString(lastSeen),
			"status":    host.Status,
		},
		HostID: uuidPtr(hostID),
	})
	return err
}

func (s *Scanner) evaluateStacks(ctx context.Context, host database.Host, stacks []map[string]any, hostID *uuid.UUID) map[string]struct{} {
	active := make(map[string]struct{})
	hostName := strings.TrimSpace(host.Name)
	hostIDStr := host.ID.String()

	for _, raw := range stacks {
		name, _ := raw["name"].(string)
		if name == "" {
			continue
		}
		stackKey := sanitizeFingerprintComponent(name)
		containersCount := intFromAny(raw["containers"])
		runningCount := intFromAny(raw["running"])
		status := strings.ToLower(getString(raw["status"]))
		managed := true
		if managedFlag, ok := raw["managed_by_flotilla"].(bool); ok {
			managed = managedFlag
		}

		fingerprintUnmanaged := fmt.Sprintf("stack_unmanaged:%s:%s", hostIDStr, stackKey)
		if !managed {
			active[fingerprintUnmanaged] = struct{}{}
			_, err := s.manager.UpsertSystemTask(ctx, SystemTaskInput{
				Fingerprint: fingerprintUnmanaged,
				Title:       fmt.Sprintf("Stack %s on %s is unmanaged", name, hostName),
				Description: "Stack is missing Flotilla management metadata. Import to manage safely.",
				Severity:    SeverityInfo,
				Status:      StatusOpen,
				Category:    "stack",
				TaskType:    "stack_unmanaged",
				Metadata: map[string]interface{}{
					"host_id":    hostIDStr,
					"stack_name": name,
				},
				HostID: hostID,
			})
			if err != nil {
				logrus.WithError(err).WithField("fingerprint", fingerprintUnmanaged).Warn("failed to upsert unmanaged stack task")
			}
		} else {
			if err := s.manager.ResolveTaskByFingerprint(ctx, fingerprintUnmanaged, StatusResolved); err != nil {
				logrus.WithError(err).WithField("fingerprint", fingerprintUnmanaged).Debug("failed to resolve unmanaged stack task")
			}
		}

		needsAttention := false
		severity := SeverityWarning
		desc := "Investigate stack container state."
		switch status {
		case "error":
			needsAttention = true
			severity = SeverityCritical
			desc = "Stack reported error state from agent."
		case "partial":
			needsAttention = true
			severity = SeverityWarning
			desc = "One or more containers are not running."
		case "stopped":
			needsAttention = true
			severity = SeverityWarning
			desc = "Stack is stopped."
		default:
			// running or unknown -> resolve
		}

		fingerprintUnhealthy := fmt.Sprintf("stack_unhealthy:%s:%s", hostIDStr, stackKey)
		if needsAttention {
			active[fingerprintUnhealthy] = struct{}{}
			_, err := s.manager.UpsertSystemTask(ctx, SystemTaskInput{
				Fingerprint: fingerprintUnhealthy,
				Title:       fmt.Sprintf("Stack %s needs attention", name),
				Description: desc,
				Severity:    severity,
				Status:      StatusOpen,
				Category:    "stack",
				TaskType:    "stack_unhealthy",
				Metadata: map[string]interface{}{
					"host_id":          hostIDStr,
					"stack_name":       name,
					"status":           status,
					"containers_total": containersCount,
					"containers_up":    runningCount,
				},
				HostID: hostID,
			})
			if err != nil {
				logrus.WithError(err).WithField("fingerprint", fingerprintUnhealthy).Warn("failed to upsert stack health task")
			}
		} else {
			if err := s.manager.ResolveTaskByFingerprint(ctx, fingerprintUnhealthy, StatusResolved); err != nil {
				logrus.WithError(err).WithField("fingerprint", fingerprintUnhealthy).Debug("failed to resolve stack health task")
			}
		}
	}

	return active
}

func (s *Scanner) resolveMissingStackTasks(ctx context.Context, hostID uuid.UUID, active map[string]struct{}) {
	if s.db == nil {
		return
	}

	var tasks []database.DashboardTask
	if err := s.db.WithContext(ctx).
		Where("host_id = ? AND source = ? AND task_type IN ? AND status IN ?",
			hostID,
			SourceSystem,
			[]string{"stack_unmanaged", "stack_unhealthy"},
			[]string{StatusOpen, StatusAcknowledged},
		).Find(&tasks).Error; err != nil {
		logrus.WithError(err).WithField("host_id", hostID.String()).Debug("failed to query existing stack tasks")
		return
	}

	for i := range tasks {
		task := tasks[i]
		if _, ok := active[task.Fingerprint]; ok {
			continue
		}
		if err := s.manager.ResolveTaskByFingerprint(ctx, task.Fingerprint, StatusResolved); err != nil {
			logrus.WithError(err).WithField("fingerprint", task.Fingerprint).Debug("failed to resolve stale stack task")
		}
	}
}

func (s *Scanner) evaluateDiskUsage(ctx context.Context, host database.Host, info map[string]any, hostID *uuid.UUID) error {
	total := floatFromAny(info["disk_total"])
	free := floatFromAny(info["disk_free"])
	if total <= 0 {
		// nothing to do
		if err := s.manager.ResolveTaskByFingerprint(ctx, fmt.Sprintf("host_low_disk:%s", host.ID.String()), StatusResolved); err != nil {
			logrus.WithError(err).WithField("host_id", host.ID.String()).Debug("failed to resolve disk task without metrics")
		}
		return nil
	}
	freePercent := 0.0
	if total > 0 {
		freePercent = (free / total) * 100.0
	}

	severity := ""
	if freePercent <= s.opts.DiskCriticalPercent {
		severity = SeverityCritical
	} else if freePercent <= s.opts.DiskWarningPercent {
		severity = SeverityWarning
	}

	fingerprint := fmt.Sprintf("host_low_disk:%s", host.ID.String())
	if severity == "" {
		return s.manager.ResolveTaskByFingerprint(ctx, fingerprint, StatusResolved)
	}

	description := fmt.Sprintf("Available disk space is %.1f%% (%.1f GiB free of %.1f GiB).", freePercent, bytesToGiB(free), bytesToGiB(total))
	_, err := s.manager.UpsertSystemTask(ctx, SystemTaskInput{
		Fingerprint: fingerprint,
		Title:       fmt.Sprintf("Host %s disk space low", strings.TrimSpace(host.Name)),
		Description: description,
		Severity:    severity,
		Status:      StatusOpen,
		Category:    "host",
		TaskType:    "host_low_disk",
		Metadata: map[string]interface{}{
			"host_id":      host.ID.String(),
			"free_bytes":   free,
			"total_bytes":  total,
			"free_percent": freePercent,
			"threshold_w":  s.opts.DiskWarningPercent,
			"threshold_c":  s.opts.DiskCriticalPercent,
		},
		HostID: hostID,
	})
	return err
}

func (s *Scanner) evaluateMemoryUsage(ctx context.Context, host database.Host, hostID *uuid.UUID) error {
	if s.metrics == nil || !s.metrics.IsEnabled() {
		return s.manager.ResolveTaskByFingerprint(ctx, fmt.Sprintf("host_low_memory:%s", host.ID.String()), StatusResolved)
	}

	end := time.Now()
	start := end.Add(-15 * time.Minute)
	metrics, err := s.metrics.QueryHostMetrics(ctx, host.ID.String(), start, end, 5*time.Minute)
	if err != nil {
		return err
	}
	if len(metrics) == 0 {
		return nil
	}

	latest := metrics[len(metrics)-1]
	if latest.MemoryTotal == 0 {
		return s.manager.ResolveTaskByFingerprint(ctx, fmt.Sprintf("host_low_memory:%s", host.ID.String()), StatusResolved)
	}

	usagePercent := (float64(latest.MemoryUsage) / float64(latest.MemoryTotal)) * 100.0
	freePercent := 100.0 - usagePercent

	severity := ""
	if freePercent <= s.opts.MemoryCriticalPercent {
		severity = SeverityCritical
	} else if freePercent <= s.opts.MemoryWarningPercent {
		severity = SeverityWarning
	}

	fingerprint := fmt.Sprintf("host_low_memory:%s", host.ID.String())
	if severity == "" {
		return s.manager.ResolveTaskByFingerprint(ctx, fingerprint, StatusResolved)
	}

	description := fmt.Sprintf("Available memory is %.1f%% (usage %.1f%%). Consider scaling or freeing memory.", freePercent, usagePercent)
	_, err = s.manager.UpsertSystemTask(ctx, SystemTaskInput{
		Fingerprint: fingerprint,
		Title:       fmt.Sprintf("Host %s low on memory", strings.TrimSpace(host.Name)),
		Description: description,
		Severity:    severity,
		Status:      StatusOpen,
		Category:    "host",
		TaskType:    "host_low_memory",
		Metadata: map[string]interface{}{
			"host_id":          host.ID.String(),
			"memory_usage":     latest.MemoryUsage,
			"memory_total":     latest.MemoryTotal,
			"usage_percent":    usagePercent,
			"free_percent":     freePercent,
			"threshold_warn":   s.opts.MemoryWarningPercent,
			"threshold_crit":   s.opts.MemoryCriticalPercent,
			"metric_timestamp": latest.Timestamp,
		},
		HostID: hostID,
	})
	return err
}

func (s *Scanner) fetchStacks(ctx context.Context, agentID string) ([]map[string]any, error) {
	command := protocol.NewCommand(uuid.NewString(), "list_stacks", map[string]any{})
	response, err := s.sendCommand(ctx, agentID, command, commandTimeout)
	if err != nil {
		return nil, err
	}
	raw, ok := response["stacks"]
	if !ok {
		return nil, nil
	}
	items, ok := raw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid stacks payload")
	}
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]any); ok {
			result = append(result, m)
		}
	}
	return result, nil
}

func (s *Scanner) fetchContainers(ctx context.Context, agentID string) ([]map[string]any, error) {
	command := protocol.NewCommand(uuid.NewString(), "list_containers", map[string]any{"all": true})
	response, err := s.sendCommand(ctx, agentID, command, commandTimeout)
	if err != nil {
		return nil, err
	}
	raw, ok := response["containers"]
	if !ok {
		return nil, nil
	}
	items, ok := raw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid containers payload")
	}
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]any); ok {
			result = append(result, m)
		}
	}
	return result, nil
}

func (s *Scanner) fetchHostInfo(ctx context.Context, agentID string) (map[string]any, error) {
	command := protocol.NewCommand(uuid.NewString(), "get_docker_info", map[string]any{})
	return s.sendCommand(ctx, agentID, command, commandTimeout)
}

func (s *Scanner) loadHosts(ctx context.Context) ([]database.Host, error) {
	var hosts []database.Host
	if err := s.db.WithContext(ctx).Find(&hosts).Error; err != nil {
		return nil, fmt.Errorf("failed to load hosts: %w", err)
	}
	return hosts, nil
}

func (s *Scanner) sendCommand(ctx context.Context, agentID string, command *protocol.Message, timeout time.Duration) (map[string]any, error) {
	responseCh := s.hub.SubscribeResponse(command.ID)
	defer s.hub.UnsubscribeResponse(command.ID)

	if err := s.hub.SendCommand(agentID, command); err != nil {
		return nil, err
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
			return nil, protocol.ErrCommandTimeout
		case response := <-responseCh:
			if response == nil || response.AgentID != agentID {
				continue
			}
			if response.Error != nil {
				return nil, response.Error
			}
			if response.Response == nil {
				return map[string]any{}, nil
			}
			payload := response.Response.Payload
			if payload == nil {
				return map[string]any{}, nil
			}
			if data, ok := payload["data"].(map[string]any); ok {
				return data, nil
			}
			return payload, nil
		}
	}
}

func uuidPtr(id uuid.UUID) *uuid.UUID {
	if id == uuid.Nil {
		return nil
	}
	v := id
	return &v
}

func sanitizeFingerprintComponent(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "_")
	return value
}

func humanizeDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

func safeTimeString(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func floatFromAny(value interface{}) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int64:
		return float64(v)
	case int:
		return float64(v)
	case uint64:
		return float64(v)
	case uint32:
		return float64(v)
	case string:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return 0
}

func intFromAny(value interface{}) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case float32:
		return int(v)
	case uint64:
		return int(v)
	case uint32:
		return int(v)
	case string:
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return 0
}

func getString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func bytesToGiB(bytes float64) float64 {
	if bytes <= 0 {
		return 0
	}
	return math.Round((bytes/1024/1024/1024)*10) / 10
}
