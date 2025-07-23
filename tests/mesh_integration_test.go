package tests

import (
	"testing"
	"time"

	discovery "github.com/peder1981/p2p-irc/internal/discovery"
)

func TestMultiPeerChannelIntegration(t *testing.T) {
	// Usa mock para simular 3 peers IRC, canais sobrepostos e mensagens
	mock := NewMockPeerDiscovery()
	mock.RegisterPeer("peer1")
	mock.RegisterPeer("peer2")
	mock.RegisterPeer("peer3")

	msgs := make(chan discovery.Message, 20)
	mock.SetMessageHandler(func(msg discovery.Message) { msgs <- msg })

	// Join nos canais
	mock.JoinChannel("peer1", "canalA")
	mock.JoinChannel("peer2", "canalA")
	mock.JoinChannel("peer2", "canalB")
	mock.JoinChannel("peer3", "canalB")

	// Peer1 envia mensagem para canalA
	mock.SendChatMessageToChannel("peer1", "canalA", "msgA")
	// Peer2 envia mensagem para canalB
	mock.SendChatMessageToChannel("peer2", "canalB", "msgB")

	// Aguarda propagação
	time.Sleep(100 * time.Millisecond)

	// Valida recebimento
	var gotA2, gotB2, gotB3 bool
	for len(msgs) > 0 {
		msg := <-msgs
		if msg.Channel == "canalA" && msg.Content == "msgA" && msg.Sender == "peer1" {
			if msg.Sender != "peer2" {
				gotA2 = true
			}
		}
		if msg.Channel == "canalB" && msg.Content == "msgB" && msg.Sender == "peer2" {
			if msg.Sender != "peer3" {
				gotB2 = true
			}
			gotB3 = true
		}
	}
	if !gotA2 {
		t.Error("Peer2 não recebeu msgA de Peer1 em canalA (mock)")
	}
	if !gotB2 {
		t.Error("Peer2 não recebeu msgB em canalB (mock)")
	}
	if !gotB3 {
		t.Error("Peer3 não recebeu msgB de Peer2 em canalB (mock)")
	}

	// Testa comando /who (GetPeersInChannel)
	peersA := mock.GetPeersInChannel("canalA")
	peersB := mock.GetPeersInChannel("canalB")
	if len(peersA) < 2 {
		t.Error("Mock: canalA deveria ter pelo menos 2 peers")
	}
	if len(peersB) < 2 {
		t.Error("Mock: canalB deveria ter pelo menos 2 peers")
	}
}


