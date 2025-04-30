package discovery

import (
	"fmt"
	"time"
)

// SyncPeers sincroniza os canais e peers entre todas as conexões
// Esta função é crucial para garantir que todos os peers saibam quais canais
// cada um está participando
func (d *Discovery) SyncPeers() {
	// Obtém todos os canais em que estamos
	d.channelMu.RLock()
	channels := make([]string, 0, len(d.channels))
	for channel := range d.channels {
		channels = append(channels, channel)
	}
	d.channelMu.RUnlock()

	// Obtém todas as conexões ativas
	d.connMu.RLock()
	connections := make(map[string]*PeerConnection)
	for addr, conn := range d.connections {
		connections[addr] = conn
	}
	d.connMu.RUnlock()

	fmt.Printf("[DEBUG] Sincronizando %d canais com %d peers\n", 
		len(channels), len(connections))

	// Para cada canal, envia uma mensagem JOIN para todos os peers
	for _, channel := range channels {
		joinMsg := Message{
			Type:      TypeJoinChannel,
			Sender:    d.instanceID,
			Channel:   channel,
			Timestamp: time.Now(),
		}

		// Envia para todos os peers
		for addr, peerConn := range connections {
			if err := peerConn.SendMessage(joinMsg); err != nil {
				fmt.Printf("[ERRO] Falha ao enviar JOIN para peer %s (canal %s): %v\n", 
					addr, channel, err)
			} else {
				fmt.Printf("[DEBUG] JOIN enviado para peer %s (canal %s)\n", 
					addr, channel)
			}
		}
	}
}

// EnhancedBroadcastMessage é uma versão melhorada do BroadcastMessage
// que garante que a mensagem seja entregue a todos os peers no canal
func (d *Discovery) EnhancedBroadcastMessage(msg Message) {
	d.connMu.RLock()
	defer d.connMu.RUnlock()

	fmt.Printf("[DEBUG] EnhancedBroadcast: tipo=%s, canal=%s, %d peers conectados\n", 
		msg.Type, msg.Channel, len(d.connections))

	// Conta quantos peers receberam a mensagem
	sentCount := 0
	failCount := 0

	for addr, peerConn := range d.connections {
		// Para mensagens de chat, verifica se o peer está no canal
		if msg.Type == TypeChatMessage && msg.Channel != "" && !peerConn.IsInChannel(msg.Channel) {
			fmt.Printf("[DEBUG] Peer %s não está no canal %s, ignorando\n", addr, msg.Channel)
			continue
		}

		// Envia a mensagem
		if err := peerConn.SendMessage(msg); err != nil {
			fmt.Printf("[ERRO] Falha ao enviar mensagem para peer %s: %v\n", addr, err)
			failCount++
		} else {
			fmt.Printf("[DEBUG] Mensagem enviada com sucesso para peer %s\n", addr)
			sentCount++
		}
	}

	fmt.Printf("[INFO] Mensagem enviada para %d peers (falhas: %d)\n", sentCount, failCount)
}
