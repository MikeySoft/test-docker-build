package config

import (
	"fmt"

	"github.com/mikeysoft/flotilla/internal/shared/config"
)

// Config extends the shared agent configuration with agent-specific fields
type Config struct {
	config.AgentConfig
}

// Load loads agent configuration from environment variables
func Load() *Config {
	return &Config{
		AgentConfig: *config.LoadAgentConfig(),
	}
}

// Validate validates the agent configuration
func (c *Config) Validate() error {
	if c.ServerAddress == "" {
		return fmt.Errorf("server address is required")
	}

	if c.ServerPort <= 0 || c.ServerPort > 65535 {
		return fmt.Errorf("server port must be between 1 and 65535")
	}

	// API key is optional for localhost development
	if c.APIKey == "" && c.ServerAddress != "localhost" {
		return fmt.Errorf("API key is required for non-localhost servers")
	}

	if c.AgentName == "" {
		return fmt.Errorf("agent name is required")
	}

	return nil
}
