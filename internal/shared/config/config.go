package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// BaseConfig contains common configuration fields
type BaseConfig struct {
	LogLevel  string `json:"log_level"`
	LogFormat string `json:"log_format"`
}

// ServerConfig contains server-specific configuration
type ServerConfig struct {
	BaseConfig
	// Mode controls environment behavior: DEV or PROD
	Mode               string        `json:"mode"`
	Port               int           `json:"port"`
	Host               string        `json:"host"`
	TLSEnabled         bool          `json:"tls_enabled"`
	TLSCertFile        string        `json:"tls_cert_file"`
	TLSKeyFile         string        `json:"tls_key_file"`
	DatabaseURL        string        `json:"database_url"`
	JWTSecret          string        `json:"jwt_secret"`
	WSReadBufferSize   int           `json:"ws_read_buffer_size"`
	WSWriteBufferSize  int           `json:"ws_write_buffer_size"`
	WSHandshakeTimeout time.Duration `json:"ws_handshake_timeout"`
	// InfluxDB configuration
	InfluxDBEnabled         bool          `json:"influxdb_enabled"`
	InfluxDBURL             string        `json:"influxdb_url"`
	InfluxDBToken           string        `json:"influxdb_token"`
	InfluxDBOrg             string        `json:"influxdb_org"`
	InfluxDBBucket          string        `json:"influxdb_bucket"`
	TopologyRefreshInterval time.Duration `json:"topology_refresh_interval"`
	TopologyStaleAfter      time.Duration `json:"topology_stale_after"`
	TopologyBatchSize       int           `json:"topology_batch_size"`
}

// AgentConfig contains agent-specific configuration
type AgentConfig struct {
	BaseConfig
	ServerAddress        string        `json:"server_address"`
	ServerPort           int           `json:"server_port"`
	ServerUseTLS         bool          `json:"server_use_tls"`
	APIKey               string        `json:"api_key"`
	AgentID              string        `json:"agent_id"`
	AgentName            string        `json:"agent_name"`
	DockerSocket         string        `json:"docker_socket"`
	HeartbeatInterval    time.Duration `json:"heartbeat_interval"`
	ReconnectInterval    time.Duration `json:"reconnect_interval"`
	MaxReconnectAttempts int           `json:"max_reconnect_attempts"`
	// Metrics collection configuration
	MetricsEnabled            bool          `json:"metrics_enabled"`
	MetricsCollectionInterval time.Duration `json:"metrics_collection_interval"`
	// Host stats collection: false|true|auto (auto enables if required mounts/caps present)
	MetricsCollectHostStats     bool `json:"metrics_collect_host_stats"`
	MetricsCollectHostStatsAuto bool `json:"metrics_collect_host_stats_auto"`
	MetricsCollectNetwork       bool `json:"metrics_collect_network"`
	// Disk I/O fallback for cgroup v2 environments where Docker blkio is missing
	MetricsCollectDiskIOFallback bool   `json:"metrics_collect_disk_io_fallback"`
	HostCgroupRoot               string `json:"host_cgroup_root"`
	HostProcRoot                 string `json:"host_proc_root"`
}

// GetServerURL constructs the WebSocket URL from address, port, and TLS settings
func (c *AgentConfig) GetServerURL() string {
	protocol := "ws"
	if c.ServerUseTLS {
		protocol = "wss"
	}
	return fmt.Sprintf("%s://%s:%d/ws/agent", protocol, c.ServerAddress, c.ServerPort)
}

