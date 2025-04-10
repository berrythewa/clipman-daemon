package config

import (
	"os"
	"path/filepath"
)

// DefaultSyncConfig returns default sync configuration
func DefaultSyncConfig() SyncConfig {
    return SyncConfig{
        Enabled:           true,
        SyncOverInternet:  true,
        UseRelayNodes:     true,
        ListenPort:        0,  // 0 means use dynamic port
        DiscoveryMethod:   "mdns",
        
        ClipboardTypes:    []string{"text", "image"},
        AutoCopyFromPeers: true,
        MaxClipboardSizeKB: 512,
        ClipboardHistorySize: 50,
        
        EnableFileSharing: true,
        RequireFileConfirmation: true,
        DefaultDownloadFolder: filepath.Join(os.Getenv("HOME"), "Downloads", "Clipman"),
        MaxFileSizeMB:     100,
        
        AllowOnlyKnownPeers: false,
        LogPeerActivity:    true,
        
        DebugLogging:      false,
        ShowPeerDebugInfo: false,
    }
}

// ValidateSyncConfig validates sync configuration options
func (c *Config) ValidateSyncConfig() error {
    // Validate discovery method
    switch c.Sync.DiscoveryMethod {
    case "mdns", "dht", "manual":
        // Valid options
    default:
        return fmt.Errorf("invalid discovery method: %s", c.Sync.DiscoveryMethod)
    }
    
    // Validate clipboard types
    for _, t := range c.Sync.ClipboardTypes {
        switch t {
        case "text", "image", "files":
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