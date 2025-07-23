package tests

import (
	"testing"
	"time"

	discovery "github.com/peder1981/p2p-irc/internal/discovery"
)

func TestMultiPeerChannelIntegration(t *testing.T) {
	// Simula 3 peers IRC, canais sobrepostos e mensagens
	// Peer1: #canalA
	// Peer2: #canalA, #canalB
	// Peer3: #canalB

	d1, _ := discovery.New(nil, 0)
	d2, _ := discovery.New(nil, 0)
	d3, _ := discovery.New(nil, 0)

	d1.Start()
	defer d1.Stop()
	d2.Start()
	defer d2.Stop()
	d3.Start()
	defer d3.Stop()

	// Handlers para capturar mensagens recebidas em cada peer
	msgs1 := make(chan discovery.Message, 10)
	msgs2 := make(chan discovery.Message, 10)
	msgs3 := make(chan discovery.Message, 10)
	d1.SetMessageHandler(func(msg discovery.Message) { msgs1 <- msg })
	d2.SetMessageHandler(func(msg discovery.Message) { msgs2 <- msg })
	d3.SetMessageHandler(func(msg discovery.Message) { msgs3 <- msg })

	// Aguarda descoberta mútua
	time.Sleep(2 * time.Second)

	// Join nos canais
	d1.JoinChannel("canalA")
	d2.JoinChannel("canalA")
	d2.JoinChannel("canalB")
	d3.JoinChannel("canalB")

	// Aguarda sincronização
	time.Sleep(1 * time.Second)

	// Peer1 envia mensagem para canalA
	d1.SendChatMessageToChannel("canalA", "msgA", "peer1")
	// Peer2 envia mensagem para canalB
	d2.SendChatMessageToChannel("canalB", "msgB", "peer2")

	// Aguarda propagação
	time.Sleep(1 * time.Second)

	// Valida recebimento
	var gotA1, gotA2, gotB2, gotB3 bool
	// Peer2 deve receber msgA
	for len(msgs2) > 0 {
		msg := <-msgs2
		if msg.Channel == "canalA" && msg.Content == "msgA" {
			gotA2 = true
		}
		if msg.Channel == "canalB" && msg.Content == "msgB" {
			gotB2 = true
		}
	}
	// Peer1 não deve receber msgB (não está em canalB)
	for len(msgs1) > 0 {
		msg := <-msgs1
		if msg.Channel == "canalA" && msg.Content == "msgA" {
			gotA1 = true
		}
	}
	// Peer3 deve receber msgB
	for len(msgs3) > 0 {
		msg := <-msgs3
		if msg.Channel == "canalB" && msg.Content == "msgB" {
			gotB3 = true
		}
	}
	if !gotA2 {
		t.Error("Peer2 não recebeu msgA de Peer1 em canalA")
	}
	if gotA1 {
		t.Error("Peer1 não deveria receber sua própria msgA")
	}
	if !gotB2 {
		t.Error("Peer2 não recebeu msgB em canalB")
	}
	if !gotB3 {
		t.Error("Peer3 não recebeu msgB de Peer2 em canalB")
	}

	// Testa comando /who (GetPeersInChannel)
	peersA := d1.GetPeersInChannel("canalA")
	peersB := d2.GetPeersInChannel("canalB")
	if len(peersA) == 0 {
		t.Error("Nenhum peer listado em canalA para Peer1")
	}
	if len(peersB) == 0 {
		t.Error("Nenhum peer listado em canalB para Peer2")
	}

	// Simula reconexão
	d3.Stop()
	time.Sleep(500 * time.Millisecond)
	d3, _ = discovery.New(nil, 0)
	d3.Start()
	defer d3.Stop()
	d3.JoinChannel("canalB")
	time.Sleep(1 * time.Second)
	// Peer2 envia novamente para canalB
	msgs3 = make(chan discovery.Message, 10)
	d3.SetMessageHandler(func(msg discovery.Message) { msgs3 <- msg })
	d2.SendChatMessageToChannel("canalB", "msgB2", "peer2")
	time.Sleep(1 * time.Second)
	gotB3 = false
	for len(msgs3) > 0 {
		msg := <-msgs3
		if msg.Channel == "canalB" && msg.Content == "msgB2" {
			gotB3 = true
		}
	}
	if !gotB3 {
		t.Error("Peer3 não recebeu msgB2 após reconexão")
	}
}


