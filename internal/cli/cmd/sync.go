package cmd

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/config"
	"github.com/berrythewa/clipman-daemon/internal/sync"
	"github.com/berrythewa/clipman-daemon/internal/storage"
	"github.com/spf13/cobra"
)

var (
	syncGroupName string
	syncMode      string
	maxSyncSize   int64
)

// syncCmd represents the sync command
var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Manage synchronization settings",
	Long: `Manage Clipman synchronization settings including mode, groups, and filters.

Examples:
  clipman sync status         # Show current sync status and configuration
  clipman sync mode p2p       # Set to peer-to-peer mode
  clipman sync join work      # Join a sync group named "work"
  clipman sync leave work     # Leave a sync group
  clipman sync groups         # List all joined groups`,
	Run: func(cmd *cobra.Command, args []string) {
		// Default to showing sync status
		showSyncStatus()
	},
}

// syncStatusCmd shows the current sync status
var syncStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Shows sync status",
	Run: func(cmd *cobra.Command, args []string) {
		// Try using the daemon client first
		daemonClient := sync.NewDaemonClient(cfg)
		if daemonClient.IsDaemonRunning() {
			// Get status from daemon
			resp, err := daemonClient.GetStatus()
			if err != nil {
				fmt.Printf("Error communicating with daemon: %v\n", err)
				os.Exit(1)
			}
			
			if !resp.Success {
				fmt.Printf("Error getting sync status: %v\n", resp.Error)
				os.Exit(1)
			}
			
			// Display status from daemon response
			fmt.Println("Synchronization Status (via daemon):")
			
			// Extract data from response
			if resp.Data != nil {
				if status, ok := resp.Data.(map[string]interface{}); ok {
					// Show mode
					if mode, exists := status["mode"].(string); exists {
						fmt.Printf("  Mode:          %s\n", mode)
					}
					
					// Show default group
					if defaultGroup, exists := status["default_group"].(string); exists {
						fmt.Printf("  Default Group: %s\n", valueOrNone(defaultGroup))
					}
					
					// Show auto-join
					if autoJoin, exists := status["auto_join"].(bool); exists {
						fmt.Printf("  Auto-Join:     %v\n", autoJoin)
					}
					
					// Show URL
					if url, exists := status["url"].(string); exists {
						fmt.Printf("  MQTT URL:      %s\n", valueOrNone(url))
					}
					
					// Show connection status
					if connected, exists := status["connected"].(bool); exists {
						if connected {
							fmt.Println("  Connection:    Connected")
						} else {
							fmt.Println("  Connection:    Disconnected")
						}
					}
				}
			}
			
			// Show groups
			if len(resp.Groups) > 0 {
				fmt.Printf("  Groups:        %s\n", strings.Join(resp.Groups, ", "))
			} else {
				fmt.Println("  Groups:        None")
			}
			
			// Show filter settings
			if resp.Data != nil {
				if status, ok := resp.Data.(map[string]interface{}); ok {
					fmt.Println("\nFilter Settings:")
					
					if maxSize, exists := status["max_sync_size"].(float64); exists {
						fmt.Printf("  Max Size:      %d bytes\n", int64(maxSize))
					}
					
					if allowedTypes, exists := status["allowed_types"].([]interface{}); exists {
						strTypes := make([]string, 0, len(allowedTypes))
						for _, t := range allowedTypes {
							if str, ok := t.(string); ok {
								strTypes = append(strTypes, str)
							}
						}
						fmt.Printf("  Allowed Types: %s\n", listOrNone(strTypes))
					}
					
					if excludedTypes, exists := status["excluded_types"].([]interface{}); exists {
						strTypes := make([]string, 0, len(excludedTypes))
						for _, t := range excludedTypes {
							if str, ok := t.(string); ok {
								strTypes = append(strTypes, str)
							}
						}
						fmt.Printf("  Excluded Types: %s\n", listOrNone(strTypes))
					}
				}
			}
			
			return
		}
		
		// Fallback to direct connection
		fmt.Println("Daemon not running, connecting directly...")
		showSyncStatus()
	},
}

