package domain

import "encoding/json"

// Message is the wire format between client and server.
type Message struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}
