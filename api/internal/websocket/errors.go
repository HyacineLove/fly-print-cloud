package websocket

import "errors"

var (
	ErrNodeNotConnected     = errors.New("edge node not connected")
	ErrConnectionClosed     = errors.New("connection closed")
	ErrAckTimeout           = errors.New("command acknowledgement timeout")
	ErrInvalidMessage       = errors.New("invalid message format")
	ErrAuthenticationFailed = errors.New("authentication failed")
)
