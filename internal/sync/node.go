package sync

import (
    "context"
    "crypto/rand"
    "fmt"
    
    "github.com/libp2p/go-libp2p"
    "github.com/libp2p/go-libp2p/core/crypto"
    "github.com/libp2p/go-libp2p/core/host"
    "github.com/libp2p/go-libp2p/p2p/security/noise"
    "go.uber.org/zap"

	"github.com/berrythewa/clipman-daemon/internal/types"
	// "github.com/berrythewa/clipman-daemon/internal/config"
)

func NewNode(ctx context.Context, cfg *config.Config, logger *zap.Logger) (*Node, error) {
    syncCfg := cfg.Sync
    ctx, cancel := context.WithCancel(ctx)
    
    // Load or generate peer identity
    var priv crypto.PrivKey
    var err error
    if syncCfg.PeerIdentity != "" {
        // Load identity from config
        priv, err = loadIdentityFromString(syncCfg.PeerIdentity)
    } else {
        // Generate new identity
        priv, _, err = crypto.GenerateKeyPairWithReader(crypto.Ed25519, -1, rand.Reader)
    }
    
    if err != nil {
        cancel()
        return nil, err
    }
    
    // Create libp2p options based on config
    opts := []libp2p.Option{
        libp2p.Identity(priv),
        libp2p.Security(noise.ID, noise.New),
    }
    
    // Add listen addresses
    if syncCfg.ListenPort > 0 {
        opts = append(opts, libp2p.ListenAddrStrings(
            fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", syncCfg.ListenPort),
            fmt.Sprintf("/ip4/0.0.0.0/udp/%d/quic", syncCfg.ListenPort),
        ))
    } else {
        opts = append(opts, libp2p.ListenAddrStrings(
            "/ip4/0.0.0.0/tcp/0",
            "/ip4/0.0.0.0/udp/0/quic",
        ))
    }
    
    // Add NAT traversal if enabled
    if syncCfg.SyncOverInternet {
        opts = append(opts, libp2p.NATPortMap())
    }
    
    // Disable multiplexing if configured
    if syncCfg.DisableMultiplexing {
        opts = append(opts, libp2p.NoTransports)
        opts = append(opts, libp2p.Transport(tcp.NewTCPTransport))
    }
    
    // Create the host
    h, err := libp2p.New(opts...)
    if err != nil {
        cancel()
        return nil, err
    }
    
    // Log host creation
    logger.Info("Created libp2p host",
        zap.String("peer_id", h.ID().String()),
        zap.Strings("addresses", getHostAddresses(h)),
    )
    
    return &Node{
        host:   h,
        ctx:    ctx,
        cancel: cancel,
        logger: logger.With(zap.String("component", "libp2p-node")),
    }, nil
}

// Host returns the libp2p host
func (n *Node) Host() host.Host {
    return n.host
}

// Close closes the node and releases resources
func (n *Node) Close() error {
    n.cancel()
    return n.host.Close()
}

// Helper function to get all host addresses as strings
func getHostAddresses(h host.Host) []string {
    var addrs []string
    for _, addr := range h.Addrs() {
        addrs = append(addrs, fmt.Sprintf("%s/p2p/%s", addr.String(), h.ID().String()))
    }
    return addrs
}