package storage

import (
    "encoding/json"
    "time"
    
    "github.com/berrythewa/clipman-daemon/internal/types"
    "go.etcd.io/bbolt"
)
// DeleteByHashes removes items from the database by their hashes
func (s *BoltStorage) DeleteByHashes(hashes []string) (int, error) {
    var deletedCount int
    return deletedCount, s.db.Update(func(tx *bbolt.Tx) error {
        b := tx.Bucket([]byte(clipboardBucket))
        for _, hash := range hashes {
            if err := b.Delete([]byte(hash)); err != nil {
                return err
            }
            deletedCount++
        }
        return nil
    })
}

// DeleteByIDs - Useful if IDs are exposed in the UI
func (s *BoltStorage) DeleteByIDs(ids []int64) (int, error) {
    var deletedCount int
    err := s.db.Update(func(tx *bbolt.Tx) error {
        b := tx.Bucket([]byte(clipboardBucket))
        return b.ForEach(func(k, v []byte) error {
            var content types.ClipboardContent
            if err := json.Unmarshal(v, &content); err != nil {
                return err // Optionally log or skip invalid entries
            }
            for _, id := range ids {
                if content.Id == id {
                    if err := b.Delete(k); err != nil {
                        return err
                    }
                    deletedCount++
                    break // ID is unique, so break once found
                }
            }
            return nil
        })
    })
    return deletedCount, err
}

// DeleteAll - Useful for clearing history/reset functionality
func (s *BoltStorage) DeleteAll() (int, error) {
    var deletedCount int
    return deletedCount, s.db.Update(func(tx *bbolt.Tx) error {
        b := tx.Bucket([]byte(clipboardBucket))
        return b.ForEach(func(k, v []byte) error {
            if err := b.Delete(k); err != nil {
                return err
            }
            deletedCount++
            return nil
        })
    })
}

// DeleteSince deletes entries whose latest occurrence is since the specified time
func (s *BoltStorage) DeleteSince(since time.Time) (int, error) {
    var deletedCount int
    err := s.db.Update(func(tx *bbolt.Tx) error {
        b := tx.Bucket([]byte(clipboardBucket))
        
        var keysToDelete [][]byte
        
        // First pass: collect keys to delete
        err := b.ForEach(func(k, v []byte) error {
            var content types.ClipboardContent
            if err := json.Unmarshal(v, &content); err != nil {
                return nil // skip invalid entries
            }
            
            // Check latest occurrence (index 0, newest first)
            if len(content.Occurrences) > 0 {
                latestOccurrence := content.Occurrences[0]
                if latestOccurrence.After(since) || latestOccurrence.Equal(since) {
                    keysToDelete = append(keysToDelete, append([]byte(nil), k...))
                }
            }
            return nil
        })
        if err != nil {
            return err
        }
        
        // Second pass: delete collected keys
        for _, key := range keysToDelete {
            if err := b.Delete(key); err != nil {
                return err
            }
            deletedCount++
        }
        
        return nil
    })
    return deletedCount, err
}

// DeleteByDate deletes entries whose latest occurrence is on the specified date
func (s *BoltStorage) DeleteByDate(date time.Time) (int, error) {
    var deletedCount int
    err := s.db.Update(func(tx *bbolt.Tx) error {
        b := tx.Bucket([]byte(clipboardBucket))
        
        var keysToDelete [][]byte
        
        // Get start and end of the specified date
        startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
        endOfDay := startOfDay.Add(24 * time.Hour)
        
        // First pass: collect keys to delete
        err := b.ForEach(func(k, v []byte) error {
            var content types.ClipboardContent
            if err := json.Unmarshal(v, &content); err != nil {
                return nil // skip invalid entries
            }
            
            // Check latest occurrence (index 0, newest first)
            if len(content.Occurrences) > 0 {
                latestOccurrence := content.Occurrences[0]
                if latestOccurrence.After(startOfDay) && latestOccurrence.Before(endOfDay) {
                    keysToDelete = append(keysToDelete, append([]byte(nil), k...))
                }
            }
            return nil
        })
        if err != nil {
            return err
        }
        
        // Second pass: delete collected keys
        for _, key := range keysToDelete {
            if err := b.Delete(key); err != nil {
                return err
            }
            deletedCount++
        }
        
        return nil
    })
    return deletedCount, err
