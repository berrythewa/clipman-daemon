package sync

import (
	"github.com/berrythewa/clipman-daemon/internal/config"
	"github.com/berrythewa/clipman-daemon/internal/types"
)


// GetConfigFromGlobal retrieves sync configuration from the global config
func GetConfigFromGlobal(cfg *config.Config) *types.SyncConfig {
    // Map from the global config to our internal sync config
    syncCfg := &types.SyncConfig{
        // Core Sync Settings
		Enabled:           cfg.Sync.Enabled,
        SyncOverInternet:  cfg.Sync.SyncOverInternet,
        UseRelayNodes:     cfg.Sync.UseRelayNodes,
        ListenPort:        cfg.Sync.ListenPort,
        DiscoveryMethod:   cfg.Sync.DiscoveryMethod,

        // Clipboard Sync Options		
        ClipboardTypes:    cfg.Sync.ClipboardTypes,
        AutoCopyFromPeers: cfg.Sync.AutoCopyFromPeers,
        MaxClipboardSizeKB: cfg.Sync.MaxClipboardSizeKB,
        ClipboardHistorySize: cfg.Sync.ClipboardHistorySize,
        ClipboardBlacklistApps: cfg.Sync.ClipboardBlacklistApps,

        // File Transfer Options
        EnableFileSharing: cfg.Sync.EnableFileSharing,
        RequireFileConfirmation: cfg.Sync.RequireFileConfirmation,
        DefaultDownloadFolder: cfg.Sync.DefaultDownloadFolder,
        MaxFileSizeMB: cfg.Sync.MaxFileSizeMB,

        // Privacy & Security
        AllowOnlyKnownPeers: cfg.Sync.AllowOnlyKnownPeers,
        TrustedPeers: cfg.Sync.TrustedPeers,
        RequireApprovalPin: cfg.Sync.RequireApprovalPin,
        LogPeerActivity: cfg.Sync.LogPeerActivity,

        // Developer & Debug Options
        DebugLogging: cfg.Sync.DebugLogging,
        ShowPeerDebugInfo: cfg.Sync.ShowPeerDebugInfo,
        DisableMultiplexing: cfg.Sync.DisableMultiplexing,
        ForceDirectConnectionOnly: cfg.Sync.ForceDirectConnectionOnly,
    }
    
    return syncCfg
}