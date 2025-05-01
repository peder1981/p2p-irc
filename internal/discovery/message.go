// Package discovery implementa o serviço de descoberta de peers
package discovery

import (
	"time"
)

// Tipos de mensagens
const (
	TypeChatMessage = "PRIVMSG"    // Mensagem de chat
	TypeJoinChannel = "JOIN"    // Entrar em um canal
	TypePartChannel = "PART"    // Sair de um canal
	TypePing        = "PING"    // Ping para verificar se o peer está ativo
	TypePong        = "PONG"    // Resposta ao ping
	TypeIdentify    = "IDENTIFY" // Identificação do peer
)

// Message representa uma mensagem trocada entre peers
type Message struct {
	Type      string    `json:"type"`      // Tipo da mensagem
	Sender    string    `json:"sender"`    // ID do remetente
	Channel   string    `json:"channel"`   // Canal da mensagem
	Content   string    `json:"content"`   // Conteúdo da mensagem
	Timestamp time.Time `json:"timestamp"` // Timestamp da mensagem
}
