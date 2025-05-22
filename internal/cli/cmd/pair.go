package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	// "strconv"
	"strings"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/types"
	"github.com/berrythewa/clipman-daemon/internal/sync"
	"github.com/berrythewa/clipman-daemon/internal/ipc"
	"github.com/spf13/cobra"
	// "go.uber.org/zap" TODO: no logging or Cobra does it ?/
)

var (
	// Pair command flags
	pairAddress string
	listPaired   bool
	removePair   string
)

// TODO   clipman pair --show-qr
// TODO   clipman pair --scan-qr "<payload string>"

// TODO   clipman pair --show-qr --json
// TODO   clipman pair --show-qr --json --device-id DEVICE_ID
// TODO   clipman pair --show-qr --json --device-id DEVICE_ID --key PAIR_KEY
// TODO   clipman pair --show-qr --json --device-id DEVICE_ID --key PAIR_KEY --room ROOM_NAME

// pairCmd represents the pair command for device pairing
var pairCmd = &cobra.Command{
	Use:   "pair",
	Short: "Pair this device with another trusted device via the daemon",
	Long: `Pair this device with another trusted device for clipboard sync.

Examples:
  clipman pair                # Enable pairing mode, print address, wait for requests
  clipman pair --address <address>  # Initiate pairing with given address
  clipman pair --list         # List paired devices
  clipman pair --remove <peerID> # Remove a paired device
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if listPaired {
			return listPairedDevices()
		}
		if removePair != "" {
			return removePairedDevice(removePair)
		}
		if pairAddress != "" {
			return requestPairing(pairAddress)
		}
		// Default: enable pairing mode and print address, prompt for each request
		return enablePairingModeInteractive()
	},
}

// requestPairing initiates a pairing request to the specified address
func requestPairing(address string) error {
	// Get sync manager
	syncManager, err := getSyncManager()
	if err != nil {
		return err
	}
	
	// Check if pairing is enabled in the config and try to enable it if not
	if err := ensurePairingEnabled(syncManager); err != nil {
		return err
	}

	// Start the sync manager if needed
	if err := syncManager.Start(); err != nil {
		return fmt.Errorf("failed to start sync manager: %w", err)
	}
	defer syncManager.Stop()

	fmt.Println("Sending pairing request to:", address)
	fmt.Println("Waiting for response... (Press Ctrl+C to cancel)")

	// Send pairing request
	response, err := syncManager.RequestPairing(address)
	if err != nil {
		return fmt.Errorf("pairing request failed: %w", err)
	}

	// Handle the response
	if response.Accepted {
		fmt.Println("✅ Pairing successful!")
		fmt.Println("Device Name:", response.DeviceName)
		fmt.Println("Device Type:", response.DeviceType)
		fmt.Println("Peer ID:", response.PeerID)
		fmt.Println()
		fmt.Println("📱 VERIFICATION CODE:", response.PairingCode)
		fmt.Println("Make sure this code matches on both devices!")
		
		// Verify if the paired device is now listed
		pairedDevices := syncManager.GetPairedDevices()
		found := false
		for _, device := range pairedDevices {
			if device.PeerID == response.PeerID {
				found = true
				break
			}
		}
		
		if !found {
			fmt.Println("\n⚠️ Warning: The paired device wasn't found in your list of paired devices.")
			fmt.Println("This could indicate a configuration issue.")
		}
	} else {
		fmt.Println("❌ Pairing request was rejected")
		if response.ErrorMessage != "" {
			fmt.Println("Error:", response.ErrorMessage)
		}
	}

	return nil
}

// enablePairingModeInteractive enables pairing mode and prompts for each incoming request
func enablePairingModeInteractive() error {
	syncManager, err := getSyncManager()
	if err != nil {
		return err
	}
	if err := ensurePairingEnabled(syncManager); err != nil {
		return err
	}
	if err := syncManager.Start(); err != nil {
		return fmt.Errorf("failed to start sync manager: %w", err)
	}
	defer syncManager.Stop()

	pairingCallback := func(request types.PairingRequest, remotePeerID string) (bool, error) {
		fmt.Println("\n🔄 Incoming pairing request:")
		fmt.Println("Device Name:", request.DeviceName)
		fmt.Println("Device Type:", request.DeviceType)
		fmt.Println("Peer ID:", request.PeerID)
		fmt.Println()
		fmt.Print("Accept pairing request? (y/n): ")
		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return false, fmt.Errorf("failed to read input: %w", err)
		}
		response = strings.TrimSpace(strings.ToLower(response))
		return response == "y" || response == "yes", nil
	}

	address, err := syncManager.EnablePairing(pairingCallback)
	if err != nil {
		return fmt.Errorf("failed to enable pairing: %w", err)
	}
	fmt.Println("\n📱 Share this address with the device you want to pair with:")
	fmt.Println(address)
	fmt.Println("\nWaiting for pairing requests... (Press Ctrl+C to cancel)")
	select {} // Block forever; user can Ctrl+C to exit
}

// ensurePairingEnabled checks if pairing is enabled in the config
// and attempts to enable it if necessary
func ensurePairingEnabled(syncManager types.SyncManager) error {
	// If pairing is already enabled, we're good
	if syncManager.IsPairingEnabled() {
		return nil
	}

	// Check if we can enable pairing
	config := syncManager.GetConfig()
	if config.DeviceName == "" {
		return fmt.Errorf("unable to access sync configuration")
	}

	if !config.PairingEnabled {
		fmt.Println("⚠️ Pairing is not enabled in your configuration")
		fmt.Println("To permanently enable pairing, set pairing_enabled=true in your config")
		fmt.Print("Would you like to temporarily enable pairing for this session? (y/n): ")
		
		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}

		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			return fmt.Errorf("pairing not enabled - operation cancelled")
		}
		
		fmt.Println("Temporarily enabling pairing for this session")
	}
	
	return nil
}

// listPairedDevices lists all paired devices
func listPairedDevices() error {
	// Get sync manager
	syncManager, err := getSyncManager()
	if err != nil {
		return err
	}

	// Start the sync manager if needed
	if err := syncManager.Start(); err != nil {
		return fmt.Errorf("failed to start sync manager: %w", err)
	}
	defer syncManager.Stop()

	// Get paired devices
	devices := syncManager.GetPairedDevices()

	if len(devices) == 0 {
		fmt.Println("No paired devices found")
		fmt.Println("\nTo pair with a device:")
		fmt.Println("1. Run 'clipman pair' on this device to enter pairing mode")
		fmt.Println("2. Share the displayed address with your other device")
		fmt.Println("3. On the other device, run 'clipman pair --request <address>'")
		fmt.Println("4. Accept the pairing request and verify the codes match")
		return nil
	}

	fmt.Printf("Found %d paired devices:\n\n", len(devices))
	
	for i, device := range devices {
		fmt.Printf("%d. Device: %s (%s)\n", i+1, device.DeviceName, device.DeviceType)
		fmt.Printf("   Peer ID: %s\n", device.PeerID)
		fmt.Printf("   Paired: %s\n", formatRelativeTime(device.PairedAt))
		fmt.Printf("   Last seen: %s\n", formatRelativeTime(device.LastSeen))
		fmt.Println()
	}
	
	fmt.Println("To remove a device, use: clipman pair --remove <peer_id>")
	
	// Check discovery method
	config := syncManager.GetConfig()
	if config != nil && config.DiscoveryMethod != "paired" {
		fmt.Println("\n⚠️ Note: Your current discovery method is not set to 'paired',")
		fmt.Println("which is the recommended setting for security. Consider updating")
		fmt.Println("your configuration to use paired discovery exclusively.")
	}

	return nil
}

// removePairedDevice removes a paired device by peer ID
func removePairedDevice(peerID string) error {
	// Get sync manager
	syncManager, err := getSyncManager()
	if err != nil {
		return err
	}

	// Start the sync manager if needed
	if err := syncManager.Start(); err != nil {
		return fmt.Errorf("failed to start sync manager: %w", err)
	}
	defer syncManager.Stop()

	// Check if device is paired
	if !syncManager.IsPaired(peerID) {
		// If the device isn't paired, provide list of available paired devices
		devices := syncManager.GetPairedDevices()
		if len(devices) == 0 {
			return fmt.Errorf("no paired devices found - you don't have any devices to remove")
		}
		
		fmt.Println("⚠️ The specified device ID is not paired")
		fmt.Println("\nHere are your paired devices:")
		for i, device := range devices {
			fmt.Printf("%d. %s (%s)\n", i+1, device.DeviceName, device.PeerID)
		}
		
		return fmt.Errorf("device with ID %s is not paired", peerID)
	}

	// Get device info for better feedback
	var deviceName string
	devices := syncManager.GetPairedDevices()
	for _, device := range devices {
		if device.PeerID == peerID {
			deviceName = device.DeviceName
			break
		}
	}

	// Confirm removal
	fmt.Printf("Are you sure you want to remove the paired device '%s' (%s)? (y/n): ", deviceName, peerID)
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}

	response = strings.TrimSpace(strings.ToLower(response))
	if response != "y" && response != "yes" {
		fmt.Println("Operation cancelled")
		return nil
	}

	// Remove the device
	if err := syncManager.RemovePairedDevice(peerID); err != nil {
		return fmt.Errorf("failed to remove device: %w", err)
	}

	fmt.Printf("Successfully removed paired device '%s' (%s)\n", deviceName, peerID)
	
	// Verify the device was actually removed
	if syncManager.IsPaired(peerID) {
		fmt.Println("\n⚠️ Warning: The device still appears to be paired despite removal attempt")
		fmt.Println("This could indicate a persistence issue in the sync system")
	} else {
		fmt.Println("Device removal confirmed")
	}
	
	return nil
}

// getSyncManager gets the sync manager from the current service
func getSyncManager() (types.SyncManager, error) {
	// Create a new sync manager instance directly
	syncManager, err := sync.New(context.Background(), cfg, zapLogger)
	if err != nil {
		return nil, fmt.Errorf("failed to create sync manager: %w", err)
	}

	// Ensure that we're configured to use pairing-based discovery
	// Note: The pair command should only use the secure pairing protocol
	// for device discovery, not other methods like mDNS or DHT
	config := syncManager.GetConfig()
	if config != nil && config.DiscoveryMethod != "paired" {
		fmt.Println("⚠️ Warning: Current discovery method is not set to 'paired'")
		fmt.Println("The most secure approach is to use paired discovery method.")
		fmt.Println("You can change this in your configuration file.")
	}

	return syncManager, nil
}

// formatRelativeTime formats a time.Time as a human-readable relative time
func formatRelativeTime(t time.Time) string {
	now := time.Now()
	duration := now.Sub(t)

	if duration < time.Minute {
		return "just now"
	} else if duration < time.Hour {
		minutes := int(duration.Minutes())
		return fmt.Sprintf("%d minute%s ago", minutes, plural(minutes))
	} else if duration < 24*time.Hour {
		hours := int(duration.Hours())
		return fmt.Sprintf("%d hour%s ago", hours, plural(hours))
	} else if duration < 30*24*time.Hour {
		days := int(duration.Hours() / 24)
		return fmt.Sprintf("%d day%s ago", days, plural(days))
	} else if duration < 365*24*time.Hour {
		months := int(duration.Hours() / 24 / 30)
		return fmt.Sprintf("%d month%s ago", months, plural(months))
	}
	
	years := int(duration.Hours() / 24 / 365)
	return fmt.Sprintf("%d year%s ago", years, plural(years))
}

// plural returns "s" if count is not 1
func plural(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}

func init() {
	pairCmd.Flags().StringVar(&pairAddress, "address", "", "Address of the device to pair with (manual entry)")
	pairCmd.Flags().BoolVar(&listPaired, "list", false, "List all securely paired devices")
	pairCmd.Flags().StringVar(&removePair, "remove", "", "Remove a securely paired device by Peer ID")
} 