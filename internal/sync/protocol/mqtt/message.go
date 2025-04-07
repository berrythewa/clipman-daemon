// Package mqtt provides an MQTT-based implementation of the sync protocol
// This file defines MQTT-specific message formats
package mqtt

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/sync"
	"github.com/berrythewa/clipman-daemon/internal/sync/protocol"
	"github.com/berrythewa/clipman-daemon/internal/types"
	"github.com/google/uuid"
)

// MQTT-specific constants
const (
	// Topic structure
	TopicPrefix       = "clipman"
	TopicSep          = "/"
	TopicContentSuffix = "content"
	TopicControlSuffix = "control"
	
	// QoS levels
	QoSAtMostOnce  = 0 // Fire and forget
	QoSAtLeastOnce = 1 // Guaranteed delivery
	QoSExactlyOnce = 2 // Guaranteed exactly-once delivery
)

// MQTTMessage implements the sync.Message interface for MQTT protocol
type MQTTMessage struct {
	protocol.CommonMessageFields
	Topic   string      `json:"topic"`   // MQTT topic
	QoS     byte        `json:"qos"`     // MQTT QoS level
	Retain  bool        `json:"retain"`  // MQTT retain flag
	Payload interface{} `json:"payload"` // Actual message payload
}

// NewMQTTMessage creates a new MQTT message
func NewMQTTMessage(msgType string, payload interface{}) *MQTTMessage {
	return &MQTTMessage{
		CommonMessageFields: protocol.CommonMessageFields{
			Type:      msgType,
			ID:        uuid.New().String(),
			Timestamp: time.Now().UTC(),
		},
		QoS:     QoSAtLeastOnce, // Default to at-least-once delivery
		Payload: payload,
	}
}

// Type returns the message type
func (m *MQTTMessage) Type() string {
	return m.CommonMessageFields.Type
}

// Payload returns the message payload as bytes
func (m *MQTTMessage) Payload() []byte {
	data, err := json.Marshal(m.Payload)
	if err != nil {
		return []byte{}
	}
	return data
}

// Source returns the message source
func (m *MQTTMessage) Source() string {
	return m.CommonMessageFields.Source
}

// Destination returns the message destination
func (m *MQTTMessage) Destination() string {
	return m.CommonMessageFields.Destination
}

// Group returns the message group
func (m *MQTTMessage) Group() string {
	return m.CommonMessageFields.Group
}

// Timestamp returns the message timestamp
func (m *MQTTMessage) Timestamp() time.Time {
	return m.CommonMessageFields.Timestamp
}

// ID returns the message ID
func (m *MQTTMessage) ID() string {
	return m.CommonMessageFields.ID
}

// SetDestination sets the message destination
func (m *MQTTMessage) SetDestination(dest string) {
	m.CommonMessageFields.Destination = dest
}

// SetGroup sets the message group and updates the topic
func (m *MQTTMessage) SetGroup(group string) {
	m.CommonMessageFields.Group = group
	m.updateTopic()
}

// RequestAck returns whether the message requires acknowledgment
func (m *MQTTMessage) RequestAck() bool {
	return m.CommonMessageFields.NeedsAck
}

// SetSource sets the message source
func (m *MQTTMessage) SetSource(source string) {
	m.CommonMessageFields.Source = source
}

// SetQoS sets the MQTT QoS level
func (m *MQTTMessage) SetQoS(qos byte) {
	m.QoS = qos
}

// SetRetain sets the MQTT retain flag
func (m *MQTTMessage) SetRetain(retain bool) {
	m.Retain = retain
}

// GetTopic returns the MQTT topic for this message
func (m *MQTTMessage) GetTopic() string {
	if m.Topic == "" {
		m.updateTopic()
	}
	return m.Topic
}

