package database

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Host represents a Docker host managed by an agent
type Host struct {
	ID           uuid.UUID  `gorm:"type:uuid;primary_key;default:uuid_generate_v4()" json:"id"`
	Name         string     `gorm:"not null" json:"name"`
	Description  string     `json:"description"`
	AgentVersion string     `json:"agent_version"`
	LastSeen     *time.Time `json:"last_seen"`
	Status       string     `gorm:"not null;default:'offline'" json:"status"` // online, offline, error
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`

	// Relationships
	Stacks  []Stack  `gorm:"foreignKey:HostID;constraint:OnDelete:CASCADE" json:"stacks,omitempty"`
	APIKeys []APIKey `gorm:"foreignKey:HostID;constraint:OnDelete:SET NULL" json:"api_keys,omitempty"`
}

// Stack represents a Docker Compose stack deployed on a host
type Stack struct {
	ID                uuid.UUID `gorm:"type:uuid;primary_key;default:uuid_generate_v4()" json:"id"`
	HostID            uuid.UUID `gorm:"type:uuid;not null" json:"host_id"`
	Name              string    `gorm:"not null" json:"name"`
	ComposeContent    string    `gorm:"type:text;not null" json:"compose_content"`
	EnvVars           JSONB     `gorm:"type:jsonb" json:"env_vars"`               // Values should be encrypted-at-rest if EnvVarsSensitive
	Status            string    `gorm:"not null;default:'stopped'" json:"status"` // running, stopped, error
	Imported          bool      `gorm:"default:false" json:"imported"`            // Indicates if stack was imported
	EnvVarsSensitive  bool      `gorm:"default:false" json:"env_vars_sensitive"`  // If true, all env_vars MUST be encrypted via AES-GCM
	ManagedByFlotilla bool      `gorm:"default:true" json:"managed_by_flotilla"`  // Managed by Flotilla or manually deployed
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`

	// Relationships
	Host Host `gorm:"foreignKey:HostID;constraint:OnDelete:CASCADE" json:"host,omitempty"`
}

// User represents a system user (for future RBAC)
type User struct {
	ID           uuid.UUID  `gorm:"type:uuid;primary_key;default:uuid_generate_v4()" json:"id"`
	Username     string     `gorm:"uniqueIndex;not null" json:"username"`
	Email        *string    `gorm:"uniqueIndex" json:"email,omitempty"`
	PasswordHash string     `gorm:"not null" json:"-"`
	Role         string     `gorm:"not null;default:'user'" json:"role"` // admin, user, viewer
	IsActive     bool       `gorm:"not null;default:true" json:"is_active"`
	LastLoginAt  *time.Time `json:"last_login_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// APIKey represents an API key for agent authentication
type APIKey struct {
	ID        uuid.UUID  `gorm:"type:uuid;primary_key;default:uuid_generate_v4()" json:"id"`
	KeyHash   string     `gorm:"uniqueIndex;not null" json:"-"`
	Name      string     `gorm:"not null" json:"name"`
	Prefix    *string    `json:"prefix,omitempty"`
	HostID    *uuid.UUID `gorm:"type:uuid" json:"host_id,omitempty"`
	CreatedBy *uuid.UUID `gorm:"type:uuid" json:"created_by,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	LastUsed  *time.Time `json:"last_used,omitempty"`
	IsActive  bool       `gorm:"not null;default:true" json:"is_active"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`

	// Relationships
	Host *Host `gorm:"foreignKey:HostID;constraint:OnDelete:SET NULL" json:"host,omitempty"`
}

// RefreshToken tracks refresh token rotation and status
type RefreshToken struct {
	ID        uuid.UUID  `gorm:"type:uuid;primary_key;default:uuid_generate_v4()" json:"id"`
	UserID    uuid.UUID  `gorm:"type:uuid;not null" json:"user_id"`
	FamilyID  uuid.UUID  `gorm:"type:uuid;not null" json:"family_id"`
	TokenID   uuid.UUID  `gorm:"type:uuid;not null" json:"token_id"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt time.Time  `json:"expires_at"`
	UserAgent *string    `json:"user_agent,omitempty"`
	IP        *string    `json:"ip,omitempty"`
}

func (RefreshToken) TableName() string { return "refresh_tokens" }

// AuditLog records security-sensitive events
type AuditLog struct {
	ID          uuid.UUID  `gorm:"type:uuid;primary_key;default:uuid_generate_v4()" json:"id"`
	ActorUserID *uuid.UUID `gorm:"type:uuid" json:"actor_user_id,omitempty"`
	Action      string     `gorm:"size:128;not null" json:"action"`
	TargetType  *string    `gorm:"size:64" json:"target_type,omitempty"`
	TargetID    *string    `json:"target_id,omitempty"`
	IP          *string    `gorm:"size:64" json:"ip,omitempty"`
	UserAgent   *string    `json:"user_agent,omitempty"`
	Metadata    JSONB      `gorm:"type:jsonb" json:"metadata,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

func (AuditLog) TableName() string { return "audit_logs" }

// JSONB is a custom type for PostgreSQL JSONB fields
type JSONB map[string]interface{}

// Scan implements the sql.Scanner interface for JSONB
func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = make(map[string]interface{})
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("unsupported type %T for JSONB scan", value)
	}

	if len(bytes) == 0 {
		*j = make(map[string]interface{})
		return nil
	}

	var data map[string]interface{}
	if err := json.Unmarshal(bytes, &data); err != nil {
		return err
	}

	*j = JSONB(data)
	return nil
}

