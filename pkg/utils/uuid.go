package utils

import (
	"crypto/rand"
	"fmt"
	"time"
)

//TODO: use device info/metrics to generate a more unique UUID
//TODO: use uuid from config if set or use user specified name to generate a UUID (hashed)
func GenerateUUID() string {
	uuid := make([]byte, 16)
	_, err := rand.Read(uuid)
	if err != nil {
		return fmt.Sprintf("error-generating-uuid-%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:])
}