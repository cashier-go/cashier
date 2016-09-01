package lib

import "time"

// SignRequest represents a signing request sent to the server.
type SignRequest struct {
	Key        string    `json:"key"`
	ValidUntil time.Time `json:"valid_until"`
}

// SignResponse is sent by the server.
// `Status' is "ok" or "error".
// `Response' contains a signed certificate or an error message.
type SignResponse struct {
	Status   string `json:"status"`
	Response string `json:"response"`
}
