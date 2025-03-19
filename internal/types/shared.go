package types

import ( 
	"time"
)

// CacheMessage represents a collection of clipboard contents to be published
type CacheMessage struct {
	DeviceID  string
	Contents  []*ClipboardContent
	Timestamp time.Time
}
