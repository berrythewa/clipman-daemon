package config

import (
	"os"
	"path/filepath"
    "fmt"
    "github.com/berrythewa/clipman-daemon/internal/types"
)

// DefaultSyncConfig returns default sync configuration
func DefaultSyncConfig() types.SyncConfig {
    return types.SyncConfig{
        // Core Settings
        Enabled:           true,
        SyncOverInternet:  true,
        UseRelayNodes:     true,
        ListenPort:        0,  // 0 means use dynamic port
        DiscoveryMethod:   "mdns",
        
        // Clipboard Options
        ClipboardTypes:    []string{"text", "image"},
        AutoCopyFromPeers: true,
        MaxClipboardSizeKB: 512,
        ClipboardHistorySize: 50,
        ClipboardBlacklistApps: []string{},
        
        // File Transfer Options
        EnableFileSharing: true,
        RequireFileConfirmation: true,
        DefaultDownloadFolder: filepath.Join(os.Getenv("HOME"), "Downloads", "Clipman"),
        MaxFileSizeMB:     100,
        
        // Privacy & Security
        AllowOnlyKnownPeers: false,
        TrustedPeers:        []string{},
        RequireApprovalPin:  false,
        LogPeerActivity:     true,
        
        // Developer & Debug Options
        DebugLogging:              false,
        ShowPeerDebugInfo:         false,
        DisableMultiplexing:       false,
        ForceDirectConnectionOnly: false,
    }
}

// ValidateSyncConfig validates sync configuration options
func (c *Config) ValidateSyncConfig() error {
    // Validate discovery method
    switch c.Sync.DiscoveryMethod {
    case "mdns", "dht", "paired", "manual":
        // Valid options
    default:
        return fmt.Errorf("invalid discovery method: %s", c.Sync.DiscoveryMethod)
    }
    
    // Validate clipboard types
    for _, t := range c.Sync.ClipboardTypes {
        switch t {
        case "text", "image", "files", "url":
            // Valid options
        default:
            return fmt.Errorf("invalid clipboard type: %s", t)
        }
    }
    
    // Validate size limits
    if c.Sync.MaxClipboardSizeKB < 0 {
        return fmt.Errorf("max_clipboard_size_kb cannot be negative")
    }
    
    if c.Sync.MaxFileSizeMB < 0 {
        return fmt.Errorf("max_file_size_mb cannot be negative")
    }
    
    return nil
}

// GetSyncConfigForExport returns a copy of the sync config for external use
func (c *Config) GetSyncConfigForExport() *types.SyncConfig {
    // Create a copy to avoid modifying the original
    configCopy := c.Sync
    return &configCopy
}