// updateTopic generates the appropriate MQTT topic based on message type and group
func (m *MQTTMessage) updateTopic() {
	topicParts := []string{TopicPrefix}
	
	// Add group to topic if specified
	if m.Group != "" {
		topicParts = append(topicParts, m.Group)
	} else {
		topicParts = append(topicParts, "default")
	}
	
	// Add appropriate suffix based on message type
	switch m.Type {
	case string(protocol.MessageTypeContent), string(protocol.MessageTypeFile):
		topicParts = append(topicParts, TopicContentSuffix)
	default:
		topicParts = append(topicParts, TopicControlSuffix)
	}
	
	// Add specific message type for control messages
	if m.Type != string(protocol.MessageTypeContent) && m.Type != string(protocol.MessageTypeFile) {
		topicParts = append(topicParts, m.Type)
	}
	
	// Add destination if specified
	if m.Destination != "" {
		topicParts = append(topicParts, m.Destination)
	}
	
	m.Topic = strings.Join(topicParts, TopicSep)
}

// ToJSON converts the message to a JSON byte array
func (m *MQTTMessage) ToJSON() ([]byte, error) {
	return json.Marshal(m)
}

// FromJSON parses a JSON byte array into the message
func (m *MQTTMessage) FromJSON(data []byte) error {
	return json.Unmarshal(data, m)
}

// MQTTContentMessage is an MQTT-specific implementation for clipboard content
type MQTTContentMessage struct {
	MQTTMessage
	Content *types.ClipboardContent `json:"content"`
}

// NewMQTTContentMessage creates a new MQTT content message
func NewMQTTContentMessage(content *types.ClipboardContent) (*MQTTContentMessage, error) {
	msg := &MQTTContentMessage{
		MQTTMessage: *NewMQTTMessage(string(protocol.MessageTypeContent), nil),
		Content:     content,
	}
	
	// Set the payload to the content
	msg.Payload = content
	
	// Update the topic
	msg.updateTopic()
	
	return msg, nil
}

// GetContent returns the clipboard content
func (m *MQTTContentMessage) GetContent() *types.ClipboardContent {
	return m.Content
}

// ParseMQTTMessage parses an MQTT message from a topic and payload
func ParseMQTTMessage(topic string, payload []byte) (sync.Message, error) {
	// First determine the message type from the topic
	topicParts := strings.Split(topic, TopicSep)
	if len(topicParts) < 3 {
		return nil, fmt.Errorf("invalid MQTT topic: %s", topic)
	}
	
	// Extract group from topic
	group := topicParts[1]
	
	// Extract message type from topic
	var msgType string
	if topicParts[2] == TopicContentSuffix {
		msgType = string(protocol.MessageTypeContent)
	} else if len(topicParts) >= 4 {
		msgType = topicParts[3]
	} else {
		return nil, fmt.Errorf("unable to determine message type from topic: %s", topic)
	}
	
	// Parse the payload based on the message type
	switch msgType {
	case string(protocol.MessageTypeContent):
		var content types.ClipboardContent
		if err := json.Unmarshal(payload, &content); err != nil {
			return nil, fmt.Errorf("failed to parse content message: %w", err)
		}
		
		msg := &MQTTContentMessage{
			MQTTMessage: MQTTMessage{
				CommonMessageFields: protocol.CommonMessageFields{
					Type:      msgType,
					Group:     group,
					Timestamp: time.Now().UTC(), // Use current time as we don't know the original time
				},
				Topic:   topic,
				Payload: content,
			},
			Content: &content,
		}
		
		return msg, nil
		
	default:
		// For other types, just create a basic MQTT message
		var data map[string]interface{}
		if err := json.Unmarshal(payload, &data); err != nil {
			return nil, fmt.Errorf("failed to parse message payload: %w", err)
		}
		
		msg := &MQTTMessage{
			CommonMessageFields: protocol.CommonMessageFields{
				Type:      msgType,
				Group:     group,
				Timestamp: time.Now().UTC(),
			},
			Topic:   topic,
			Payload: data,
		}
		
		return msg, nil
	}
} 