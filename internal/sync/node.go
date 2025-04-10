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

// Node manages the libp2p host and networking
type Node struct {
    host   host.Host
    ctx    context.Context
    cancel context.CancelFunc
    logger *zap.Logger
}

// NewNode creates a new libp2p node
func NewNode(ctx context.Context, logger *zap.Logger) (*Node, error) {
    ctx, cancel := context.WithCancel(ctx)
    
    // Generate a new keypair for this host
    priv, _, err := crypto.GenerateKeyPairWithReader(crypto.Ed25519, -1, rand.Reader)
    if err != nil {
        cancel()
        return nil, err
    }
    
    // Create a new libp2p host using the keypair
    opts := []libp2p.Option{
        libp2p.Identity(priv),
        libp2p.Security(noise.ID, noise.New),
        libp2p.ListenAddrStrings(
            "/ip4/0.0.0.0/tcp/0",
            "/ip4/0.0.0.0/udp/0/quic",
        ),
        libp2p.NATPortMap(),
    }
    
    h, err := libp2p.New(opts...)
    if err != nil {
        cancel()
        return nil, err
    }
    
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