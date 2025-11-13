package docker

import (
	"context"
	"io"
	"strconv"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/sirupsen/logrus"
)

// LogStreamer handles container log streaming
type LogStreamer struct {
	client DockerAPI
}

// NewLogStreamer creates a new log streamer
func NewLogStreamer(client DockerAPI) *LogStreamer {
	return &LogStreamer{
		client: client,
	}
}

// LogOptions represents options for log streaming
type LogOptions struct {
	Follow     bool
	Tail       string
	Timestamps bool
	Since      string
	Until      string
}

// LogChunk represents a chunk of log data
type LogChunk struct {
	Data      string    `json:"data"`
	Timestamp time.Time `json:"timestamp"`
	Stream    string    `json:"stream"` // "stdout" or "stderr"
}

// StreamLogs streams container logs and sends them via the provided callback
func (ls *LogStreamer) StreamLogs(ctx context.Context, containerID string, options LogOptions, callback func(LogChunk) error) error {
	logrus.Debugf("Starting log stream for container %s with options: %+v", containerID, options)

	// Prepare Docker log options
	dockerOptions := types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     options.Follow,
		Timestamps: options.Timestamps,
	}

	// Set tail option
	if options.Tail != "" {
		if tailNum, err := strconv.Atoi(options.Tail); err == nil && tailNum > 0 {
			dockerOptions.Tail = strconv.Itoa(tailNum)
		}
	}

	// Set since option
	if options.Since != "" {
		dockerOptions.Since = options.Since
	}

	// Set until option
	if options.Until != "" {
		dockerOptions.Until = options.Until
	}

	// Get log stream from Docker
	reader, err := ls.client.ContainerLogs(ctx, containerID, dockerOptions)
	if err != nil {
		logrus.Errorf("Failed to get container logs for %s: %v", containerID, err)
		return err
	}
	defer reader.Close()

	// Stream logs
	buffer := make([]byte, 1024)
	for {
		select {
		case <-ctx.Done():
			logrus.Debugf("Log stream context cancelled for container %s", containerID)
			return ctx.Err()
		default:
			n, err := reader.Read(buffer)
			if err != nil {
				if err == io.EOF {
					logrus.Debugf("Log stream ended for container %s", containerID)
					return nil
				}
				logrus.Errorf("Error reading log stream for container %s: %v", containerID, err)
				return err
			}

			if n > 0 {
				// Parse Docker log format (8-byte header + data)
				data := buffer[:n]
				chunks := ls.parseLogChunks(data)

				// Send each chunk via callback
				for _, chunk := range chunks {
					if err := callback(chunk); err != nil {
						logrus.Errorf("Error sending log chunk for container %s: %v", containerID, err)
						return err
					}
				}
			}
		}
	}
}

// parseLogChunks parses Docker log format and extracts all log entries
// Docker log format: multiple [8-byte header][log data] entries concatenated
// Header format: [4 bytes stream type][4 bytes size]
func (ls *LogStreamer) parseLogChunks(data []byte) []LogChunk {
	var chunks []LogChunk
	offset := 0

	for offset < len(data) {
		// Need at least 8 bytes for header
		if len(data)-offset < 8 {
			// If there's leftover data that doesn't form a complete header, treat it as raw stdout
			if len(data)-offset > 0 {
				chunks = append(chunks, LogChunk{
					Data:      string(data[offset:]),
					Timestamp: time.Now(),
					Stream:    "stdout",
				})
			}
			break
		}

		// Extract stream type (first 4 bytes)
		streamType := data[offset]
		stream := "stdout"
		if streamType == 2 {
			stream = "stderr"
		}

		// Extract size (next 4 bytes, big-endian)
		size := int(data[offset+4])<<24 | int(data[offset+5])<<16 | int(data[offset+6])<<8 | int(data[offset+7])

		// Move past header
		offset += 8

		// Extract log data
		if size > 0 && offset+size <= len(data) {
			logData := data[offset : offset+size]
			chunks = append(chunks, LogChunk{
				Data:      string(logData),
				Timestamp: time.Now(),
				Stream:    stream,
			})
			offset += size
		} else if size > 0 {
			// Size is larger than remaining data, take what we have
			logData := data[offset:]
			chunks = append(chunks, LogChunk{
				Data:      string(logData),
				Timestamp: time.Now(),
				Stream:    stream,
			})
			break
		}
	}

	return chunks
}

// GetLogs gets a snapshot of container logs without streaming
func (ls *LogStreamer) GetLogs(ctx context.Context, containerID string, options LogOptions) ([]LogChunk, error) {
	var chunks []LogChunk

	err := ls.StreamLogs(ctx, containerID, options, func(chunk LogChunk) error {
		chunks = append(chunks, chunk)
		return nil
	})

	return chunks, err
}
