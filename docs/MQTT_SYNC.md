# Clipman MQTT Synchronization

This document provides a detailed explanation of how Clipman uses MQTT to synchronize clipboard content between different instances running on multiple devices.

## Overview

Clipman supports real-time synchronization of clipboard content across multiple devices using MQTT (Message Queuing Telemetry Transport), a lightweight publish/subscribe messaging protocol designed for constrained devices and low-bandwidth, high-latency networks.

## Architecture

The synchronization system consists of:

1. **MQTT Client**: Manages connections to the MQTT broker and handles message publishing/subscribing
2. **Command Handlers**: Process incoming commands from other instances
3. **Content Publishing**: Sends clipboard content to other instances
4. **Cache Synchronization**: Shares clipboard history between instances

## MQTT Client Implementation

The `MQTTClient` struct implements the `MQTTClientInterface` interface:

```go
type MQTTClientInterface interface {
    PublishContent(content *types.ClipboardContent) error
    PublishCache(cache *types.CacheMessage) error
    SubscribeToCommands() error
    RegisterCommandHandler(commandType string, handler func([]byte) error)
    IsConnected() bool
    Disconnect() error
}
```

### Initialization

The MQTT client is initialized with the configuration from Clipman's config file:

```go
func NewMQTTClient(cfg *config.Config, logger *utils.Logger) (*MQTTClient, error) {
    client := &MQTTClient{
        config:          cfg,
        logger:          logger,
        deviceID:        cfg.DeviceID,
        commandHandlers: make(map[string]func([]byte) error),
    }

    opts := mqtt.NewClientOptions().
        AddBroker(cfg.Broker.URL).
        SetClientID(cfg.DeviceID).
        SetUsername(cfg.Broker.Username).
        SetPassword(cfg.Broker.Password).
        SetAutoReconnect(true).
        SetOnConnectHandler(client.onConnect).
        SetConnectionLostHandler(client.onConnectionLost).
        SetReconnectingHandler(client.onReconnecting)

    client.client = mqtt.NewClient(opts)

    if err := client.connect(); err != nil {
        return nil, err
    }

    return client, nil
}
```

## Message Types

### Clipboard Content

The `ClipboardContent` struct represents a clipboard item:

```go
type ClipboardContent struct {
    Type        ContentType  // text, image, file, url, filepath
    Data        []byte       // The actual clipboard data
    Created     time.Time    // When the item was copied
    Compressed  bool         // Whether the data is compressed
}
```

### Cache Message

The `CacheMessage` struct represents a batch of clipboard history items:

```go
type CacheMessage struct {
    DeviceID   string                // Identifier of the sending device
    Contents   []*ClipboardContent   // Collection of clipboard items
    Timestamp  time.Time             // When the message was created
}
```

## Communication Protocol

### Topics

Clipman uses the following MQTT topics for communication:

1. **Content Publishing**: `clipman/{deviceID}/content`
   - Used to publish new clipboard content
   
2. **Cache Publishing**: `clipman/cache/{deviceID}`
   - Used to publish clipboard history

3. **Commands**: `clipman/{deviceID}/commands`
   - Used to send commands to specific devices

### Publishing Content

When a new clipboard item is detected, it's published to other instances:

```go
func (m *MQTTClient) PublishContent(content *types.ClipboardContent) error {
    if m == nil || m.client == nil {
        return nil // Skip publishing if no MQTT client
    }

    if !m.isConnected {
        return fmt.Errorf("not connected to MQTT broker")
    }

    topic := fmt.Sprintf("clipman/%s/content", m.deviceID)
    payload, err := json.Marshal(content)
    if err != nil {
        return fmt.Errorf("failed to marshal clipboard content: %v", err)
    }

    token := m.client.Publish(topic, 1, false, payload)
    if token.Wait() && token.Error() != nil {
        return fmt.Errorf("failed to publish content: %v", token.Error())
    }

    return nil
}
```

### Publishing Cache

When synchronizing clipboard history, cache messages are published:

```go
func (c *MQTTClient) PublishCache(cache *types.CacheMessage) error {
    topic := fmt.Sprintf("clipman/cache/%s", cache.DeviceID)

    payload, err := json.Marshal(cache)
    if err != nil {
        return fmt.Errorf("failed to marshal cache: %v", err)
    }

    if token := c.client.Publish(topic, 1, false, payload); token.Wait() && token.Error() != nil {
        return fmt.Errorf("failed to publish cache: %v", token.Error())
    }

    return nil
}
```

### Command Subscription

Clipman instances subscribe to commands from other instances:

```go
func (m *MQTTClient) SubscribeToCommands() error {
    if !m.isConnected {
        return fmt.Errorf("not connected to MQTT broker")
    }

    topic := fmt.Sprintf("clipman/%s/commands", m.deviceID)
    token := m.client.Subscribe(topic, 1, m.handleCommand)
    if token.Wait() && token.Error() != nil {
        return fmt.Errorf("failed to subscribe to commands: %v", token.Error())
    }

    return nil
}
```

