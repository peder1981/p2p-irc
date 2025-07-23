package tests

import (
	"sync"
	"github.com/peder1981/p2p-irc/internal/discovery"
)

type MockPeerDiscovery struct {
	mu       sync.RWMutex
	peers    map[string]*MockPeer
	channels map[string]map[string]bool // canal -> peers
	msgs     []discovery.Message
	msgHandler func(msg discovery.Message)
}

type MockPeer struct {
	ID      string
	channels map[string]bool
}

func NewMockPeerDiscovery() *MockPeerDiscovery {
	return &MockPeerDiscovery{
		peers:    make(map[string]*MockPeer),
		channels: make(map[string]map[string]bool),
	}
}

func (m *MockPeerDiscovery) RegisterPeer(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.peers[id] = &MockPeer{ID: id, channels: make(map[string]bool)}
}

func (m *MockPeerDiscovery) JoinChannel(peerID, channel string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	peer, ok := m.peers[peerID]
	if !ok { return }
	peer.channels[channel] = true
	if m.channels[channel] == nil {
		m.channels[channel] = make(map[string]bool)
	}
	m.channels[channel][peerID] = true
	// Simula broadcast JOIN
	if m.msgHandler != nil {
		m.msgHandler(discovery.Message{Type: discovery.TypeJoinChannel, Sender: peerID, Channel: channel})
	}
}

func (m *MockPeerDiscovery) SendChatMessageToChannel(peerID, channel, content string) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for pid := range m.channels[channel] {
		if pid == peerID { continue }
		if m.msgHandler != nil {
			m.msgHandler(discovery.Message{Type: discovery.TypeChatMessage, Sender: peerID, Channel: channel, Content: content})
		}
	}
}

func (m *MockPeerDiscovery) GetPeersInChannel(channel string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	peers := make([]string, 0)
	for pid := range m.channels[channel] {
		peers = append(peers, pid)
	}
	return peers
}

func (m *MockPeerDiscovery) SetMessageHandler(handler func(msg discovery.Message)) {
	m.msgHandler = handler
}
