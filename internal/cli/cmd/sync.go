package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/berrythewa/clipman-daemon/internal/config"
	"github.com/berrythewa/clipman-daemon/internal/sync"
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

// syncStatusCmd shows current sync status
var syncStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current sync status",
	Run: func(cmd *cobra.Command, args []string) {
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
	Use:   "join [group_name]",
	Short: "Join a synchronization group",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		groupName := args[0]

		// Create sync client
		syncClient, err := sync.CreateClient(cfg, logger)
		if err != nil {
			fmt.Printf("Error connecting to sync system: %v\n", err)
			os.Exit(1)
		}

		// Join the group
		if err := syncClient.JoinGroup(groupName); err != nil {
			fmt.Printf("Error joining group '%s': %v\n", groupName, err)
			os.Exit(1)
		}

		// Update default group in config if this is the first group
		if cfg.Sync.DefaultGroup == "" {
			cfg.Sync.DefaultGroup = groupName
			if err := cfg.Save(); err != nil {
				fmt.Printf("Error saving config: %v\n", err)
				// Don't exit, joining the group was successful
			}
		}

		fmt.Printf("Successfully joined group: %s\n", groupName)
	},
}

// syncLeaveCmd leaves a sync group
var syncLeaveCmd = &cobra.Command{
	Use:   "leave [group_name]",
	Short: "Leave a synchronization group",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		groupName := args[0]

		// Create sync client
		syncClient, err := sync.CreateClient(cfg, logger)
		if err != nil {
			fmt.Printf("Error connecting to sync system: %v\n", err)
			os.Exit(1)
		}

		// Leave the group
		if err := syncClient.LeaveGroup(groupName); err != nil {
			fmt.Printf("Error leaving group '%s': %v\n", groupName, err)
			os.Exit(1)
		}

		// If this was the default group, clear that setting
		if cfg.Sync.DefaultGroup == groupName {
			cfg.Sync.DefaultGroup = ""
			if err := cfg.Save(); err != nil {
				fmt.Printf("Error saving config: %v\n", err)
				// Don't exit, leaving the group was successful
			}
		}

		fmt.Printf("Successfully left group: %s\n", groupName)
	},
}

// syncGroupsCmd lists all joined groups
var syncGroupsCmd = &cobra.Command{
	Use:   "groups",
	Short: "List all joined synchronization groups",
	Run: func(cmd *cobra.Command, args []string) {
		// Create sync client
		syncClient, err := sync.CreateClient(cfg, logger)
		if err != nil {
			fmt.Printf("Error connecting to sync system: %v\n", err)
			os.Exit(1)
		}

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
	syncFilterCmd.AddCommand(syncFilterStatusCmd)

	// Add flags
	syncFilterCmd.Flags().Int64Var(&maxSyncSize, "max-size", 0, "Maximum content size in bytes for synchronization")
	syncFilterCmd.Flags().StringSlice("allow", []string{}, "Content types to allow (comma-separated list)")
	syncFilterCmd.Flags().StringSlice("exclude", []string{}, "Content types to exclude (comma-separated list)")
	syncFilterCmd.Flags().StringSlice("include-pattern", []string{}, "Regex patterns to include (comma-separated list)")
	syncFilterCmd.Flags().StringSlice("exclude-pattern", []string{}, "Regex patterns to exclude (comma-separated list)")
} 