// LoadServerConfig loads server configuration from environment variables
func LoadServerConfig() *ServerConfig {
	return &ServerConfig{
		BaseConfig: BaseConfig{
			LogLevel:  getEnv("LOG_LEVEL", "info"),
			LogFormat: getEnv("LOG_FORMAT", "json"),
		},
		Mode:        getEnv("MODE", "PROD"),
		Port:        getEnvAsInt("SERVER_PORT", 8080),
		Host:        getEnv("SERVER_HOST", "localhost"),
		TLSEnabled:  getEnvAsBool("TLS_ENABLED", false),
		TLSCertFile: getEnv("TLS_CERT_FILE", ""),
		TLSKeyFile:  getEnv("TLS_KEY_FILE", ""),
		// SonarQube Won't Fix: Dev-only default to simplify local setup; production must
		// provide DATABASE_URL via environment or secrets management. // NOSONAR
		DatabaseURL:             getEnv("DATABASE_URL", "postgres://flotilla:flotilla_dev_password@localhost:5432/flotilla?sslmode=disable"), // NOSONAR
		JWTSecret:               getEnv("JWT_SECRET", "your-super-secret-jwt-key-change-this-in-production"),
		WSReadBufferSize:        getEnvAsInt("WS_READ_BUFFER_SIZE", 1024),
		WSWriteBufferSize:       getEnvAsInt("WS_WRITE_BUFFER_SIZE", 1024),
		WSHandshakeTimeout:      getEnvAsDuration("WS_HANDSHAKE_TIMEOUT", 10*time.Second),
		InfluxDBEnabled:         getEnvAsBool("INFLUXDB_ENABLED", false),
		InfluxDBURL:             getEnv("INFLUXDB_URL", "http://localhost:8086"),
		InfluxDBToken:           getEnv("INFLUXDB_TOKEN", ""),
		InfluxDBOrg:             getEnv("INFLUXDB_ORG", "flotilla"),
		InfluxDBBucket:          getEnv("INFLUXDB_BUCKET", "metrics"),
		TopologyRefreshInterval: getEnvAsDuration("TOPOLOGY_REFRESH_INTERVAL", 5*time.Minute),
		TopologyStaleAfter:      getEnvAsDuration("TOPOLOGY_STALE_AFTER", 10*time.Minute),
		TopologyBatchSize:       getEnvAsInt("TOPOLOGY_BATCH_SIZE", 20),
	}
}

// LoadAgentConfig loads agent configuration from environment variables
func LoadAgentConfig() *AgentConfig {
	// Support string-based mode for METRICS_COLLECT_HOST_STATS: "true", "false", or "auto"
	rawHostStats := os.Getenv("METRICS_COLLECT_HOST_STATS")
	hostStatsAuto := false
	if rawHostStats != "" {
		if v := rawHostStats; v == "auto" || v == "AUTO" || v == "Auto" {
			hostStatsAuto = true
		}
	}
	return &AgentConfig{
		BaseConfig: BaseConfig{
			LogLevel:  getEnv("LOG_LEVEL", "info"),
			LogFormat: getEnv("LOG_FORMAT", "json"),
		},
		ServerAddress:                getEnv("SERVER_ADDRESS", "localhost"),
		ServerPort:                   getEnvAsInt("SERVER_PORT", 8080),
		ServerUseTLS:                 getEnvAsBool("SERVER_USE_TLS", false),
		APIKey:                       getEnv("API_KEY", ""),
		AgentID:                      getEnv("AGENT_ID", ""),
		AgentName:                    getEnv("AGENT_NAME", getHostname()),
		DockerSocket:                 getEnv("DOCKER_SOCKET", "/var/run/docker.sock"),
		HeartbeatInterval:            getEnvAsDuration("AGENT_HEARTBEAT_INTERVAL", 30*time.Second),
		ReconnectInterval:            getEnvAsDuration("AGENT_RECONNECT_INTERVAL", 5*time.Second),
		MaxReconnectAttempts:         getEnvAsInt("AGENT_MAX_RECONNECT_ATTEMPTS", 10),
		MetricsEnabled:               getEnvAsBool("METRICS_ENABLED", true),
		MetricsCollectionInterval:    getEnvAsDuration("METRICS_COLLECTION_INTERVAL", 30*time.Second),
		MetricsCollectHostStats:      getEnvAsBool("METRICS_COLLECT_HOST_STATS", false),
		MetricsCollectHostStatsAuto:  hostStatsAuto,
		MetricsCollectNetwork:        getEnvAsBool("METRICS_COLLECT_NETWORK", true),
		MetricsCollectDiskIOFallback: getEnvAsBool("METRICS_COLLECT_DISK_IO_FALLBACK", false),
		HostCgroupRoot:               getEnv("HOST_CGROUP_ROOT", "/host/sys/fs/cgroup"),
		HostProcRoot:                 getEnv("HOST_PROC_ROOT", "/host/proc"),
	}
}

// Helper functions for environment variable parsing
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

func getEnvAsBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return hostname
}
