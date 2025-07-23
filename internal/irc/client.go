package irc

import (
	"fmt"
	"strings"
	"time"

	"github.com/peder1981/p2p-irc/internal/discovery"
	"github.com/peder1981/p2p-irc/internal/ui"
)

// Client gerencia o estado e a lógica do cliente IRC sobre a camada de descoberta P2P.
type Client struct {
	nickname         string
	activeChannel    string
	discoveryService *discovery.Discovery
	ui               ui.Interface
}

// NewClient cria um novo cliente IRC.
func NewClient(nickname string, discoveryService *discovery.Discovery, ui ui.Interface) *Client {
	return &Client{
		nickname:         nickname,
		discoveryService: discoveryService,
		ui:               ui,
	}
}

// HandleCommand processa a entrada do usuário, tratando comandos IRC e mensagens de chat.
func (c *Client) HandleCommand(input string) error {
	if len(input) == 0 {
		return nil
	}

	if input[0] != '/' {
		// Envia como mensagem de chat no canal ativo
		return c.sendChatMessage(input)
	}

	parts := strings.Split(input, " ")
	command := strings.ToLower(parts[0])

	switch command {
	case "/join":
		if len(parts) < 2 {
			return fmt.Errorf("uso: /join <#canal>")
		}
		return c.JoinChannel(parts[1])
	case "/part":
		if len(parts) < 2 {
			return fmt.Errorf("uso: /part <#canal>")
		}
		return c.PartChannel(parts[1])
	case "/nick":
		if len(parts) < 2 {
			return fmt.Errorf("uso: /nick <novo_nickname>")
		}
		c.nickname = parts[1]
		c.ui.AddMessage(fmt.Sprintf("Seu nickname foi alterado para %s", c.nickname))
		return nil
	case "/msg":
		if len(parts) < 3 {
			return fmt.Errorf("uso: /msg <#canal> <mensagem>")
		}
		channel := parts[1]
		message := strings.Join(parts[2:], " ")
		return c.sendMessageToChannel(channel, message)
	case "/help":
		c.ui.AddMessage("Comandos disponíveis: /join, /part, /nick, /msg, /help, /mesh")
		return nil
	default:
		return fmt.Errorf("comando desconhecido: %s", command)
	}
}

// JoinChannel entra em um canal, se inscreve para receber mensagens e notifica outros peers.
func (c *Client) JoinChannel(channel string) error {
	if !strings.HasPrefix(channel, "#") {
		return fmt.Errorf("nome de canal inválido, deve começar com #")
	}
	c.discoveryService.JoinChannel(channel)
	c.activeChannel = channel
	c.ui.AddMessage(fmt.Sprintf("Você entrou no canal %s", channel))
	c.ui.SetActiveChannel(channel)
	return nil
}

// PartChannel sai de um canal.
func (c *Client) PartChannel(channel string) error {
	c.discoveryService.PartChannel(channel)
	c.ui.AddMessage(fmt.Sprintf("Você saiu do canal %s", channel))
	if c.activeChannel == channel {
		c.activeChannel = ""
		c.ui.SetActiveChannel("")
	}
	return nil
}

// sendChatMessage envia uma mensagem para o canal ativo.
func (c *Client) sendChatMessage(message string) error {
	if c.activeChannel == "" {
		return fmt.Errorf("você não está em nenhum canal. Use /join <#canal>")
	}
	return c.sendMessageToChannel(c.activeChannel, message)
}

// sendMessageToChannel constrói e transmite uma mensagem de chat.
func (c *Client) sendMessageToChannel(channel, message string) error {
	msg := discovery.Message{
		Type:      discovery.TypeChatMessage,
		Sender:    c.nickname,
		Channel:   channel,
		Content:   message,
		Timestamp: time.Now(),
	}
	c.discoveryService.BroadcastMessage(msg)
	return nil
}

// GetNickname retorna o nickname atual do cliente.
func (c *Client) GetNickname() string {
	return c.nickname
}

// CurrentChannel retorna o canal ativo.
func (c *Client) CurrentChannel() string {
	return c.activeChannel
}
