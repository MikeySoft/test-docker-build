package topology

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/mikeysoft/flotilla/internal/server/database"
	"github.com/mikeysoft/flotilla/internal/server/websocket"
	"github.com/mikeysoft/flotilla/internal/shared/protocol"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	defaultRefreshInterval = 10 * time.Minute
	defaultStaleMultiplier = 2
	defaultBatchSize       = 20
	commandTimeout         = 45 * time.Second
)

// Manager coordinates cached network and volume topology state.
type Manager struct {
	hub             *websocket.Hub
	db              *gorm.DB
	refreshInterval time.Duration
	staleAfter      time.Duration
	batchSize       int
}

// NewManager constructs a new topology manager.
func NewManager(hub *websocket.Hub, db *gorm.DB, refreshInterval, staleAfter time.Duration, batchSize int) *Manager {
	if refreshInterval <= 0 {
		refreshInterval = defaultRefreshInterval
	}
	if staleAfter <= 0 {
		staleAfter = refreshInterval * defaultStaleMultiplier
	}
	if batchSize <= 0 {
		batchSize = defaultBatchSize
	}
	return &Manager{
		hub:             hub,
		db:              db,
		refreshInterval: refreshInterval,
		staleAfter:      staleAfter,
		batchSize:       batchSize,
	}
}

// RefreshNetworks triggers an inspect for the provided network IDs (or all networks when ids is empty).
func (m *Manager) RefreshNetworks(ctx context.Context, hostID string, ids []string) error {
	if m.hub == nil || m.db == nil {
		return errors.New("topology manager not fully initialised")
	}

	agent, ok := m.hub.GetAgentByHost(hostID)
	if !ok {
		return errors.New("host agent not connected")
	}

	hostUUID, err := uuid.Parse(hostID)
	if err != nil {
		return err
	}

	if len(ids) == 0 {
		response, err := m.sendCommand(ctx, agent.ID, "list_networks", map[string]any{})
		if err != nil {
			return err
		}
		ids = extractIDsFromList(response["networks"])
	}

	if len(ids) == 0 {
		return nil
	}

	now := time.Now().UTC()
	batches := chunkStrings(ids, m.batchSize)
	for _, batch := range batches {
		response, err := m.sendCommand(ctx, agent.ID, "inspect_networks", map[string]any{"ids": batch})
		if err != nil {
			return err
		}
		if err := m.persistNetworkSnapshots(ctx, hostUUID, response["networks"], now); err != nil {
			return err
		}
		m.logAgentErrors("network", hostUUID, response["errors"])
	}

	return nil
}

// RefreshVolumes triggers an inspect for the provided volume names (or all volumes when names is empty).
func (m *Manager) RefreshVolumes(ctx context.Context, hostID string, names []string) error {
	if m.hub == nil || m.db == nil {
		return errors.New("topology manager not fully initialised")
	}

	agent, ok := m.hub.GetAgentByHost(hostID)
	if !ok {
		return errors.New("host agent not connected")
	}

	hostUUID, err := uuid.Parse(hostID)
	if err != nil {
		return err
	}

	if len(names) == 0 {
		response, err := m.sendCommand(ctx, agent.ID, "list_volumes", map[string]any{})
		if err != nil {
			return err
		}
		names = extractIDsFromList(response["volumes"], "name")
	}

	if len(names) == 0 {
		return nil
	}

	now := time.Now().UTC()
	batches := chunkStrings(names, m.batchSize)
	for _, batch := range batches {
		response, err := m.sendCommand(ctx, agent.ID, "inspect_volumes", map[string]any{"ids": batch})
		if err != nil {
			return err
		}
		if err := m.persistVolumeSnapshots(ctx, hostUUID, response["volumes"], now); err != nil {
			return err
		}
		m.logAgentErrors("volume", hostUUID, response["errors"])
	}

	return nil
}

// RefreshHostTopology refreshes both networks and volumes for a host.
func (m *Manager) RefreshHostTopology(ctx context.Context, hostID string) {
	if err := m.RefreshNetworks(ctx, hostID, nil); err != nil {
		logrus.WithError(err).WithField("host_id", hostID).Warn("failed to refresh network topology")
	}
	if err := m.RefreshVolumes(ctx, hostID, nil); err != nil {
		logrus.WithError(err).WithField("host_id", hostID).Warn("failed to refresh volume topology")
	}
}

// StartBackgroundRefresh begins a periodic refresh loop.
func (m *Manager) StartBackgroundRefresh(ctx context.Context) {
	if m.refreshInterval <= 0 {
		return
	}

	ticker := time.NewTicker(m.refreshInterval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.refreshAllHosts(ctx)
			}
		}
	}()
}

func (m *Manager) refreshAllHosts(ctx context.Context) {
	if m.db == nil {
		return
	}

	agents := m.hub.GetAgents()
	if len(agents) == 0 {
		return
	}
	for _, agent := range agents {
		select {
		case <-ctx.Done():
			return
		default:
		}
		m.RefreshHostTopology(ctx, agent.HostID)
	}
}

// GetNetworkTopology returns cached network snapshots for a host keyed by network ID.
func (m *Manager) GetNetworkTopology(hostID string) (map[string]database.NetworkTopology, error) {
	hostUUID, err := uuid.Parse(hostID)
	if err != nil {
		return nil, err
	}

	var records []database.NetworkTopology
	if err := m.db.Where("host_id = ?", hostUUID).Find(&records).Error; err != nil {
		return nil, err
	}

	result := make(map[string]database.NetworkTopology, len(records))
	for _, rec := range records {
		result[rec.NetworkID] = rec
	}
	return result, nil
}

