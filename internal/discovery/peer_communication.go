package discovery

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"
)

// Cache para armazenar mensagens já processadas e evitar loops
var (
	processedMessages   = make(map[string]bool)
	processedMessagesMu sync.RWMutex
)

// IsMessageProcessed verifica se uma mensagem já foi processada
func IsMessageProcessed(messageID string) bool {
	processedMessagesMu.RLock()
	defer processedMessagesMu.RUnlock()
	return processedMessages[messageID]
}

// MarkMessageAsProcessed marca uma mensagem como processada
func MarkMessageAsProcessed(messageID string) {
	processedMessagesMu.Lock()
	processedMessages[messageID] = true
	processedMessagesMu.Unlock()
}

// SendChatMessageToChannel envia uma mensagem de chat para um canal específico
// Esta função garante que a mensagem seja enviada para todos os peers no canal
func (d *Discovery) SendChatMessageToChannel(channel, content, sender string) {
	// Verifica se estamos no canal
	if !d.IsInChannel(channel) {
		fmt.Printf("[ERRO] Tentativa de enviar mensagem para canal %s, mas não estamos neste canal\n", channel)
		return
	}

	// Gera um ID único para a mensagem
	messageID := generateMessageID(sender, channel, content)

	// Verifica se a mensagem já foi processada
	if IsMessageProcessed(messageID) {
		fmt.Printf("[DEBUG] Mensagem %s já foi processada, ignorando\n", messageID)
		return
	}

	// Cria a mensagem
	msg := Message{
		Type:      "chat", // Usando string constante para garantir consistência
		Sender:    sender,
		Channel:   channel,
		Content:   content,
		Timestamp: time.Now(),
		MessageID: messageID,
	}

	// Adiciona a mensagem ao cache de processadas para evitar loops
	MarkMessageAsProcessed(messageID)

	fmt.Printf("[DEBUG] Enviando mensagem para canal %s: %s (de %s)\n", channel, content, sender)

	// Envia para todos os peers que estão no canal
	d.broadcastToChannel(msg, channel)
}

// broadcastToChannel envia uma mensagem para todos os peers em um canal específico
func (d *Discovery) broadcastToChannel(msg Message, channel string) {
	d.connMu.RLock()
	defer d.connMu.RUnlock()

	fmt.Printf("[DEBUG] Broadcast de mensagem para canal %s, %d peers conectados\n", 
		channel, len(d.connections))

	// Conta quantos peers receberam a mensagem
	sentCount := 0

	// Lista de peers no canal para debug
	var peersInChannel []string

	for addr, peerConn := range d.connections {
		// Verifica se o peer está no canal da mensagem
		isInChannel := peerConn.IsInChannel(channel)
		if isInChannel {
			peersInChannel = append(peersInChannel, addr)
		} else {
			fmt.Printf("[DEBUG] Peer %s não está no canal %s, ignorando\n", addr, channel)
			continue
		}

		// Envia a mensagem
		if err := peerConn.SendMessage(msg); err != nil {
			fmt.Printf("[ERRO] Falha ao enviar mensagem para peer %s: %v\n", addr, err)
		} else {
			fmt.Printf("[DEBUG] Mensagem enviada com sucesso para peer %s\n", addr)
			sentCount++
		}
	}

	fmt.Printf("[INFO] Mensagem enviada para %d peers no canal %s: %v\n", sentCount, channel, peersInChannel)
}

// SyncChannels sincroniza os canais com todos os peers conectados
// Isso garante que todos os peers saibam em quais canais estamos
func (d *Discovery) SyncChannels() {
	d.channelMu.RLock()
	channels := make([]string, 0, len(d.channels))
	for channel := range d.channels {
		channels = append(channels, channel)
	}
	d.channelMu.RUnlock()

	fmt.Printf("[INFO] Sincronizando %d canais com todos os peers\n", len(channels))

	// Envia mensagens JOIN para todos os canais
	for _, channel := range channels {
		joinMsg := Message{
			Type:      "join", // Usando string constante para garantir consistência
			Sender:    d.instanceID,
			Channel:   channel,
			Timestamp: time.Now(),
		}

		// Envia para todos os peers
		d.broadcastToChannel(joinMsg, channel)
	}
}

// GetPeersInChannel retorna todos os peers que estão em um canal específico
func (d *Discovery) GetPeersInChannel(channel string) []string {
	d.connMu.RLock()
	defer d.connMu.RUnlock()

	peers := make([]string, 0)
	for addr, peerConn := range d.connections {
		if peerConn.IsInChannel(channel) {
			peers = append(peers, addr)
		}
	}

	return peers
}

// generateMessageID gera um ID único para uma mensagem
func generateMessageID(sender, channel, content string) string {
	timestamp := time.Now().Format(time.RFC3339Nano)
	data := fmt.Sprintf("%s:%s:%s:%s", sender, channel, content, timestamp)
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash)
}