## Synchronization Flow

### New Clipboard Content

1. User copies content to clipboard on Device A
2. Clipman on Device A detects the change
3. Content is processed and saved locally
4. Content is published to the MQTT broker
5. Other instances (Device B, C, etc.) receive the content
6. Other instances add the content to their local storage

### Cache Synchronization

1. Device joins the network or reconnects
2. It publishes its current cache to the broker
3. Other devices receive the cache and merge with their local storage
4. Conflicts are resolved based on timestamps

## Command Handling

Clipman instances can send commands to each other for various operations:

```go
func (m *MQTTClient) handleCommand(client mqtt.Client, msg mqtt.Message) {
    var command struct {
        Type string          `json:"type"`
        Data json.RawMessage `json:"data"`
    }

    if err := json.Unmarshal(msg.Payload(), &command); err != nil {
        m.logger.Error("Failed to unmarshal command", "error", err)
        return
    }

    handler, ok := m.commandHandlers[command.Type]
    if !ok {
        m.logger.Warn("Unknown command type", "type", command.Type)
        return
    }

    if err := handler(command.Data); err != nil {
        m.logger.Error("Failed to handle command", "type", command.Type, "error", err)
    }
}
```

Commands can be registered by other components:

```go
func (m *MQTTClient) RegisterCommandHandler(commandType string, handler func([]byte) error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.commandHandlers[commandType] = handler
}
```

## Configuration Options

### MQTT Broker Settings

| Option | Config File Key | Environment Variable | Description |
|--------|----------------|---------------------|-------------|
| URL | `broker.url` | `CLIPMAN_BROKER_URL` | URL of the MQTT broker (e.g., `mqtt://broker.example.com:1883`) |
| Username | `broker.username` | `CLIPMAN_BROKER_USERNAME` | Username for broker authentication |
| Password | `broker.password` | `CLIPMAN_BROKER_PASSWORD` | Password for broker authentication |
| Disable | - | - | Use `--no-broker` flag to disable MQTT connection |

### Example Configuration

```json
{
  "broker": {
    "url": "mqtt://broker.example.com:1883",
    "username": "your_username",
    "password": "your_password"
  }
}
```

## Security Considerations

### Transport Security

For secure clipboard synchronization:

1. **Use TLS**: Configure the broker URL with `mqtts://` instead of `mqtt://`
2. **Strong Authentication**: Set strong, unique username and password
3. **Private Broker**: Use a private MQTT broker rather than public ones
4. **Access Control**: Configure ACLs on your MQTT broker to limit access
5. **Network Security**: Consider restricting broker access to VPN connections

### Data Security

1. **Clipboard data is sent as-is**: Currently no encryption layer for the data itself
2. **Sensitive Content**: Be careful when syncing sensitive clipboard content
3. **Compression**: Content compression is implemented but currently disabled

## Setting Up Your Own MQTT Broker

For optimal security and performance, consider setting up your own MQTT broker:

### Option 1: Mosquitto

1. Install Mosquitto:
   ```bash
   # Ubuntu/Debian
   sudo apt-get install mosquitto mosquitto-clients
   
   # macOS
   brew install mosquitto
   
   # Windows
   # Download from https://mosquitto.org/download/
   ```

2. Configure Mosquitto with authentication:
   ```
   # /etc/mosquitto/mosquitto.conf
   listener 1883
   allow_anonymous false
   password_file /etc/mosquitto/pwfile
   ```

3. Create a password file:
   ```bash
   sudo mosquitto_passwd -c /etc/mosquitto/pwfile your_username
   ```

4. Configure Clipman to use your broker:
   ```json
   {
     "broker": {
       "url": "mqtt://your_mosquitto_server:1883",
       "username": "your_username",
       "password": "your_password"
     }
   }
   ```

### Option 2: Cloud MQTT Services

Several cloud providers offer managed MQTT services:

- HiveMQ Cloud (has free tier)
- AWS IoT Core
- CloudMQTT
- Azure IoT Hub

## Troubleshooting

### Common Issues

1. **Connection Errors**:
   - Check broker URL, username, and password
   - Verify network connectivity to the broker
   - Check firewall settings

2. **Missing Synchronization**:
   - Ensure devices have the same broker configuration
   - Check that DeviceID is unique for each device
   - Verify MQTT client is connected

3. **High Resource Usage**:
   - Large clipboard content can consume bandwidth
   - Consider enabling compression feature

### Debugging

Enable debug logging to see detailed MQTT communication:

```bash
clipman --log-level debug
```

## Future Enhancements

1. **End-to-end Encryption**: Add encryption for clipboard data
2. **Selective Synchronization**: Control which content types are synchronized
3. **Smart Conflict Resolution**: Improve handling of conflicting clipboard items
4. **Bandwidth Optimization**: Implement delta updates for large content
5. **Multi-broker Support**: Allow configuration of multiple brokers for redundancy 