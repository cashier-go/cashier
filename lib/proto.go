package lib

import "time"

// SignRequest represents a signing request sent to the server.
type SignRequest struct {
	Key        string    `json:"key"`
	ValidUntil time.Time `json:"valid_until"`
	Message    string    `json:"message"`
	Version    string    `json:"version"`
}

// SignResponse is sent by the server.
type SignResponse struct {
	Status   string `json:"status"`   // Status will be "ok" or "error".
	Response string `json:"response"` // Response will contain either the signed certificate or the error message.
	Version  string `json:"version"`
}
