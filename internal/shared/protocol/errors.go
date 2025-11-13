package protocol

import "errors"

var (
	ErrInvalidMessageType = errors.New("invalid message type")
	ErrInvalidPayload     = errors.New("invalid payload format")
	ErrCommandTimeout     = errors.New("command timeout")
	ErrConnectionClosed   = errors.New("connection closed")
)
