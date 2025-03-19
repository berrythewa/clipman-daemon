package clipboard

// ChangeTracker is an optional interface that clipboard implementations can provide
// if they support monitoring clipboard changes efficiently.
type ChangeTracker interface {
	// HasChanged returns true if the clipboard content has changed since the last check
	HasChanged() bool
}

// IsChangeTracker checks if the given clipboard implements the ChangeTracker interface
func IsChangeTracker(c Clipboard) (ChangeTracker, bool) {
	tracker, ok := c.(ChangeTracker)
	return tracker, ok
}

// ContentTypeDetector is an optional interface for clipboard implementations
// that can detect content types natively
type ContentTypeDetector interface {
	// DetectContentType returns the detected content type from the clipboard
	DetectContentType() string
} 