package protocol

// ResourceType represents a generic resource managed via the Flotilla agent.
type ResourceType string

const (
	// ResourceTypeImage represents a Docker image resource.
	ResourceTypeImage ResourceType = "image"
	// ResourceTypeVolume represents a Docker volume resource.
	ResourceTypeVolume ResourceType = "volume"
	// ResourceTypeNetwork represents a Docker network resource.
	ResourceTypeNetwork ResourceType = "network"
)

// ResourceRemovalBlocker describes an entity that is preventing a resource from being removed.
type ResourceRemovalBlocker struct {
	Kind    string            `json:"kind"`
	ID      string            `json:"id,omitempty"`
	Name    string            `json:"name,omitempty"`
	Stack   string            `json:"stack,omitempty"`
	Details map[string]string `json:"details,omitempty"`
}

// ResourceRemovalConflict captures information about a resource removal request that
// could not be completed because other entities still reference the resource.
type ResourceRemovalConflict struct {
	ResourceType   ResourceType             `json:"resource_type"`
	ResourceID     string                   `json:"resource_id,omitempty"`
	ResourceName   string                   `json:"resource_name,omitempty"`
	Reason         string                   `json:"reason"`
	Blockers       []ResourceRemovalBlocker `json:"blockers,omitempty"`
	ForceSupported bool                     `json:"force_supported"`
	OriginalError  string                   `json:"original_error,omitempty"`
}

// ResourceRemovalError represents an unexpected error that occurred while attempting to remove a resource.
type ResourceRemovalError struct {
	ResourceType ResourceType `json:"resource_type"`
	ResourceID   string       `json:"resource_id,omitempty"`
	ResourceName string       `json:"resource_name,omitempty"`
	Message      string       `json:"message"`
}
