package ui

import (
	"testing"
	"strings"
	"github.com/peder1981/p2p-irc/internal/dht"
	"github.com/peder1981/p2p-irc/internal/discovery"
	"fyne.io/fyne/v2/widget"
)

type mockDiscovery struct {
	knownPeers     []string
	peersInChannel map[string][]string
}

func (m *mockDiscovery) GetKnownPeers() []string {
	return m.knownPeers
}
// Métodos obrigatórios da interface PeerDiscovery
func (m *mockDiscovery) Start() error { return nil }
func (m *mockDiscovery) Stop() error { return nil }
func (m *mockDiscovery) GetPeers() []dht.Node { return nil }
func (m *mockDiscovery) ConnectToPeer(addr string) error { return nil }
func (m *mockDiscovery) SetMessageHandler(handler func(msg discovery.Message)) {}
func (m *mockDiscovery) GetInstanceID() string { return "mock" }
func (m *mockDiscovery) GetPort() int { return 0 }
// Helper para /who
func (m *mockDiscovery) GetPeersInChannel(channel string) []string {
	return m.peersInChannel[channel]
}

func TestHandlePeersCommand(t *testing.T) {
	g := &GUI{
		discovery: &mockDiscovery{knownPeers: []string{"peer1", "peer2"}},
		activeChannel: "#general",
		chatContent: make(map[string][]string),
		chatOutput: widget.NewTextGrid(),
	}
	input := "/peers"
	called := false
	fallback := func(s string) { called = true }

	ok := handlePeersCommand(g, g.discovery, input, fallback)
	if !ok {
		t.Errorf("/peers não foi interceptado")
	}
	if called {
		t.Errorf("Fallback não deveria ser chamado para /peers")
	}
	msgs := g.chatContent["#general"]
	if len(msgs) == 0 || !strings.Contains(msgs[len(msgs)-1], "peer1") {
		t.Errorf("Peer não exibido corretamente: %v", msgs)
	}
}

func TestHandleWhoCommand(t *testing.T) {
	g := &GUI{
		discovery: &mockDiscovery{peersInChannel: map[string][]string{"#general": {"peerA"}}},
		activeChannel: "#general",
		chatContent: make(map[string][]string),
	}
	input := "/who"
	ok := handleWhoCommand(g, g.discovery, input, nil)
	if !ok {
		t.Errorf("/who não foi interceptado")
	}
	msgs := g.chatContent["#general"]
	if len(msgs) == 0 || !strings.Contains(msgs[len(msgs)-1], "peerA") {
		t.Errorf("Peer do canal não exibido corretamente: %v", msgs)
	}
}

func TestHandleInfoCommand(t *testing.T) {
	g := &GUI{
		discovery: &mockDiscovery{knownPeers: []string{"peer1"}},
		activeChannel: "#general",
		chatContent: make(map[string][]string),
	}
	input := "/info"
	ok := handleInfoCommand(g, g.discovery, input, nil)
	if !ok {
		t.Errorf("/info não foi interceptado")
	}
	msgs := g.chatContent["#general"]
	if len(msgs) == 0 || !strings.Contains(msgs[len(msgs)-1], "Sessão IRC P2P") {
		t.Errorf("Info da sessão não exibida corretamente: %v", msgs)
	}
}

func TestPeersCommandEdgeCases(t *testing.T) {
	cases := []struct {
		peers []string
		expect string
	}{
		{[]string{}, "Nenhum peer conhecido"},
		{[]string{"peer1", "peer1", "peer2"}, "peer1"},
	}
	for _, c := range cases {
		g := &GUI{
			discovery: &mockDiscovery{knownPeers: c.peers},
			activeChannel: "#general",
			chatContent: make(map[string][]string),
		}
		handlePeersCommand(g, g.discovery, "/peers", nil)
		msgs := g.chatContent["#general"]
		if len(msgs) == 0 || !strings.Contains(msgs[len(msgs)-1], c.expect) {
			t.Errorf("Peer edge case não exibido corretamente para %v: %v", c.peers, msgs)
		}
	}
}
