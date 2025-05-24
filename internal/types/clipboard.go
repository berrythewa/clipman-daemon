package types

import ( 
	"time"
	"bytes"
)

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

type ClipboardContent struct {
	Type		ContentType
	Data		[]byte
	Created		time.Time
	Compressed	bool
	Hash		string
	Occurrences	[]time.Time
}

func (c1 *ClipboardContent) Equal(c2 *ClipboardContent) bool {
	if c1 == nil || c2 == nil {
		return c1 == c2
	}
	return c1.Type == c2.Type && bytes.Equal(c1.Data, c2.Data)
}

type CustomMimeTypeHandler struct {
    MimeType    string
    TypeID      string
    Description string
    DetectFunc  func([]byte) bool
    ConvertFunc func([]byte, string) ([]byte, error) // optional: convert to another type
}

// OccurrenceStats provides statistics about clipboard content occurrences
type OccurrenceStats struct {
	Hash             string
	TotalOccurrences int
	FirstSeen        time.Time
	LastSeen         time.Time
	ContentType      ContentType
	AverageFrequency time.Duration
}