// Value implements the driver.Valuer interface for JSONB
func (j JSONB) Value() (interface{}, error) {
	if j == nil {
		return nil, nil
	}
	bytes, err := json.Marshal(map[string]interface{}(j))
	if err != nil {
		return nil, err
	}
	return bytes, nil
}

// TableName returns the table name for the Host model
func (Host) TableName() string {
	return "hosts"
}

// TableName returns the table name for the Stack model
func (Stack) TableName() string {
	return "stacks"
}

// TableName returns the table name for the User model
func (User) TableName() string {
	return "users"
}

// TableName returns the table name for the APIKey model
func (APIKey) TableName() string {
	return "api_keys"
}

// BeforeCreate is called before creating a record
func (h *Host) BeforeCreate(tx *gorm.DB) error {
	if h.ID == uuid.Nil {
		h.ID = uuid.New()
	}
	return nil
}

func (s *Stack) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}

func (a *APIKey) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}

// DashboardTask represents actionable items surfaced on the dashboard
type DashboardTask struct {
	ID             uuid.UUID  `gorm:"type:uuid;primary_key;default:uuid_generate_v4()" json:"id"`
	Title          string     `gorm:"not null" json:"title"`
	Description    string     `json:"description"`
	Status         string     `gorm:"not null;default:'open'" json:"status"`   // open, acknowledged, resolved, dismissed
	Severity       string     `gorm:"not null;default:'info'" json:"severity"` // info, warning, critical
	Source         string     `gorm:"not null;default:'system'" json:"source"` // system, manual
	Category       string     `json:"category"`                                // e.g., host, stack, container
	TaskType       string     `json:"task_type"`                               // e.g., host_offline
	Fingerprint    string     `gorm:"index:idx_dashboard_tasks_fingerprint" json:"fingerprint"`
	HostID         *uuid.UUID `gorm:"type:uuid" json:"host_id,omitempty"`
	StackID        *uuid.UUID `gorm:"type:uuid" json:"stack_id,omitempty"`
	ContainerID    *string    `json:"container_id,omitempty"`
	Metadata       JSONB      `gorm:"type:jsonb" json:"metadata"`
	DueAt          *time.Time `json:"due_at,omitempty"`
	SnoozedUntil   *time.Time `json:"snoozed_until,omitempty"`
	AcknowledgedAt *time.Time `json:"acknowledged_at,omitempty"`
	ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
	CreatedBy      *uuid.UUID `gorm:"type:uuid" json:"created_by,omitempty"`
	AcknowledgedBy *uuid.UUID `gorm:"type:uuid" json:"acknowledged_by,omitempty"`
	ResolvedBy     *uuid.UUID `gorm:"type:uuid" json:"resolved_by,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// NetworkTopology stores cached network inspection data for a host.
type NetworkTopology struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;default:uuid_generate_v4()" json:"id"`
	HostID      uuid.UUID `gorm:"type:uuid;not null;index:idx_network_topology_host_network,unique" json:"host_id"`
	NetworkID   string    `gorm:"not null;index:idx_network_topology_host_network,unique" json:"network_id"`
	Snapshot    JSONB     `gorm:"type:jsonb;not null" json:"snapshot"`
	RefreshedAt time.Time `json:"refreshed_at"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// VolumeTopology stores cached volume inspection data for a host.
type VolumeTopology struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;default:uuid_generate_v4()" json:"id"`
	HostID      uuid.UUID `gorm:"type:uuid;not null;index:idx_volume_topology_host_volume,unique" json:"host_id"`
	VolumeName  string    `gorm:"not null;index:idx_volume_topology_host_volume,unique" json:"volume_name"`
	Snapshot    JSONB     `gorm:"type:jsonb;not null" json:"snapshot"`
	RefreshedAt time.Time `json:"refreshed_at"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (n *NetworkTopology) BeforeCreate(tx *gorm.DB) error {
	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}
	return nil
}

func (v *VolumeTopology) BeforeCreate(tx *gorm.DB) error {
	if v.ID == uuid.Nil {
		v.ID = uuid.New()
	}
	return nil
}

func (t *DashboardTask) BeforeCreate(tx *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	if t.Metadata == nil {
		t.Metadata = JSONB{}
	}
	if t.Status == "" {
		t.Status = "open"
	}
	if t.Severity == "" {
		t.Severity = "info"
	}
	if t.Source == "" {
		t.Source = "system"
	}
	return nil
}

func (t *DashboardTask) BeforeSave(tx *gorm.DB) error {
	if t.Metadata == nil {
		t.Metadata = JSONB{}
	}
	return nil
}

func (DashboardTask) TableName() string {
	return "dashboard_tasks"
}
