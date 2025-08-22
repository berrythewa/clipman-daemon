package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/config"
	"github.com/berrythewa/clipman-daemon/internal/p2p"
	"github.com/berrythewa/clipman-daemon/internal/types"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// newPairCmd creates the pair command
func newPairCmd() *cobra.Command {
	var (
		address   string
		listPairs bool
		removePair string
	)

	cmd := &cobra.Command{
		Use:   "pair",
		Short: "Securely pair with another device for clipboard sync",
		Long: `Securely pair with another trusted device for clipboard synchronization.

Pairing establishes an authenticated, secure connection between two devices to
enable secure clipboard sharing.

Examples:
  clipman pair                    # Enable pairing mode, show shareable address
  clipman pair --address <addr>   # Request pairing with a device at the given address
  clipman pair --list             # List all paired devices
  clipman pair --remove <peerID>  # Remove a previously paired device`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get logger and config from globals
			logger := GetZapLogger()
			cfg := GetConfig()
			
			if cfg == nil {
				return fmt.Errorf("configuration not available")
			}
			
			// Validate sync configuration
			if err := validateSyncConfig(cfg); err != nil {
				return err
			}

			// Handle specific modes
			if listPairs {
				return listPairedDevices(logger)
			}

			if removePair != "" {
				return removePairedDevice(removePair, logger)
			}

			if address != "" {
				return requestPairing(address, logger)
			}

			// Default behavior: enable pairing mode
			return enablePairingMode(logger)
		},
	}

	// Register flags
	cmd.Flags().StringVar(&address, "address", "", "Address of the device to pair with")
	cmd.Flags().BoolVar(&listPairs, "list", false, "List all paired devices")
	cmd.Flags().StringVar(&removePair, "remove", "", "Remove a paired device by its Peer ID")

	return cmd
}

// requestPairing sends a pairing request to a device at the given address
func requestPairing(address string, logger *zap.Logger) error {
	// Validate address format (basic check)
	if !strings.Contains(address, "/p2p/") {
		return fmt.Errorf("invalid address format - should contain '/p2p/' followed by peer ID")
	}

	fmt.Println("🔄 Initiating pairing request...")

	// Get and start sync manager
	syncManager, err := getSyncManager(logger)
	if err != nil {
		return fmt.Errorf("failed to initialize sync: %w", err)
	}

	// Start the sync manager
	if err := syncManager.Start(); err != nil {
		return fmt.Errorf("failed to start sync: %w", err)
	}
	defer syncManager.Stop()

	// Check if pairing is enabled
	if err := ensurePairingEnabled(syncManager); err != nil {
		return err
	}

	fmt.Println("📡 Connecting to:", address)
	fmt.Println("⏳ Waiting for response... (Press Ctrl+C to cancel)")

	// Send the pairing request
	response, err := syncManager.RequestPairing(address)
	if err != nil {
		return fmt.Errorf("pairing request failed: %w", err)
	}

	// Handle the response
	if response.Accepted {
		fmt.Println("\n✅ Pairing successful!")
		fmt.Println("  Device Name:", response.DeviceName)
		fmt.Println("  Device Type:", response.DeviceType)
		fmt.Println("  Peer ID:", response.PeerID)
		fmt.Println()
		fmt.Println("📋 VERIFICATION CODE:", response.PairingCode)
		fmt.Println("   Make sure this code matches on both devices!")

		// Verify the device is in the paired devices list
		pairedDevices := syncManager.GetPairedDevices()
		found := false
		for _, device := range pairedDevices {
			if device.PeerID == response.PeerID {
				found = true
				break
			}
		}

		if !found {
			fmt.Println("\n⚠️  Warning: The paired device wasn't found in your list of paired devices.")
			fmt.Println("   This could indicate a persistence issue with paired device storage.")
		}
	} else {
		fmt.Println("\n❌ Pairing request was rejected")
		if response.ErrorMessage != "" {
			fmt.Println("   Error:", response.ErrorMessage)
		}
		return fmt.Errorf("pairing request was rejected")
	}

	return nil
}