// syncModeCmd sets the sync mode
var syncModeCmd = &cobra.Command{
	Use:   "mode [p2p|centralized]",
	Short: "Set the synchronization mode",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// If no args, just show current mode
		if len(args) == 0 {
			fmt.Printf("Current sync mode: %s\n", cfg.Sync.Mode)
			return
		}

		mode := strings.ToLower(args[0])
		if mode != config.SyncModeP2P && mode != config.SyncModeCentralized {
			fmt.Printf("Invalid mode: %s (must be 'p2p' or 'centralized')\n", mode)
			return
		}

		// Update config
		cfg.Sync.Mode = mode
		if err := cfg.Save(); err != nil {
			fmt.Printf("Error saving config: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Sync mode set to: %s\n", mode)
		fmt.Println("Restart Clipman for changes to take effect.")
	},
}

// syncJoinCmd joins a sync group
var syncJoinCmd = &cobra.Command{
	Use:   "join [group_name1,group_name2,...]",
	Short: "Join one or more synchronization groups",
	Long: `Join one or more synchronization groups.

Examples:
  clipman sync join work              # Join a single group
  clipman sync join work,home,family  # Join multiple groups at once
`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// Parse comma-separated group names
		groupNames := strings.Split(args[0], ",")
		var groupList []string
		
		for _, name := range groupNames {
			name = strings.TrimSpace(name)
			if name != "" {
				groupList = append(groupList, name)
			}
		}
		
		if len(groupList) == 0 {
			fmt.Println("No valid group names specified")
			os.Exit(1)
		}
		
		// Try using the daemon client first
		daemonClient := sync.NewDaemonClient(cfg)
		if daemonClient.IsDaemonRunning() {
			fmt.Println("Using sync daemon...")
			resp, err := daemonClient.JoinGroups(groupList)
			
			if err != nil {
				fmt.Printf("Error communicating with sync daemon: %v\n", err)
				os.Exit(1)
			}
			
			// Process response
			if resp.Success {
				fmt.Println(resp.Message)
				
				// Display any partial errors
				if len(resp.Errors) > 0 {
					fmt.Println("\nSome groups could not be joined:")
					for _, errMsg := range resp.Errors {
						fmt.Printf("  - %s\n", errMsg)
					}
				}
				
				if len(resp.Groups) > 0 {
					fmt.Println("\nSuccessfully joined groups:")
					for _, group := range resp.Groups {
						fmt.Printf("  - %s\n", group)
					}
				}
				
				return
			} else {
				fmt.Printf("Error: %s\n", resp.Message)
				if len(resp.Errors) > 0 {
					fmt.Println("\nErrors:")
					for _, errMsg := range resp.Errors {
						fmt.Printf("  - %s\n", errMsg)
					}
				}
				os.Exit(1)
			}
		}
		
		// Fallback to direct connection
		fmt.Println("No sync daemon running, using direct connection...")
		
		// Create sync client (only once for all operations)
		syncClient, err := sync.CreateClient(cfg, zapLogger)
		if err != nil {
			fmt.Printf("Error connecting to sync system: %v\n", err)
			os.Exit(1)
		}
		defer syncClient.Disconnect() // Ensure we properly disconnect

		successCount := 0
		firstSuccessGroup := ""
		
		for _, groupName := range groupList {
			fmt.Printf("Joining group: %s... ", groupName)
			
			// Join the group
			if err := syncClient.JoinGroup(groupName); err != nil {
				fmt.Printf("Error: %v\n", err)
				continue
			}

			fmt.Println("Success!")
			successCount++
			
			// Remember first successful group for default group setting
			if firstSuccessGroup == "" {
				firstSuccessGroup = groupName
			}
		}
		
		// Update default group in config if this is the first group
		if successCount > 0 && cfg.Sync.DefaultGroup == "" {
			cfg.Sync.DefaultGroup = firstSuccessGroup
			if err := cfg.Save(); err != nil {
				fmt.Printf("Warning: Error saving config: %v\n", err)
				// Don't exit, joining the group was successful
			}
		}

		if successCount > 0 {
			fmt.Printf("Successfully joined %d group(s)\n", successCount)
		} else {
			fmt.Println("No groups were joined successfully")
			os.Exit(1)
		}
	},
}

