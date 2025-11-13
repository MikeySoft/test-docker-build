package config

import (
	"testing"

	shared "github.com/mikeysoft/flotilla/internal/shared/config"
)

func TestValidate(t *testing.T) {
	cfg := &Config{
		AgentConfig: shared.AgentConfig{
			ServerAddress: "localhost",
			ServerPort:    8080,
			APIKey:        "key",
			AgentName:     "agent",
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}
}

func TestValidateErrors(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
	}{
		{
			name: "missing server address",
			cfg: Config{
				AgentConfig: shared.AgentConfig{
					ServerPort: 8080,
					APIKey:     "key",
					AgentName:  "agent",
				},
			},
		},
		{
			name: "invalid port",
			cfg: Config{
				AgentConfig: shared.AgentConfig{
					ServerAddress: "localhost",
					ServerPort:    -1,
					APIKey:        "key",
					AgentName:     "agent",
				},
			},
		},
		{
			name: "missing api key when not localhost",
			cfg: Config{
				AgentConfig: shared.AgentConfig{
					ServerAddress: "remote",
					ServerPort:    8080,
					AgentName:     "agent",
				},
			},
		},
		{
			name: "missing agent name",
			cfg: Config{
				AgentConfig: shared.AgentConfig{
					ServerAddress: "localhost",
					ServerPort:    8080,
					APIKey:        "key",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.cfg.Validate(); err == nil {
				t.Fatalf("Validate() expected error for %s", tt.name)
			}
		})
	}
}
