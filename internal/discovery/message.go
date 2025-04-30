// Package discovery implementa o serviço de descoberta de peers
package discovery

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"
)

// Tipos de mensagens
const (
	TypeChatMessage = "CHAT"    // Mensagem de chat
	TypeJoinChannel = "JOIN"    // Entrar em um canal
	TypePartChannel = "PART"    // Sair de um canal
	TypePing        = "PING"    // Ping para verificar se o peer está ativo
	TypePong        = "PONG"    // Resposta ao ping
)

// Message representa uma mensagem trocada entre peers
type Message struct {
	Type      string    `json:"type"`      // Tipo da mensagem
	Sender    string    `json:"sender"`    // ID do remetente
	Channel   string    `json:"channel"`   // Canal da mensagem
	Content   string    `json:"content"`   // Conteúdo da mensagem
	Timestamp time.Time `json:"timestamp"` // Timestamp da mensagem
}

// PeerConnection representa uma conexão com um peer
type PeerConnection struct {
	Conn       net.Conn      // Conexão TCP
	LastPing   time.Time     // Último ping recebido
	Channels   map[string]bool // Canais em que o peer está
	channelsMu sync.RWMutex  // Mutex para proteger o mapa de canais
}

// NewPeerConnection cria uma nova conexão de peer
func NewPeerConnection(conn net.Conn) *PeerConnection {
	return &PeerConnection{
		Conn:     conn,
		LastPing: time.Now(),
		Channels: make(map[string]bool),
	}
}

// SendMessage envia uma mensagem para o peer
func (p *PeerConnection) SendMessage(msg Message) error {
	// Serializa a mensagem
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("erro ao serializar mensagem: %w", err)
	}
	
	// Adiciona um delimitador
	data = append(data, '\n')
	
	// Define um timeout para o envio
	p.Conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	
	// Envia a mensagem
	_, err = p.Conn.Write(data)
	if err != nil {
		return fmt.Errorf("erro ao enviar mensagem: %w", err)
	}
	
	return nil
}

// AddChannel adiciona um canal à lista de canais do peer
func (p *PeerConnection) AddChannel(channel string) {
	p.channelsMu.Lock()
	defer p.channelsMu.Unlock()
	p.Channels[channel] = true
	fmt.Printf("[DEBUG] Peer adicionado ao canal %s\n", channel)
}

// RemoveChannel remove um canal da lista de canais do peer
func (p *PeerConnection) RemoveChannel(channel string) {
	p.channelsMu.Lock()
	defer p.channelsMu.Unlock()
	delete(p.Channels, channel)
	fmt.Printf("[DEBUG] Peer removido do canal %s\n", channel)
}

// IsInChannel verifica se o peer está em um canal
func (p *PeerConnection) IsInChannel(channel string) bool {
	p.channelsMu.RLock()
	defer p.channelsMu.RUnlock()
	return p.Channels[channel]
}