// syncLeaveCmd leaves a sync group
var syncLeaveCmd = &cobra.Command{
	Use:   "leave [group_name1,group_name2,...]",
	Short: "Leave one or more synchronization groups",
	Long: `Leave one or more synchronization groups.

Examples:
  clipman sync leave work              # Leave a single group
  clipman sync leave work,home,family  # Leave multiple groups at once
`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// Parse comma-separated group names
		groupNames := strings.Split(args[0], ",")
		
		// Create sync client (only once for all operations)
		syncClient, err := sync.CreateClient(cfg, zapLogger)
		if err != nil {
			fmt.Printf("Error connecting to sync system: %v\n", err)
			os.Exit(1)
		}
		defer syncClient.Disconnect() // Ensure we properly disconnect

		successCount := 0
		for _, groupName := range groupNames {
			groupName = strings.TrimSpace(groupName)
			if groupName == "" {
				continue
			}
			
			fmt.Printf("Leaving group: %s... ", groupName)
			
			// Leave the group
			if err := syncClient.LeaveGroup(groupName); err != nil {
				fmt.Printf("Error: %v\n", err)
				continue
			}

			// If this was the default group, clear that setting
			if cfg.Sync.DefaultGroup == groupName {
				cfg.Sync.DefaultGroup = ""
				if err := cfg.Save(); err != nil {
					fmt.Printf("Warning: Error saving config: %v\n", err)
					// Don't exit, leaving the group was successful
				}
			}

			fmt.Println("Success!")
			successCount++
		}

		if successCount > 0 {
			fmt.Printf("Successfully left %d group(s)\n", successCount)
		} else {
			fmt.Println("No groups were left successfully")
			os.Exit(1)
		}
	},
}

// syncGroupsCmd lists all joined groups
var syncGroupsCmd = &cobra.Command{
	Use:   "groups",
	Short: "List all joined synchronization groups",
	Run: func(cmd *cobra.Command, args []string) {
		// Try using the daemon client first
		daemonClient := sync.NewDaemonClient(cfg)
		if daemonClient.IsDaemonRunning() {
			fmt.Println("Using sync daemon...")
			resp, err := daemonClient.ListGroups()
			
			if err != nil {
				fmt.Printf("Error communicating with sync daemon: %v\n", err)
				os.Exit(1)
			}
			
			// Process response
			if resp.Success {
				if len(resp.Groups) == 0 {
					fmt.Println("Not a member of any synchronization groups.")
					return
				}
				
				fmt.Println("Synchronization groups:")
				for _, group := range resp.Groups {
					if group == cfg.Sync.DefaultGroup {
						fmt.Printf("  * %s (default)\n", group)
					} else {
						fmt.Printf("  - %s\n", group)
					}
				}
				return
			} else {
				fmt.Printf("Error: %s\n", resp.Message)
				if len(resp.Errors) > 0 {
					for _, errMsg := range resp.Errors {
						fmt.Printf("  - %s\n", errMsg)
					}
				}
				os.Exit(1)
			}
		}
		
		// Fallback to direct connection
		fmt.Println("No sync daemon running, using direct connection...")
		
		// Create sync client
		syncClient, err := sync.CreateClient(cfg, zapLogger)
		if err != nil {
			fmt.Printf("Error connecting to sync system: %v\n", err)
			os.Exit(1)
		}
		defer syncClient.Disconnect()

		// Get list of groups
		groups, err := syncClient.ListGroups()
		if err != nil {
			fmt.Printf("Error listing groups: %v\n", err)
			os.Exit(1)
		}

		if len(groups) == 0 {
			fmt.Println("Not a member of any synchronization groups.")
			return
		}

		fmt.Println("Synchronization groups:")
		for _, group := range groups {
			if group == cfg.Sync.DefaultGroup {
				fmt.Printf("  * %s (default)\n", group)
			} else {
				fmt.Printf("  - %s\n", group)
			}
		}
	},
}

