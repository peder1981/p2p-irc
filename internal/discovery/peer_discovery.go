// peer_discovery.go
package discovery

import (
	"net"
	"time"
	"github.com/peder1981/p2p-irc/internal/dht"
)

// PeerDiscovery is an interface for discovering peers.
type PeerDiscovery interface {
	Start() error
	Stop() error
	GetPeers() []dht.Node
	GetKnownPeers() []string
	ConnectToPeer(addr string) error
	SetMessageHandler(handler func(msg Message))
	GetInstanceID() string
	GetPort() int
}

// Basic implementation of Peer interface
type Peer struct {
	ID         dht.NodeID
	Addr       *net.UDPAddr
	LastSeen   time.Time
	InstanceID string
}
