package types

import ( 
	"time"
)

// CacheMessage represents a collection of clipboard contents to be published
type CacheMessage struct {
	DeviceID    string
	ContentList []*ClipboardContent
	TotalSize   int64
	Timestamp   time.Time
}
