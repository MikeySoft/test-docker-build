package websocket

import "errors"

var (
	ErrAgentNotFound = errors.New("agent not found")
	ErrHostNotFound  = errors.New("host not found")
	ErrAgentNotReady = errors.New("agent not ready")
	ErrInvalidAPIKey = errors.New("invalid API key")
	ErrUnauthorized  = errors.New("unauthorized")
)
