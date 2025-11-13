package dashboard

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/mikeysoft/flotilla/internal/server/database"
	"gorm.io/gorm"
)

const (
	StatusOpen         = "open"
	StatusAcknowledged = "acknowledged"
	StatusResolved     = "resolved"
	StatusDismissed    = "dismissed"

	SeverityInfo     = "info"
	SeverityWarning  = "warning"
	SeverityCritical = "critical"

	SourceSystem = "system"
	SourceManual = "manual"
)

var (
	allowedStatuses = map[string]struct{}{
		StatusOpen:         {},
		StatusAcknowledged: {},
		StatusResolved:     {},
		StatusDismissed:    {},
	}
	allowedSeverities = map[string]struct{}{
		SeverityInfo:     {},
		SeverityWarning:  {},
		SeverityCritical: {},
	}
)

// Summary represents high-level fleet statistics shown on the dashboard.
type Summary struct {
	HostsTotal      int       `json:"hosts_total"`
	HostsOnline     int       `json:"hosts_online"`
	HostsOffline    int       `json:"hosts_offline"`
	HostsError      int       `json:"hosts_error"`
	ContainersTotal int       `json:"containers_total"`
	StacksTotal     int       `json:"stacks_total"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// TaskFilter controls filtering and pagination of dashboard tasks.
type TaskFilter struct {
	Statuses   []string
	Severities []string
	Sources    []string
	Limit      int
	Offset     int
}

// ManualTaskInput captures the fields needed to create a manual dashboard task.
type ManualTaskInput struct {
	Title        string
	Description  string
	Severity     string
	Category     string
	TaskType     string
	Metadata     map[string]interface{}
	HostID       *uuid.UUID
	StackID      *uuid.UUID
	ContainerID  *string
	DueAt        *time.Time
	SnoozedUntil *time.Time
	CreatedBy    *uuid.UUID
}

// SystemTaskInput captures the fields needed to upsert an automated dashboard task.
type SystemTaskInput struct {
	Fingerprint string
	Title       string
	Description string
	Severity    string
	Status      string
	Category    string
	TaskType    string
	Metadata    map[string]interface{}
	HostID      *uuid.UUID
	StackID     *uuid.UUID
	ContainerID *string
}

// UpdateTaskInput represents the mutable fields for an existing task.
type UpdateTaskInput struct {
	Title           *string
	Description     *string
	Severity        *string
	Category        *string
	TaskType        *string
	Metadata        map[string]interface{}
	DueAtSet        bool
	DueAt           *time.Time
	SnoozedUntilSet bool
	SnoozedUntil    *time.Time
}

// Manager orchestrates dashboard summary data and task lifecycle operations.
type Manager struct {
	db      *gorm.DB
	mu      sync.RWMutex
	summary Summary
}

// NewManager constructs a dashboard manager backed by the provided database.
func NewManager(db *gorm.DB) *Manager {
	return &Manager{
		db: db,
	}
}

// GetSummary returns the cached summary, lazily refreshing it if empty.
func (m *Manager) GetSummary(ctx context.Context) (Summary, error) {
	m.mu.RLock()
	summary := m.summary
	m.mu.RUnlock()

	if summary.UpdatedAt.IsZero() {
		if err := m.refreshSummary(ctx); err != nil {
			return summary, err
		}
		m.mu.RLock()
		summary = m.summary
		m.mu.RUnlock()
	}

	return summary, nil
}

// RefreshSummary recomputes the summary from persistent data sources.
func (m *Manager) RefreshSummary(ctx context.Context) error {
	return m.refreshSummary(ctx)
}

// UpdateSummary allows external systems (e.g., background scanners) to inject fresh summary data.
func (m *Manager) UpdateSummary(summary Summary) {
	if summary.UpdatedAt.IsZero() {
		summary.UpdatedAt = time.Now().UTC()
	}
	m.mu.Lock()
	m.summary = summary
	m.mu.Unlock()
}

func (m *Manager) refreshSummary(ctx context.Context) error {
	if m.db == nil {
		return errors.New("dashboard manager database not configured")
	}

	type row struct {
		Status string
		Count  int
	}

	var rows []row
	if err := m.db.WithContext(ctx).
		Model(&database.Host{}).
		Select("status, COUNT(*) AS count").
		Group("status").
		Scan(&rows).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("failed to compute host summary: %w", err)
	}

	var stackTotal int64
	if err := m.db.WithContext(ctx).
		Model(&database.Stack{}).
		Count(&stackTotal).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("failed to compute stack summary: %w", err)
	}

	summary := Summary{
		StacksTotal: int(stackTotal),
		UpdatedAt:   time.Now().UTC(),
	}

	for _, r := range rows {
		switch strings.ToLower(r.Status) {
		case "online":
			summary.HostsOnline += r.Count
		case "error":
			summary.HostsError += r.Count
		default:
			summary.HostsOffline += r.Count
		}
		summary.HostsTotal += r.Count
	}

	m.mu.Lock()
	m.summary = summary
	m.mu.Unlock()
	return nil
}

// ListTasks returns dashboard tasks that match the provided filter along with the total count.
func (m *Manager) ListTasks(ctx context.Context, filter TaskFilter) ([]database.DashboardTask, int64, error) {
	query := m.db.WithContext(ctx).Model(&database.DashboardTask{})

	if len(filter.Statuses) > 0 {
		query = query.Where("status IN ?", filter.Statuses)
	}
	if len(filter.Severities) > 0 {
		query = query.Where("severity IN ?", filter.Severities)
	}
	if len(filter.Sources) > 0 {
		query = query.Where("source IN ?", filter.Sources)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count dashboard tasks: %w", err)
	}

	limit := filter.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	ordered := query.
		Order("CASE severity WHEN 'critical' THEN 3 WHEN 'warning' THEN 2 ELSE 1 END DESC").
		Order("created_at DESC")

	var tasks []database.DashboardTask
	if err := ordered.Limit(limit).Offset(offset).Find(&tasks).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list dashboard tasks: %w", err)
	}

	return tasks, total, nil
}

// CreateManualTask inserts a new manual dashboard task.
func (m *Manager) CreateManualTask(ctx context.Context, input ManualTaskInput) (*database.DashboardTask, error) {
	title := strings.TrimSpace(input.Title)
	if title == "" {
		return nil, errors.New("title is required")
	}

	severity := normalizeSeverity(input.Severity)
	if _, ok := allowedSeverities[severity]; !ok {
		return nil, fmt.Errorf("invalid severity: %s", input.Severity)
	}

	task := database.DashboardTask{
		Title:        title,
		Description:  strings.TrimSpace(input.Description),
		Severity:     severity,
		Status:       StatusOpen,
		Source:       SourceManual,
		Category:     strings.TrimSpace(input.Category),
		TaskType:     strings.TrimSpace(input.TaskType),
		DueAt:        input.DueAt,
		SnoozedUntil: input.SnoozedUntil,
		Metadata:     database.JSONB{},
	}

	if input.Metadata != nil {
		task.Metadata = database.JSONB(input.Metadata)
	}
	if input.HostID != nil {
		task.HostID = input.HostID
	}
	if input.StackID != nil {
		task.StackID = input.StackID
	}
	if input.ContainerID != nil {
		task.ContainerID = input.ContainerID
	}
	if input.CreatedBy != nil {
		task.CreatedBy = input.CreatedBy
	}

	if err := m.db.WithContext(ctx).Create(&task).Error; err != nil {
		return nil, fmt.Errorf("failed to create dashboard task: %w", err)
	}

	return &task, nil
}

// UpsertSystemTask creates or updates an automated dashboard task identified by fingerprint.
func (m *Manager) UpsertSystemTask(ctx context.Context, input SystemTaskInput) (*database.DashboardTask, error) {
	if input.Fingerprint == "" {
		return nil, errors.New("fingerprint is required")
	}

	severity := normalizeSeverity(input.Severity)
	status := normalizeStatus(input.Status)
	if status == "" {
		status = StatusOpen
	}

	var existing database.DashboardTask
	err := m.db.WithContext(ctx).
		Where("fingerprint = ? AND source = ? AND status IN ?", input.Fingerprint, SourceSystem, []string{StatusOpen, StatusAcknowledged}).
		Order("created_at DESC").
		First(&existing).Error

	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("failed to look up system task: %w", err)
		}

		task := database.DashboardTask{
			Title:       strings.TrimSpace(input.Title),
			Description: strings.TrimSpace(input.Description),
			Severity:    severity,
			Status:      status,
			Source:      SourceSystem,
			Category:    strings.TrimSpace(input.Category),
			TaskType:    strings.TrimSpace(input.TaskType),
			Fingerprint: input.Fingerprint,
			Metadata:    database.JSONB{},
		}

		if input.Metadata != nil {
			task.Metadata = database.JSONB(input.Metadata)
		}
		if input.HostID != nil {
			task.HostID = input.HostID
		}
		if input.StackID != nil {
			task.StackID = input.StackID
		}
		if input.ContainerID != nil {
			task.ContainerID = input.ContainerID
		}

		if err := m.db.WithContext(ctx).Create(&task).Error; err != nil {
			return nil, fmt.Errorf("failed to create system task: %w", err)
		}

		return &task, nil
	}

	needsUpdate := false

	if input.Title != "" && existing.Title != strings.TrimSpace(input.Title) {
		existing.Title = strings.TrimSpace(input.Title)
		needsUpdate = true
	}
	if input.Description != "" && existing.Description != strings.TrimSpace(input.Description) {
		existing.Description = strings.TrimSpace(input.Description)
		needsUpdate = true
	}
	if existing.Severity != severity {
		existing.Severity = severity
		needsUpdate = true
	}
	if input.Category != "" && existing.Category != strings.TrimSpace(input.Category) {
		existing.Category = strings.TrimSpace(input.Category)
		needsUpdate = true
	}
	if input.TaskType != "" && existing.TaskType != strings.TrimSpace(input.TaskType) {
		existing.TaskType = strings.TrimSpace(input.TaskType)
		needsUpdate = true
	}
	if input.Metadata != nil {
		existing.Metadata = database.JSONB(input.Metadata)
		needsUpdate = true
	}
	if input.HostID != nil {
		if existing.HostID == nil || existing.HostID.String() != input.HostID.String() {
			existing.HostID = input.HostID
			needsUpdate = true
		}
	}
	if input.StackID != nil {
		if existing.StackID == nil || existing.StackID.String() != input.StackID.String() {
			existing.StackID = input.StackID
			needsUpdate = true
		}
	}
	if input.ContainerID != nil {
		if existing.ContainerID == nil || *existing.ContainerID != *input.ContainerID {
			existing.ContainerID = input.ContainerID
			needsUpdate = true
		}
	}

	if status != "" && existing.Status != status && existing.Status != StatusAcknowledged {
		existing.Status = status
		needsUpdate = true
		if status == StatusResolved || status == StatusDismissed {
			now := time.Now().UTC()
			existing.ResolvedAt = &now
			existing.ResolvedBy = nil
		}
	}

	if !needsUpdate {
		return &existing, nil
	}

	if err := m.db.WithContext(ctx).Save(&existing).Error; err != nil {
		return nil, fmt.Errorf("failed to update system task: %w", err)
	}

	return &existing, nil
}

// UpdateTask applies manual edits to a dashboard task.
func (m *Manager) UpdateTask(ctx context.Context, id uuid.UUID, input UpdateTaskInput) (*database.DashboardTask, error) {
	var task database.DashboardTask
	if err := m.db.WithContext(ctx).First(&task, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("failed to load dashboard task: %w", err)
	}

	if task.Source != SourceManual {
		return nil, errors.New("only manual tasks can be edited")
	}

	if input.Title != nil {
		title := strings.TrimSpace(*input.Title)
		if title == "" {
			return nil, errors.New("title cannot be empty")
		}
		task.Title = title
	}
	if input.Description != nil {
		task.Description = strings.TrimSpace(*input.Description)
	}
	if input.Severity != nil {
		severity := normalizeSeverity(*input.Severity)
		if _, ok := allowedSeverities[severity]; !ok {
			return nil, fmt.Errorf("invalid severity: %s", *input.Severity)
		}
		task.Severity = severity
	}
	if input.Category != nil {
		task.Category = strings.TrimSpace(*input.Category)
	}
	if input.TaskType != nil {
		task.TaskType = strings.TrimSpace(*input.TaskType)
	}
	if input.Metadata != nil {
		task.Metadata = database.JSONB(input.Metadata)
	}
	if input.DueAtSet {
		task.DueAt = input.DueAt
	}
	if input.SnoozedUntilSet {
		task.SnoozedUntil = input.SnoozedUntil
	}

	if err := m.db.WithContext(ctx).Save(&task).Error; err != nil {
		return nil, fmt.Errorf("failed to update dashboard task: %w", err)
	}

	return &task, nil
}

// UpdateTaskStatus transitions a task to a new status and records audit metadata.
func (m *Manager) UpdateTaskStatus(ctx context.Context, id uuid.UUID, status string, actorID *uuid.UUID) (*database.DashboardTask, error) {
	status = strings.ToLower(strings.TrimSpace(status))
	if _, ok := allowedStatuses[status]; !ok {
		return nil, fmt.Errorf("invalid status: %s", status)
	}

	var task database.DashboardTask
	if err := m.db.WithContext(ctx).First(&task, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("failed to load dashboard task: %w", err)
	}

	now := time.Now().UTC()

	switch status {
	case StatusOpen:
		task.AcknowledgedAt = nil
		task.AcknowledgedBy = nil
		task.ResolvedAt = nil
		task.ResolvedBy = nil
	case StatusAcknowledged:
		task.AcknowledgedAt = &now
		task.AcknowledgedBy = actorID
	case StatusResolved:
		task.ResolvedAt = &now
		task.ResolvedBy = actorID
	case StatusDismissed:
		task.ResolvedAt = &now
		task.ResolvedBy = actorID
	}
	task.Status = status

	if err := m.db.WithContext(ctx).Save(&task).Error; err != nil {
		return nil, fmt.Errorf("failed to update task status: %w", err)
	}

	return &task, nil
}

// GetTask returns a single dashboard task by identifier.
func (m *Manager) GetTask(ctx context.Context, id uuid.UUID) (*database.DashboardTask, error) {
	var task database.DashboardTask
	if err := m.db.WithContext(ctx).First(&task, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

// ResolveTaskByFingerprint resolves any open or acknowledged system task with the provided fingerprint.
func (m *Manager) ResolveTaskByFingerprint(ctx context.Context, fingerprint string, resolvedStatus string) error {
	if fingerprint == "" {
		return nil
	}
	status := normalizeStatus(resolvedStatus)
	if status == "" {
		status = StatusResolved
	}

	var tasks []database.DashboardTask
	if err := m.db.WithContext(ctx).
		Where("fingerprint = ? AND source = ? AND status IN ?", fingerprint, SourceSystem, []string{StatusOpen, StatusAcknowledged}).
		Find(&tasks).Error; err != nil {
		return fmt.Errorf("failed to resolve system task: %w", err)
	}

	if len(tasks) == 0 {
		return nil
	}

	now := time.Now().UTC()
	for i := range tasks {
		task := &tasks[i]
		task.Status = status
		task.ResolvedAt = &now
		task.ResolvedBy = nil
		if err := m.db.WithContext(ctx).Save(task).Error; err != nil {
			return fmt.Errorf("failed to update system task: %w", err)
		}
	}
	return nil
}

func normalizeSeverity(severity string) string {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case SeverityCritical:
		return SeverityCritical
	case SeverityWarning:
		return SeverityWarning
	default:
		return SeverityInfo
	}
}

func normalizeStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case StatusOpen:
		return StatusOpen
	case StatusAcknowledged:
		return StatusAcknowledged
	case StatusResolved:
		return StatusResolved
	case StatusDismissed:
		return StatusDismissed
	default:
		return ""
	}
}
