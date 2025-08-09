package storage

import (
	"fmt"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/types"
	"go.uber.org/zap"
)

// StorageStats represents comprehensive storage statistics
type StorageStats struct {
	TotalItems       int64                        `json:"total_items"`
	TotalSize        int64                        `json:"total_size_bytes"`
	AverageSize      float64                      `json:"average_size_bytes"`
	ByType           map[types.ContentType]TypeStats `json:"by_type"`
	ByDevice         map[string]DeviceStats       `json:"by_device"`
	OldestItem       time.Time                    `json:"oldest_item"`
	NewestItem       time.Time                    `json:"newest_item"`
	CompressedItems  int64                        `json:"compressed_items"`
	CompressionRatio float64                      `json:"compression_ratio"`
	TopOccurrences   []types.OccurrenceStats      `json:"top_occurrences"`
}

// TypeStats represents statistics for a specific content type
type TypeStats struct {
	Count       int64   `json:"count"`
	TotalSize   int64   `json:"total_size_bytes"`
	AverageSize float64 `json:"average_size_bytes"`
	MinSize     int64   `json:"min_size_bytes"`
	MaxSize     int64   `json:"max_size_bytes"`
}

// DeviceStats represents statistics for a specific device
type DeviceStats struct {
	Count       int64     `json:"count"`
	TotalSize   int64     `json:"total_size_bytes"`
	FirstSeen   time.Time `json:"first_seen"`
	LastSeen    time.Time `json:"last_seen"`
}

// OccurrenceStats represents statistics about content occurrences (from types.go)
// Already defined in types.go, so we can use it directly

// IStorageStats defines interface for storage statistics operations
type IStorageStats interface {
	GetStats() (*StorageStats, error)
	GetTypeStats() (map[types.ContentType]TypeStats, error)
	GetDeviceStats() (map[string]DeviceStats, error)
	GetTopOccurrences(limit int) ([]types.OccurrenceStats, error)
	GetStorageUsage() (int64, error)
}

// GetStats calculates comprehensive storage statistics
func (s *BoltStorage) GetStats() (*StorageStats, error) {
	s.logger.Debug("Calculating comprehensive storage statistics")

	stats := &StorageStats{
		ByType:   make(map[types.ContentType]TypeStats),
		ByDevice: make(map[string]DeviceStats),
		OldestItem: time.Now(),
		NewestItem: time.Time{},
	}

	// Get all content for analysis
	allContent, err := s.Query(QueryOptions{})
	if err != nil {
		s.logger.Error("Failed to retrieve content for stats calculation", zap.Error(err))
		return nil, fmt.Errorf("failed to retrieve content: %w", err)
	}

	if len(allContent) == 0 {
		s.logger.Debug("No content found, returning empty stats")
		return stats, nil
	}

	// Process each content item
	var totalSize, totalCompressed int64
	typeData := make(map[types.ContentType][]int64) // For size calculations
	deviceData := make(map[string][]time.Time)      // For device timing
	occurrenceData := make([]types.OccurrenceStats, 0)

	for _, content := range allContent {
		contentSize := int64(len(content.Data))
		totalSize += contentSize
		stats.TotalItems++

		// Track oldest/newest
		if content.Created.Before(stats.OldestItem) {
			stats.OldestItem = content.Created
		}
		if content.Created.After(stats.NewestItem) {
			stats.NewestItem = content.Created
		}

		// Compression stats
		if content.Compressed {
			stats.CompressedItems++
			totalCompressed += contentSize
		}

		// Type statistics
		if _, exists := typeData[content.Type]; !exists {
			typeData[content.Type] = make([]int64, 0)
		}
		typeData[content.Type] = append(typeData[content.Type], contentSize)

		// Device statistics
		if content.DeviceID != "" {
			if _, exists := deviceData[content.DeviceID]; !exists {
				deviceData[content.DeviceID] = make([]time.Time, 0)
			}
			deviceData[content.DeviceID] = append(deviceData[content.DeviceID], content.Created)
		}

		// Occurrence statistics
		if len(content.Occurrences) > 0 {
			occStat := types.OccurrenceStats{
				Hash:             content.Hash,
				TotalOccurrences: len(content.Occurrences),
				FirstSeen:        content.Created,
				ContentType:      content.Type,
			}

			// Find latest occurrence
			latestOcc := content.Created
			for _, occ := range content.Occurrences {
				if occ.After(latestOcc) {
					latestOcc = occ
				}
			}
			occStat.LastSeen = latestOcc

			// Calculate average frequency
			if len(content.Occurrences) > 1 {
				totalDuration := latestOcc.Sub(content.Created)
				occStat.AverageFrequency = totalDuration / time.Duration(len(content.Occurrences)-1)
			}

			occurrenceData = append(occurrenceData, occStat)
		}
	}

	// Calculate overall statistics
	stats.TotalSize = totalSize
	if stats.TotalItems > 0 {
		stats.AverageSize = float64(totalSize) / float64(stats.TotalItems)
	}
	if totalCompressed > 0 && totalSize > 0 {
		stats.CompressionRatio = float64(totalCompressed) / float64(totalSize)
	}

	// Calculate type statistics
	for contentType, sizes := range typeData {
		typeStat := TypeStats{
			Count: int64(len(sizes)),
		}

		var totalTypeSize int64
		minSize := int64(^uint64(0) >> 1) // Max int64
		maxSize := int64(0)

		for _, size := range sizes {
			totalTypeSize += size
			if size < minSize {
				minSize = size
			}
			if size > maxSize {
				maxSize = size
			}
		}

		typeStat.TotalSize = totalTypeSize
		typeStat.AverageSize = float64(totalTypeSize) / float64(len(sizes))
		typeStat.MinSize = minSize
		typeStat.MaxSize = maxSize

		stats.ByType[contentType] = typeStat
	}

	// Calculate device statistics
	for deviceID, timestamps := range deviceData {
		deviceStat := DeviceStats{
			Count: int64(len(timestamps)),
		}

		// Find first and last seen
		firstSeen := time.Now()
		lastSeen := time.Time{}
		for _, ts := range timestamps {
			if ts.Before(firstSeen) {
				firstSeen = ts
			}
			if ts.After(lastSeen) {
				lastSeen = ts
			}
		}

		deviceStat.FirstSeen = firstSeen
		deviceStat.LastSeen = lastSeen

		// Calculate total size for this device
		deviceContent, err := s.Query(QueryOptions{
			DeviceIDs: []string{deviceID},
		})
		if err == nil {
			for _, content := range deviceContent {
				deviceStat.TotalSize += int64(len(content.Data))
			}
		}

		stats.ByDevice[deviceID] = deviceStat
	}

	// Sort and limit occurrence statistics
	stats.TopOccurrences = s.getTopOccurrences(occurrenceData, 10)

	s.logger.Debug("Storage statistics calculated successfully",
		zap.Int64("total_items", stats.TotalItems),
		zap.Int64("total_size", stats.TotalSize),
		zap.Int("content_types", len(stats.ByType)),
		zap.Int("devices", len(stats.ByDevice)))

	return stats, nil
}

