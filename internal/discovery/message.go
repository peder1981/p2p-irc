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
	MessageID string    `json:"message_id"` // Identificador único da mensagem para evitar loops
}

// PeerConnection representa uma conexão com um peer
type PeerConnection struct {
	Conn       net.Conn      // Conexão TCP
	LastPing   time.Time     // Último ping recebido
	InstanceID string        // ID único da instância do peer
	Channels   map[string]bool // Canais em que o peer está
	channelsMu sync.RWMutex  // Mutex para proteger o mapa de canais
}

// NewPeerConnection cria uma nova conexão de peer
func NewPeerConnection(conn net.Conn, instanceID string) *PeerConnection {
	return &PeerConnection{
		Conn:       conn,
		InstanceID: instanceID,
		LastPing:   time.Now(),
		Channels:   make(map[string]bool),
	}
}

// SendMessage envia uma mensagem para um peer
func (pc *PeerConnection) SendMessage(msg Message) error {
	// Serializa a mensagem
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("erro ao serializar mensagem: %v", err)
	}
	
	// Adiciona um delimitador
	data = append(data, '\n')
	
	// Envia a mensagem
	_, err = pc.Conn.Write(data)
	if err != nil {
		return fmt.Errorf("erro ao enviar mensagem: %v", err)
	}
	
	// Força o flush da conexão para garantir que a mensagem seja enviada imediatamente
	if tcpConn, ok := pc.Conn.(*net.TCPConn); ok {
		if err := tcpConn.SetNoDelay(true); err != nil {
			return fmt.Errorf("erro ao configurar NoDelay: %v", err)
		}
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

// GetChannels retorna a lista de canais do peer
func (p *PeerConnection) GetChannels() []string {
	p.channelsMu.RLock()
	defer p.channelsMu.RUnlock()
	
	channels := make([]string, 0, len(p.Channels))
	for channel := range p.Channels {
		channels = append(channels, channel)
	}
	
	return channels
}

// UpdateLastPing atualiza o timestamp do último ping
func (p *PeerConnection) UpdateLastPing() {
	p.LastPing = time.Now()
}
