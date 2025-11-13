package config

import (
	"fmt"
	"time"

	"github.com/mikeysoft/flotilla/internal/shared/config"
)

// Config extends the shared server configuration with server-specific fields
type Config struct {
	config.ServerConfig
}

// Load loads server configuration from environment variables
func Load() *Config {
	return &Config{
		ServerConfig: *config.LoadServerConfig(),
	}
}

// GetServerAddress returns the server address in host:port format
func (c *Config) GetServerAddress() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// GetWebSocketReadBufferSize returns the WebSocket read buffer size
func (c *Config) GetWebSocketReadBufferSize() int {
	return c.WSReadBufferSize
}

// GetWebSocketWriteBufferSize returns the WebSocket write buffer size
func (c *Config) GetWebSocketWriteBufferSize() int {
	return c.WSWriteBufferSize
}

// GetWebSocketHandshakeTimeout returns the WebSocket handshake timeout
func (c *Config) GetWebSocketHandshakeTimeout() time.Duration {
	return c.WSHandshakeTimeout
}