// syncFilterCmd manages content filtering
var syncFilterCmd = &cobra.Command{
	Use:   "filter",
	Short: "Configure content filtering for synchronization",
	Long: `Configure which clipboard content types are synchronized.

Examples:
  clipman sync filter status            # Show current filter settings
  clipman sync filter --max-size 1MB    # Set maximum content size
  clipman sync filter --allow text,url  # Only allow text and URLs`,
	Run: func(cmd *cobra.Command, args []string) {
		// If no specific flags, show current filter settings
		showFilterStatus()
	},
}

// syncFilterStatusCmd shows current filter settings
var syncFilterStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current filter settings",
	Run: func(cmd *cobra.Command, args []string) {
		showFilterStatus()
	},
}

// syncResyncCmd resyncs the clipboard history with other devices
var syncResyncCmd = &cobra.Command{
	Use:   "resync",
	Short: "Resynchronize clipboard history with other devices",
	Long: `Resynchronize your clipboard history with other connected devices.
	
This will publish your entire clipboard history to all the groups you have joined.
Use this when you want to ensure all devices have the same clipboard history.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Try using the daemon client first
		daemonClient := sync.NewDaemonClient(cfg)
		if daemonClient.IsDaemonRunning() {
			fmt.Println("Using sync daemon...")
			resp, err := daemonClient.Resync()
			
			if err != nil {
				fmt.Printf("Error communicating with sync daemon: %v\n", err)
				os.Exit(1)
			}
			
			if !resp.Success {
				fmt.Printf("Error resyncing clipboard history: %v\n", resp.Error)
				os.Exit(1)
			}
			
			fmt.Println(resp.Message)
			return
		}
		
		// Fallback to direct connection
		fmt.Println("No sync daemon running, using direct connection...")
	
		// Create storage to access clipboard history
		storageConfig := storage.StorageConfig{
			DBPath:   cfg.Storage.DBPath,
			MaxSize:  cfg.Storage.MaxSize,
			DeviceID: cfg.DeviceID,
			Logger:   zapLogger,
		}
		
		store, err := storage.NewBoltStorage(storageConfig)
		if err != nil {
			fmt.Printf("Error opening storage: %v\n", err)
			os.Exit(1)
		}
		defer store.Close()
		
		// Create sync client
		syncClient, err := sync.CreateClient(cfg, zapLogger)
		if err != nil {
			fmt.Printf("Error creating sync client: %v\n", err)
			os.Exit(1)
		}
		
		// Set the sync client for the storage
		store.SetSyncClient(syncClient)
		
		// Publish the entire clipboard history
		timeZero := time.Time{} // Unix epoch 0
		fmt.Println("Resyncing clipboard history...")
		if err := store.PublishCacheHistory(timeZero); err != nil {
			fmt.Printf("Error publishing cache history: %v\n", err)
			os.Exit(1)
		}
		
		fmt.Println("Successfully resynced clipboard history!")
	},
}

// syncDaemonCmd manages the sync daemon
var syncDaemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Manage the sync daemon",
	Long: `Manage the synchronization daemon.

The daemon provides a persistent sync client that handles operations more efficiently.
When running, all sync commands will use the daemon instead of creating new connections.

Examples:
  clipman sync daemon start    # Start the sync daemon
  clipman sync daemon stop     # Stop the sync daemon
  clipman sync daemon status   # Check if the daemon is running`,
	Run: func(cmd *cobra.Command, args []string) {
		// Default to showing daemon status
		showDaemonStatus()
	},
}

