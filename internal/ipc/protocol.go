package ipc

// Request represents a command sent from the CLI to the daemon.
type Request struct {
	Command string                 `json:"command"` // e.g. "history", "flush", "write"
	Args    map[string]interface{} `json:"args,omitempty"` // Command-specific arguments
}

// Response represents a reply from the daemon to the CLI.
type Response struct {
	Status  string      `json:"status"` // "ok" or "error"
	Message string      `json:"message,omitempty"` // Human-readable message or error
	Data    interface{} `json:"data,omitempty"`    // Command-specific data (history, etc.)
}

// Example usage:
// req := &Request{Command: "history", Args: map[string]interface{}{ "limit": 10 }}
// resp := &Response{Status: "ok", Data: historySlice} 