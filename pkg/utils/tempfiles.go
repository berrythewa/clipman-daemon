package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// CreateTempFile creates a temp file in the given dataDir with a unique name and extension.
// Returns the full path and the open file handle.
func CreateTempFile(dataDir, prefix, ext string) (string, *os.File, error) {
	// Use UUID for uniqueness, or timestamp if you prefer
	id := utils.NewUUID() // or time.Now().Format("20060102_150405.000")
	name := fmt.Sprintf("%s_%s%s", prefix, id, ext)
	fullPath := filepath.Join(dataDir, name)
	f, err := os.Create(fullPath)
	if err != nil {
		return "", nil, err
	}
	return fullPath, f, nil
}

// RemoveTempFile safely deletes a temp file.
func RemoveTempFile(path string) error {
	return os.Remove(path)
}

// RemoveAllTempFiles removes all temp files in tempDir matching the given prefix and extension.
// Example: RemoveAllTempFiles("/tmp/clipman", "clip", ".tmp") removes all files like clip_*.tmp
func RemoveAllTempFiles(tempDir, prefix, ext string) error {
	// Ensure the directory exists
	if _, err := os.Stat(tempDir); os.IsNotExist(err) {
		return nil // Nothing to clean
	}

	pattern := filepath.Join(tempDir, fmt.Sprintf("%s_*%s", prefix, ext))
	files, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("failed to glob temp files: %w", err)
	}
	if len(files) == 0 {
		return nil // Nothing to remove
	}

	var firstErr error
	for _, file := range files {
		if err := os.Remove(file); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("failed to remove temp file %s: %w", file, err)
		}
	}
	return firstErr
}

// RemoveAllTempFiles removes all temp files in tempDir matching the given prefix and extension.
// Returns a combined error if any files could not be deleted.
func RemoveAllTempFiles_Aggregated(tempDir, prefix, ext string) error {
	if _, err := os.Stat(tempDir); os.IsNotExist(err) {
		return nil
	}
	pattern := filepath.Join(tempDir, fmt.Sprintf("%s_*%s", prefix, ext))
	files, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("failed to glob temp files: %w", err)
	}
	if len(files) == 0 {
		return nil
	}

	var errs []error
	for _, file := range files {
		if err := os.Remove(file); err != nil {
			errs = append(errs, fmt.Errorf("failed to remove %s: %w", file, err))
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...) // Go 1.20+; for older Go, join as a string
	}
	return nil
}


// TempFileExists checks if a temp file exists.
func TempFileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}