// syncDaemonStartCmd starts the sync daemon
var syncDaemonStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the sync daemon",
	Run: func(cmd *cobra.Command, args []string) {
		// Check if already running
		daemonClient := sync.NewDaemonClient(cfg)
		if daemonClient.IsDaemonRunning() {
			fmt.Println("Sync daemon is already running")
			return
		}
		
		// Create the daemon
		daemon, err := sync.NewSyncDaemon(cfg, zapLogger)
		if err != nil {
			fmt.Printf("Error creating sync daemon: %v\n", err)
			os.Exit(1)
		}
		
		// Start the daemon
		if err := daemon.Start(); err != nil {
			fmt.Printf("Error starting sync daemon: %v\n", err)
			os.Exit(1)
		}
		
		fmt.Println("Sync daemon started successfully")
		fmt.Println("Note: The daemon will stop when you close this terminal")
		fmt.Println("For persistent daemon, use 'clipmand run --daemon' instead")
		
		// Keep running until interrupted
		fmt.Println("Press Ctrl+C to stop the daemon...")
		
		// Block forever (or until interrupted)
		select {}
	},
}

// syncDaemonStopCmd stops the sync daemon
var syncDaemonStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the sync daemon",
	Run: func(cmd *cobra.Command, args []string) {
		// Check if running
		daemonClient := sync.NewDaemonClient(cfg)
		if !daemonClient.IsDaemonRunning() {
			fmt.Println("Sync daemon is not running")
			return
		}
		
		// Get daemon socket path
		sockPath := filepath.Join(cfg.SystemPaths.DataDir, "sockets", "clipman-sync.sock")
		
		// Create a basic message to stop the daemon
		// Note: This is a bit of a hack, but we don't have a proper
		// stop command in the daemon protocol. A real solution would
		// add a "shutdown" command to the protocol.
		fmt.Println("Stopping sync daemon...")
		conn, err := net.Dial("unix", sockPath)
		if err != nil {
			fmt.Printf("Error connecting to daemon: %v\n", err)
			os.Exit(1)
		}
		conn.Close()
		
		// Wait a bit and check if it's still running
		time.Sleep(500 * time.Millisecond)
		if !daemonClient.IsDaemonRunning() {
			fmt.Println("Sync daemon stopped successfully")
		} else {
			fmt.Println("Failed to stop sync daemon, it's still running")
			fmt.Println("If needed, you can manually kill the process")
			os.Exit(1)
		}
	},
}

// syncDaemonStatusCmd shows the status of the sync daemon
var syncDaemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show sync daemon status",
	Run: func(cmd *cobra.Command, args []string) {
		showDaemonStatus()
	},
}

// showDaemonStatus displays the status of the sync daemon
func showDaemonStatus() {
	daemonClient := sync.NewDaemonClient(cfg)
	isRunning := daemonClient.IsDaemonRunning()
	
	fmt.Println("Sync Daemon Status:")
	if isRunning {
		fmt.Println("  Status: Running")
		
		// Try to get more info from the daemon
		resp, err := daemonClient.GetStatus()
		if err == nil && resp.Success {
			// Extract useful information
			if resp.Data != nil {
				if status, ok := resp.Data.(map[string]interface{}); ok {
					if connected, exists := status["connected"].(bool); exists && connected {
						fmt.Println("  Connection: Connected to MQTT broker")
					} else {
						fmt.Println("  Connection: Not connected to MQTT broker")
					}
					
					if mode, exists := status["mode"].(string); exists {
						fmt.Printf("  Mode: %s\n", mode)
					}
					
					if defaultGroup, exists := status["default_group"].(string); exists && defaultGroup != "" {
						fmt.Printf("  Default Group: %s\n", defaultGroup)
					}
				}
			}
			
			if len(resp.Groups) > 0 {
				fmt.Println("  Groups:")
				for _, group := range resp.Groups {
					fmt.Printf("    - %s\n", group)
				}
			} else {
				fmt.Println("  Groups: None")
			}
		}
	} else {
		fmt.Println("  Status: Not running")
		fmt.Println("\nTo start the daemon, run:")
		fmt.Println("  clipman sync daemon start")
	}
}