// GetTypeStats returns statistics grouped by content type
func (s *BoltStorage) GetTypeStats() (map[types.ContentType]TypeStats, error) {
	stats, err := s.GetStats()
	if err != nil {
		return nil, err
	}
	return stats.ByType, nil
}

// GetDeviceStats returns statistics grouped by device ID
func (s *BoltStorage) GetDeviceStats() (map[string]DeviceStats, error) {
	stats, err := s.GetStats()
	if err != nil {
		return nil, err
	}
	return stats.ByDevice, nil
}

// GetTopOccurrences returns the most frequently occurring content items
func (s *BoltStorage) GetTopOccurrences(limit int) ([]types.OccurrenceStats, error) {
	stats, err := s.GetStats()
	if err != nil {
		return nil, err
	}
	return stats.TopOccurrences, nil
}

// GetStorageUsage returns total storage usage in bytes
func (s *BoltStorage) GetStorageUsage() (int64, error) {
	stats, err := s.GetStats()
	if err != nil {
		return 0, err
	}
	return stats.TotalSize, nil
}

// getTopOccurrences sorts and limits occurrence statistics
func (s *BoltStorage) getTopOccurrences(occurrenceData []types.OccurrenceStats, limit int) []types.OccurrenceStats {
	if len(occurrenceData) == 0 {
		return []types.OccurrenceStats{}
	}

	// Sort by total occurrences (descending)
	for i := 0; i < len(occurrenceData)-1; i++ {
		for j := i + 1; j < len(occurrenceData); j++ {
			if occurrenceData[j].TotalOccurrences > occurrenceData[i].TotalOccurrences {
				occurrenceData[i], occurrenceData[j] = occurrenceData[j], occurrenceData[i]
			}
		}
	}

	// Return top N items
	if limit > 0 && len(occurrenceData) > limit {
		return occurrenceData[:limit]
	}
	return occurrenceData
}

// GetStatsSummary returns a formatted summary of storage statistics
func (s *BoltStorage) GetStatsSummary() (string, error) {
	stats, err := s.GetStats()
	if err != nil {
		return "", err
	}

	summary := fmt.Sprintf(`Storage Statistics Summary:
`)
	summary += fmt.Sprintf(`  Total Items: %d
`, stats.TotalItems)
	summary += fmt.Sprintf(`  Total Size: %d bytes (%.2f MB)
`, stats.TotalSize, float64(stats.TotalSize)/(1024*1024))
	summary += fmt.Sprintf(`  Average Size: %.1f bytes
`, stats.AverageSize)
	summary += fmt.Sprintf(`  Compressed Items: %d (%.1f%%)
`, stats.CompressedItems, float64(stats.CompressedItems)/float64(stats.TotalItems)*100)

	if !stats.OldestItem.IsZero() && !stats.NewestItem.IsZero() {
		summary += fmt.Sprintf(`  Date Range: %s to %s
`, stats.OldestItem.Format("2006-01-02"), stats.NewestItem.Format("2006-01-02"))
	}

	summary += fmt.Sprintf(`  Content Types: %d
`, len(stats.ByType))
	for contentType, typeStat := range stats.ByType {
		summary += fmt.Sprintf(`    %s: %d items (%.1f avg bytes)
`, contentType, typeStat.Count, typeStat.AverageSize)
	}

	if len(stats.ByDevice) > 0 {
		summary += fmt.Sprintf(`  Devices: %d
`, len(stats.ByDevice))
		for deviceID, deviceStat := range stats.ByDevice {
			summary += fmt.Sprintf(`    %s: %d items
`, deviceID, deviceStat.Count)
		}
	}

	return summary, nil
}
