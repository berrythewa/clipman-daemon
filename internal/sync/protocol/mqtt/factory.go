// Package mqtt provides an MQTT-based implementation of the sync protocol
// This file handles MQTT protocol registration
package mqtt

import (
	"github.com/berrythewa/clipman-daemon/internal/sync"
	"github.com/berrythewa/clipman-daemon/internal/sync/protocol"
	"github.com/berrythewa/clipman-daemon/internal/types"
)

// Protocol name constant
const (
	ProtocolName = "mqtt"
)

// Factory is the MQTT protocol factory
type Factory struct{}

// NewFactory creates a new MQTT protocol factory
func NewFactory() *Factory {
	return &Factory{}
}

// init registers the MQTT protocol factory with the protocol registry
func init() {
	protocol.RegisterProtocolFactory(ProtocolName, NewFactory())
}

// CreateProtocol creates an MQTT protocol implementation
func (f *Factory) CreateProtocol(options interface{}) (sync.Protocol, error) {
	// Not implemented yet - will be needed when we implement the full Protocol interface
	return nil, nil
}

// SupportsConfig checks if this factory supports the given configuration
func (f *Factory) SupportsConfig(config interface{}) bool {
	// We support any config, but prefer MQTT-specific config
	// For now, we just return true
	return true
}

// NewClient creates a new MQTT client
func (f *Factory) NewClient(options interface{}) (protocol.Client, error) {
	// Convert options
	mqttOpts, ok := options.(*MQTTOptions)
	if !ok {
		// Check if we received a generic ProtocolOptions
		if genericOpts, ok := options.(*protocol.ProtocolOptions); ok {
			mqttOpts = DefaultMQTTOptions()
			mqttOpts.ProtocolOptions = *genericOpts
		} else {
			return nil, nil
		}
	}
	
	// Create client
	return NewMQTTClient(mqttOpts)
}

// CreateContentMessage creates a new MQTT content message
func (f *Factory) CreateContentMessage(content *types.ClipboardContent) (sync.Message, error) {
	if content == nil {
		return nil, nil
	}
	
	return NewMQTTContentMessage(content)
} 