// Helper function to show sync status
func showSyncStatus() {
	fmt.Println("Synchronization Status:")
	fmt.Printf("  Mode:          %s\n", cfg.Sync.Mode)
	fmt.Printf("  Default Group: %s\n", valueOrNone(cfg.Sync.DefaultGroup))
	fmt.Printf("  Auto-Join:     %v\n", cfg.Sync.AutoJoinGroups)
	fmt.Printf("  MQTT URL:      %s\n", valueOrNone(cfg.Sync.URL))
	
	// Check connection status if possible
	if cfg.Sync.URL != "" {
		syncClient, err := sync.CreateClient(cfg, logger)
		if err != nil {
			fmt.Printf("  Connection:    Error (%v)\n", err)
		} else {
			if syncClient.IsConnected() {
				fmt.Println("  Connection:    Connected")
			} else {
				fmt.Println("  Connection:    Disconnected")
			}
			
			// Try to list groups
			groups, err := syncClient.ListGroups()
			if err == nil {
				fmt.Printf("  Groups:        %s\n", listOrNone(groups))
			} else {
				fmt.Printf("  Groups:        Error (%v)\n", err)
			}
		}
	} else {
		fmt.Println("  Connection:    Not configured")
	}

	// Show filter settings
	fmt.Println("\nFilter Settings:")
	fmt.Printf("  Max Size:      %d bytes\n", cfg.Sync.MaxSyncSize)
	fmt.Printf("  Allowed Types: %s\n", listOrNone(cfg.Sync.AllowedTypes))
	fmt.Printf("  Excluded Types: %s\n", listOrNone(cfg.Sync.ExcludedTypes))
}

// Helper function to show filter status
func showFilterStatus() {
	fmt.Println("Sync Filter Settings:")
	fmt.Printf("  Max Size:       %d bytes\n", cfg.Sync.MaxSyncSize)
	fmt.Printf("  Allowed Types:  %s\n", listOrNone(cfg.Sync.AllowedTypes))
	fmt.Printf("  Excluded Types: %s\n", listOrNone(cfg.Sync.ExcludedTypes))
	fmt.Printf("  Include Patterns: %s\n", listOrNone(cfg.Sync.IncludePatterns))
	fmt.Printf("  Exclude Patterns: %s\n", listOrNone(cfg.Sync.ExcludePatterns))
}

// Helper function to display a value or "None" if empty
func valueOrNone(value string) string {
	if value == "" {
		return "None"
	}
	return value
}

// Helper function to display a list or "None" if empty
func listOrNone(list []string) string {
	if len(list) == 0 {
		return "None"
	}
	return strings.Join(list, ", ")
}

func init() {
	RootCmd.AddCommand(syncCmd)

	// Add subcommands
	syncCmd.AddCommand(syncStatusCmd)
	syncCmd.AddCommand(syncModeCmd)
	syncCmd.AddCommand(syncJoinCmd)
	syncCmd.AddCommand(syncLeaveCmd)
	syncCmd.AddCommand(syncGroupsCmd)
	syncCmd.AddCommand(syncFilterCmd)
	syncCmd.AddCommand(syncResyncCmd)
	syncCmd.AddCommand(syncDaemonCmd)
	syncFilterCmd.AddCommand(syncFilterStatusCmd)

	// Register daemon subcommands
	syncDaemonCmd.AddCommand(syncDaemonStartCmd)
	syncDaemonCmd.AddCommand(syncDaemonStopCmd)
	syncDaemonCmd.AddCommand(syncDaemonStatusCmd)

	// Add flags
	syncFilterCmd.Flags().Int64Var(&maxSyncSize, "max-size", 0, "Maximum content size in bytes for synchronization")
	syncFilterCmd.Flags().StringSlice("allow", []string{}, "Content types to allow (comma-separated list)")
	syncFilterCmd.Flags().StringSlice("exclude", []string{}, "Content types to exclude (comma-separated list)")
	syncFilterCmd.Flags().StringSlice("include-pattern", []string{}, "Regex patterns to include (comma-separated list)")
	syncFilterCmd.Flags().StringSlice("exclude-pattern", []string{}, "Regex patterns to exclude (comma-separated list)")
} 