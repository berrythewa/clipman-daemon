// File: pkg/utils/uuid_test.go

package utils

import (
	"testing"
	"regexp"
	"strings"
	"strconv"
)

func TestGenerateUUID(t *testing.T) {
	// Pattern matching the format of the generated UUID
	uuidPattern := `^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`
	re := regexp.MustCompile(uuidPattern)

	// Generate multiple UUIDs to ensure uniqueness
	uuids := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		uuid := GenerateUUID()

		// Check if the UUID matches the expected pattern
		if !re.MatchString(uuid) {
			t.Errorf("Generated UUID does not match expected pattern: %s", uuid)
		}

		// Check for uniqueness
		if uuids[uuid] {
			t.Errorf("Duplicate UUID generated: %s", uuid)
		}
		uuids[uuid] = true

		// Check the length of each segment
		segments := strings.Split(uuid, "-")
		if len(segments) != 5 {
			t.Errorf("UUID does not have 5 segments: %s", uuid)
		} else {
			expectedLengths := []int{8, 4, 4, 4, 12}
			for i, segment := range segments {
				if len(segment) != expectedLengths[i] {
					t.Errorf("UUID segment %d has incorrect length. Expected %d, got %d: %s", 
						i+1, expectedLengths[i], len(segment), uuid)
				}
			}
		}
	}
}

func TestGenerateUUIDError(t *testing.T) {
	// This test is to verify the error case
	// We can't easily force crypto/rand.Read to fail, so we'll just check 
	// that the error prefix is present if an error occurs
	uuid := GenerateUUID()
	if strings.HasPrefix(uuid, "error-generating-uuid-") {
		// This is the error case
		if _, err := parseNanoTime(uuid[23:]); err != nil {
			t.Errorf("Error UUID doesn't contain valid nanosecond timestamp: %s", uuid)
		}
	} else {
		// This is the success case, so it should match our UUID pattern
		if !regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`).MatchString(uuid) {
			t.Errorf("Generated UUID does not match expected pattern: %s", uuid)
		}
	}
}

// Helper function to parse the nanosecond timestamp
func parseNanoTime(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}