// GetVolumeTopology returns cached volume snapshots for a host keyed by volume name.
func (m *Manager) GetVolumeTopology(hostID string) (map[string]database.VolumeTopology, error) {
	hostUUID, err := uuid.Parse(hostID)
	if err != nil {
		return nil, err
	}

	var records []database.VolumeTopology
	if err := m.db.Where("host_id = ?", hostUUID).Find(&records).Error; err != nil {
		return nil, err
	}

	result := make(map[string]database.VolumeTopology, len(records))
	for _, rec := range records {
		result[rec.VolumeName] = rec
	}
	return result, nil
}

// IsStale reports whether the cached snapshot should be considered stale.
func (m *Manager) IsStale(refreshedAt time.Time) bool {
	if refreshedAt.IsZero() {
		return true
	}
	if m.staleAfter <= 0 {
		return false
	}
	return time.Since(refreshedAt) > m.staleAfter
}

// PurgeHost removes cached topology for the specified host.
func (m *Manager) PurgeHost(hostID string) error {
	hostUUID, err := uuid.Parse(hostID)
	if err != nil {
		return err
	}

	if err := m.db.Where("host_id = ?", hostUUID).Delete(&database.NetworkTopology{}).Error; err != nil {
		return err
	}
	if err := m.db.Where("host_id = ?", hostUUID).Delete(&database.VolumeTopology{}).Error; err != nil {
		return err
	}
	return nil
}

func (m *Manager) persistNetworkSnapshots(ctx context.Context, hostID uuid.UUID, payload interface{}, refreshedAt time.Time) error {
	slice, ok := payload.([]interface{})
	if !ok {
		return nil
	}

	for _, item := range slice {
		data, ok := item.(map[string]any)
		if !ok {
			continue
		}
		networkID, _ := data["id"].(string)
		if networkID == "" {
			continue
		}
		snapshot := cloneJSONMap(data)
		if err := m.db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "host_id"}, {Name: "network_id"}},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"snapshot":     database.JSONB(snapshot),
				"refreshed_at": refreshedAt,
				"updated_at":   time.Now().UTC(),
			}),
		}).Create(&database.NetworkTopology{
			HostID:      hostID,
			NetworkID:   networkID,
			Snapshot:    database.JSONB(snapshot),
			RefreshedAt: refreshedAt,
		}).Error; err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) persistVolumeSnapshots(ctx context.Context, hostID uuid.UUID, payload interface{}, refreshedAt time.Time) error {
	slice, ok := payload.([]interface{})
	if !ok {
		return nil
	}

	for _, item := range slice {
		data, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name, _ := data["name"].(string)
		if name == "" {
			continue
		}
		snapshot := cloneJSONMap(data)
		if err := m.db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "host_id"}, {Name: "volume_name"}},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"snapshot":     database.JSONB(snapshot),
				"refreshed_at": refreshedAt,
				"updated_at":   time.Now().UTC(),
			}),
		}).Create(&database.VolumeTopology{
			HostID:      hostID,
			VolumeName:  name,
			Snapshot:    database.JSONB(snapshot),
			RefreshedAt: refreshedAt,
		}).Error; err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) sendCommand(ctx context.Context, agentID string, action string, params map[string]any) (map[string]any, error) {
	commandID := uuid.NewString()
	command := protocol.NewCommand(commandID, action, params)
	if err := m.hub.SendCommand(agentID, command); err != nil {
		return nil, err
	}
	return m.waitForResponse(ctx, agentID, command.ID)
}

func (m *Manager) waitForResponse(ctx context.Context, agentID, commandID string) (map[string]any, error) {
	responseCh := m.hub.SubscribeResponse(commandID)
	defer m.hub.UnsubscribeResponse(commandID)

	timeoutTimer := time.NewTimer(commandTimeout)
	defer timeoutTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeoutTimer.C:
			return nil, protocol.ErrCommandTimeout
		case response := <-responseCh:
			if response == nil || response.AgentID != agentID {
				continue
			}
			if response.Error != nil {
				return nil, response.Error
			}
			if response.Response == nil {
				return map[string]any{"message": "command completed"}, nil
			}
			if data, ok := response.Response.Payload["data"].(map[string]any); ok {
				return data, nil
			}
			return response.Response.Payload, nil
		}
	}
}

func (m *Manager) logAgentErrors(resource string, hostID uuid.UUID, payload interface{}) {
	entries, ok := payload.([]interface{})
	if !ok {
		return
	}
	for _, item := range entries {
		if errMap, ok := item.(map[string]any); ok {
			logrus.WithFields(logrus.Fields{
				"resource_type": resource,
				"host_id":       hostID.String(),
				"resource_id":   errMap["id"],
			}).Warnf("agent reported error: %v", errMap["error"])
		}
	}
}

func extractIDsFromList(value interface{}, keyOverride ...string) []string {
	slice, ok := value.([]interface{})
	if !ok {
		return nil
	}
	key := "id"
	if len(keyOverride) > 0 && keyOverride[0] != "" {
		key = keyOverride[0]
	}
	out := make([]string, 0, len(slice))
	for _, item := range slice {
		if m, ok := item.(map[string]any); ok {
			if id, ok := m[key].(string); ok && id != "" {
				out = append(out, id)
			}
		}
	}
	return out
}

func chunkStrings(values []string, size int) [][]string {
	if size <= 0 || len(values) <= size {
		return [][]string{values}
	}
	result := make([][]string, 0, (len(values)+size-1)/size)
	for i := 0; i < len(values); i += size {
		end := i + size
		if end > len(values) {
			end = len(values)
		}
		chunk := make([]string, end-i)
		copy(chunk, values[i:end])
		result = append(result, chunk)
	}
	return result
}

func cloneJSONMap(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	out := make(map[string]any, len(input))
	for k, v := range input {
		out[k] = v
	}
	return out
}
