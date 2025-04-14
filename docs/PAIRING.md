# Secure Device Pairing in Clipman

This document describes Clipman's secure device pairing functionality, which enables safe and trusted clipboard synchronization between multiple devices.

## Overview

Secure device pairing is the recommended approach for establishing trusted connections between your devices for clipboard synchronization. Instead of relying on general network discovery methods, pairing creates a direct, verified relationship between specific devices that you own or trust.

## Benefits of Secure Pairing

- **Explicit Trust**: Only devices that have been explicitly paired can exchange clipboard content
- **Verification Codes**: Visual verification ensures you're connecting to the intended device
- **Persistent Connections**: Paired devices maintain their trusted relationship across restarts
- **Enhanced Privacy**: Your clipboard data only flows between your verified devices
- **Prevention of Man-in-the-Middle Attacks**: The verification process helps prevent interception

## Pairing Command Usage

The `pair` command provides a complete interface for secure device pairing and management:

### Entering Pairing Mode

To make a device available for pairing:

```bash
clipman pair
```

This puts the device in pairing mode, displaying an address that can be shared with another device to establish the connection.

Options:
- `--timeout <seconds>`: Automatically exit pairing mode after the specified time (default: no timeout)
- `--auto-accept`: Automatically accept all incoming pairing requests (use with caution)

### Requesting Pairing

To connect to a device that's in pairing mode:

```bash
clipman pair --request "/ip4/192.168.1.100/tcp/45678/p2p/QmHashOfThePeer"
```

This sends a pairing request to the specified device address. If accepted, both devices will display a verification code that should match.

### Managing Paired Devices

To list all paired devices:

```bash
clipman pair --list
```

To remove a paired device:

```bash
clipman pair --remove "QmHashOfThePeer"
```

## Pairing Process Explained

1. **Initialization**: One device enters pairing mode, generating a unique connection address
2. **Connection Request**: Another device initiates a pairing request using this address
3. **Approval**: The first device receives a prompt to accept or reject the request
4. **Verification**: Upon acceptance, both devices generate and display a verification code
5. **Confirmation**: Users visually verify the codes match on both devices
6. **Persistence**: The pairing is saved for future connections
7. **Automatic Reconnection**: Paired devices can reconnect automatically in the future

## Configuration

The pairing functionality can be configured through Clipman's configuration file:

```json
{
  "sync": {
    "discovery_method": "paired",  // Use paired devices as the discovery method
    "pairing_enabled": true,       // Enable the pairing functionality
    "pairing_timeout": 300,        // Timeout in seconds (0 = no timeout)
    "device_name": "My Laptop",    // Human-readable name shown during pairing
    "device_type": "laptop"        // Type of device (desktop, laptop, mobile, etc.)
  }
}
```

For a comprehensive list of sync and pairing options, see [CONFIG.md](../internal/sync/CONFIG.md).

## Security Recommendations

1. **Verify Codes**: Always visually verify that the pairing codes match on both devices
2. **Use Secure Networks**: Perform initial pairing on a trusted network when possible
3. **Avoid Auto-Accept**: Only use the `--auto-accept` option in controlled environments
4. **Set Device Names Clearly**: Use distinctive device names to avoid confusion
5. **Set Timeouts**: Use the `--timeout` option to limit how long your device accepts pairing requests

## Troubleshooting

### Common Issues

1. **Connection Failures**:
   - Ensure both devices are on the same network
   - Check for firewalls or network restrictions
   - Verify the pairing address was copied correctly

2. **Device Not Found After Pairing**:
   - Check if both devices have network connectivity
   - Ensure the sync service is running on both devices
   - Verify the discovery method is set to "paired" in configuration

3. **Pairing Requests Not Received**:
   - Make sure pairing is enabled in configuration
   - Check if device names are configured correctly
   - Restart the daemon on both devices

### Verifying Pairing Status

To check if your devices are correctly paired and able to discover each other:

```bash
clipman pair --list
```

This will show all paired devices along with their connection status.

## Internal Architecture

The pairing functionality is implemented through several components:

1. **PairingManager**: Core component that handles the pairing protocol
2. **PairingDiscoveryService**: Service that discovers and connects to paired devices
3. **CLI Command**: User interface for initiating and managing pairings
4. **Secure Protocol**: Custom protocol for handling the pairing exchange

The pairing process uses libp2p's secure transport for end-to-end encrypted communications during the pairing exchange and subsequent clipboard synchronization.

## Future Enhancements

Planned improvements to the pairing system include:

1. **Group Pairing**: Create trusted groups of devices that can all sync with each other
2. **QR Code Pairing**: Generate and scan QR codes for easier pairing on mobile devices
3. **Certificate-Based Trust**: Enhanced security through digital certificates
4. **Permission-Based Access**: Granular control over what content types are shared with specific devices
5. **Pairing Expiration**: Optional time-limited pairings for temporary access 