package types

import (
	"bytes"
	"time"
)

// ContentType represents the type of clipboard content
type ContentType string

const (
	TypeUnknown  ContentType = "unknown"
	TypeString   ContentType = "string"
	TypeText     ContentType = "text"
	TypeImage    ContentType = "image"
	TypeFile     ContentType = "file"
	TypeURL      ContentType = "url"
	TypeFilePath ContentType = "filepath"
	TypeHTML     ContentType = "html"
	TypeRTF      ContentType = "rtf"
)

// ClipboardContent represents a clipboard item
type ClipboardContent struct {
	Type        ContentType   `json:"type"`
	Data        []byte        `json:"data"`
	Created     time.Time     `json:"created"`
	DeviceID    string        `json:"device_id,omitempty"`
	Hash        string        `json:"hash,omitempty"`
	Compressed  bool          `json:"compressed,omitempty"`
	Occurrences []time.Time   `json:"occurrences,omitempty"`
}

// Equal compares two ClipboardContent instances for equality
func (c1 *ClipboardContent) Equal(c2 *ClipboardContent) bool {
	if c1 == nil || c2 == nil {
		return c1 == c2
	}
	return c1.Type == c2.Type && bytes.Equal(c1.Data, c2.Data)
}

// CustomMimeTypeHandler defines handlers for custom MIME types
type CustomMimeTypeHandler struct {
	MimeType    string `json:"mime_type"`
	TypeID      string `json:"type_id"`
	Description string `json:"description"`
	DetectFunc  func([]byte) bool
	ConvertFunc func([]byte, string) ([]byte, error) // optional: convert to another type
}

// OccurrenceStats represents statistics about content occurrences
type OccurrenceStats struct {
	Hash             string        `json:"hash"`
	TotalOccurrences int           `json:"total_occurrences"`
	FirstSeen        time.Time     `json:"first_seen"`
	LastSeen         time.Time     `json:"last_seen"`
	AverageFrequency time.Duration `json:"average_frequency"`
	ContentType      ContentType   `json:"content_type"`
}

// MonitoringStatus represents the current state of clipboard monitoring
type MonitoringStatus struct {
	IsRunning    bool      `json:"is_running"`
	Mode         string    `json:"mode"`
	LastActivity time.Time `json:"last_activity"`
	ErrorCount   int       `json:"error_count"`
	LastError    string    `json:"last_error"`
} 