// enablePairingMode enables pairing mode and waits for incoming requests
func enablePairingMode(logger *zap.Logger) error {
	fmt.Println("🔐 Enabling pairing mode...")

	// Get and start sync manager
	syncManager, err := getSyncManager(logger)
	if err != nil {
		return fmt.Errorf("failed to initialize sync: %w", err)
	}

	if err := syncManager.Start(); err != nil {
		return fmt.Errorf("failed to start sync: %w", err)
	}
	defer syncManager.Stop()

	// Check if pairing is enabled in the config
	if err := ensurePairingEnabled(syncManager); err != nil {
		return err
	}

	// Define the pairing callback
	pairingCallback := func(request types.PairingRequest, remotePeerID string) (bool, error) {
		fmt.Println("\n🔄 Incoming pairing request:")
		fmt.Println("  Device Name:", request.DeviceName)
		fmt.Println("  Device Type:", request.DeviceType)
		fmt.Println("  Peer ID:", request.PeerID)
		fmt.Println()

		// Prompt for confirmation
		fmt.Print("Accept pairing request? (y/n): ")
		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return false, fmt.Errorf("failed to read input: %w", err)
		}

		response = strings.TrimSpace(strings.ToLower(response))
		return response == "y" || response == "yes", nil
	}

	// Enable pairing and get shareable address
	address, err := syncManager.EnablePairing(pairingCallback)
	if err != nil {
		return fmt.Errorf("failed to enable pairing: %w", err)
	}

	// Show the address to share
	fmt.Println("\n📱 Share this address with the device you want to pair with:")
	fmt.Println("   " + address)
	fmt.Println("\n⏳ Waiting for pairing requests... (Press Ctrl+C to cancel)")

	// Block until user presses Ctrl+C
	select {}
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
func listPairedDevices(logger *zap.Logger) error {
	fmt.Println("📋 Retrieving paired devices...")

	// Get and start sync manager
	syncManager, err := getSyncManager(logger)
	if err != nil {
		return fmt.Errorf("failed to initialize sync: %w", err)
	}

	// Start the sync manager
	if err := syncManager.Start(); err != nil {
		return fmt.Errorf("failed to start sync: %w", err)
	}
	defer syncManager.Stop()

	// Get the list of paired devices
	devices := syncManager.GetPairedDevices()

	if len(devices) == 0 {
		fmt.Println("No paired devices found")
		fmt.Println("\nTo pair with a device:")
		fmt.Println("1. Run 'clipman pair' on this device to enter pairing mode")
		fmt.Println("2. Share the displayed address with your other device")
		fmt.Println("3. On the other device, run 'clipman pair --address <address>'")
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
	config := syncConfig(syncManager)
	if config != nil && config.DiscoveryMethod != "paired" {
		fmt.Println("\n⚠️  Note: Your current discovery method is not set to 'paired',")
		fmt.Println("   which is the recommended setting for security. Consider updating")
		fmt.Println("   your configuration to use paired discovery exclusively.")
	}

	return nil
}

// removePairedDevice removes a paired device
func removePairedDevice(peerID string, logger *zap.Logger) error {
	// Get and start sync manager
	syncManager, err := getSyncManager(logger)
	if err != nil {
		return fmt.Errorf("failed to initialize sync: %w", err)
	}

	// Start the sync manager
	if err := syncManager.Start(); err != nil {
		return fmt.Errorf("failed to start sync: %w", err)
	}
	defer syncManager.Stop()

	// Check if the device is paired
	if !syncManager.IsPaired(peerID) {
		// If not paired, list available paired devices
		devices := syncManager.GetPairedDevices()
		if len(devices) == 0 {
			return fmt.Errorf("no paired devices found - nothing to remove")
		}

		fmt.Println("⚠️  The specified peer ID is not paired")
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
	fmt.Printf("Are you sure you want to remove the paired device '%s' (%s)? (y/n): ",
		deviceName, peerID)
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
		fmt.Println("\n⚠️  Warning: The device still appears to be paired despite removal attempt")
		fmt.Println("   This could indicate a persistence issue in the sync system")
	} else {
		fmt.Println("Device removal confirmed")
	}

	return nil
}

// getSyncManager creates and initializes a sync manager instance
func getSyncManager(logger *zap.Logger) (types.SyncManager, error) {
	// Get config from globals
	cfg := GetConfig()
	if cfg == nil {
		return nil, fmt.Errorf("configuration not available")
	}
	
	// Create a new sync manager directly
	syncManager, err := p2p.New(context.Background(), cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create sync manager: %w", err)
	}

	// Check discovery method and warn if not using paired discovery
	config := syncConfig(syncManager)
	if config != nil && config.DiscoveryMethod != "paired" {
		logger.Warn("Using non-paired discovery method",
			zap.String("current_method", config.DiscoveryMethod),
			zap.String("recommended", "paired"))

		fmt.Println("⚠️  Warning: You're not using 'paired' discovery method, which is most secure.")
		fmt.Println("   Consider updating your config to use paired discovery exclusively.")
	}

	return syncManager, nil
}

// validateSyncConfig validates that sync configuration is properly set up for pairing
func validateSyncConfig(cfg *config.Config) error {
	// Check if sync is enabled
	if !cfg.Sync.Enabled {
		fmt.Println("❌ Synchronization is not enabled in your configuration")
		fmt.Println("\nTo enable sync, you can:")
		fmt.Println("  1. Edit your config file and set 'enabled: true' under the 'sync' section")
		fmt.Println("  2. Use environment variable: export CLIPMAN_SYNC_ENABLED=true")
		fmt.Println("  3. Use the config command: clipman config sync --enable")
		fmt.Println("\nExample config:")
		fmt.Println("  sync:")
		fmt.Println("    enabled: true")
		fmt.Println("    pairing_enabled: true")
		fmt.Println("    device_name: \"My Device\"")
		fmt.Println("    discovery_method: \"paired\"")
		return fmt.Errorf("synchronization is not enabled")
	}

	// Check device name
	if cfg.Sync.DeviceName == "" {
		fmt.Println("⚠️  Warning: No device name configured for sync")
		fmt.Printf("   Using system hostname: %s\n", cfg.DeviceName)
		cfg.Sync.DeviceName = cfg.DeviceName
	}

	// Check if pairing is enabled
	if !cfg.Sync.PairingEnabled {
		fmt.Println("⚠️  Warning: Pairing is not enabled in your configuration")
		fmt.Println("   This command will ask for temporary permission to enable pairing")
	}

	// Recommend secure settings
	var warnings []string
	if cfg.Sync.DiscoveryMethod != "paired" {
		warnings = append(warnings, fmt.Sprintf("Discovery method is '%s' (recommended: 'paired')", cfg.Sync.DiscoveryMethod))
	}
	if !cfg.Sync.AllowOnlyKnownPeers {
		warnings = append(warnings, "Allowing unknown peers to connect (recommended: false)")
	}
	if cfg.Sync.SyncOverInternet {
		warnings = append(warnings, "Internet sync enabled (consider if this is needed)")
	}

	if len(warnings) > 0 {
		fmt.Println("\n🔒 Security recommendations:")
		for _, warning := range warnings {
			fmt.Println("  •", warning)
		}
		fmt.Println()
	}

	return nil
}

// syncConfig gets the sync configuration safely
func syncConfig(syncManager types.SyncManager) *types.SyncConfig {
	if syncManager == nil {
		return nil
	}
	return syncManager.GetConfig()
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
