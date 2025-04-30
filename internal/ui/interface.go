// Package ui implementa as interfaces de usuário para o P2P-IRC
// Copyright (c) 2025 Peder Munksgaard
// Licenciado sob MIT License

package ui

// Interface define a interface comum para todas as implementações de UI
type Interface interface {
	// Métodos para exibir mensagens
	AddMessage(msg string)
	AddMessageToChannel(channel, msg string)
	AddLogMessage(msg string)
	
	// Métodos para gerenciar canais
	SetChannels(channels []string)
	AddChannel(channel string)
	RemoveChannel(channel string)
	SetActiveChannel(channel string)
	GetActiveChannel() string
	GetChannelList() []string
	
	// Métodos para gerenciar peers
	SetPeers(peers []string)
	
	// Configurações
	SetDebugMode(enabled bool)
	SetInputHandler(handler func(string))
	
	// Execução
	Run() error
}
