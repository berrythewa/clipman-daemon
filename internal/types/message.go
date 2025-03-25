package types

// Message represents a message received from a message broker
type Message struct {
	// Topic is the message topic
	Topic string
	
	// Payload is the raw message content
	Payload []byte
}

// MessageCallback is a function that processes messages
type MessageCallback func(msg Message) 