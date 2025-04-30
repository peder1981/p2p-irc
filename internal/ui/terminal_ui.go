package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// TerminalUI é uma implementação simples de interface de terminal
type TerminalUI struct {
	activeChannel string
	channels      []string
	debugMode     bool
	inputHandler  func(string)
	mu            sync.RWMutex
}

// NewTerminalUI cria uma nova interface de terminal
func NewTerminalUI() *TerminalUI {
	return &TerminalUI{
		activeChannel: "#general",
		channels:      []string{},
		debugMode:     false,
	}
}

// SetInputHandler define a função de callback para entrada do usuário
func (ui *TerminalUI) SetInputHandler(handler func(string)) {
	ui.inputHandler = handler
}

// AddMessage adiciona uma mensagem ao chat
func (ui *TerminalUI) AddMessage(msg string) {
	if !strings.Contains(msg, "[2") {
		timestamp := time.Now().Format("[2006-01-02 15:04:05]")
		msg = fmt.Sprintf("%s %s", timestamp, msg)
	}
	fmt.Println(msg)
}

// AddMessageToChannel adiciona uma mensagem a um canal específico
func (ui *TerminalUI) AddMessageToChannel(channel, msg string) {
	if !strings.Contains(msg, "[2") {
		timestamp := time.Now().Format("[2006-01-02 15:04:05]")
		msg = fmt.Sprintf("%s %s", timestamp, msg)
	}
	fmt.Printf("[%s] %s\n", channel, msg)
}

// AddLogMessage adiciona uma mensagem de log
func (ui *TerminalUI) AddLogMessage(msg string) {
	if ui.debugMode {
		timestamp := time.Now().Format("[2006-01-02 15:04:05]")
		fmt.Printf("[DEBUG] %s %s\n", timestamp, msg)
	}
}

// ClearLogs limpa os logs (não faz nada no terminal)
func (ui *TerminalUI) ClearLogs() {
	// Não faz nada no terminal
}

// SetDebugMode ativa ou desativa o modo de depuração
func (ui *TerminalUI) SetDebugMode(enabled bool) {
	ui.mu.Lock()
	defer ui.mu.Unlock()
	
	ui.debugMode = enabled
	
	if enabled {
		fmt.Println("Modo de depuração ativado")
	}
}

// SetActiveChannel define o canal ativo
func (ui *TerminalUI) SetActiveChannel(channel string) {
	ui.mu.Lock()
	defer ui.mu.Unlock()
	
	ui.activeChannel = channel
	fmt.Printf("Canal ativo: %s\n", channel)
}

// GetActiveChannel retorna o canal ativo
func (ui *TerminalUI) GetActiveChannel() string {
	ui.mu.RLock()
	defer ui.mu.RUnlock()
	
	return ui.activeChannel
}

// SetChannels define a lista de canais
func (ui *TerminalUI) SetChannels(channels []string) {
	ui.mu.Lock()
	defer ui.mu.Unlock()
	
	ui.channels = channels
}

// AddChannel adiciona um canal à lista
func (ui *TerminalUI) AddChannel(channel string) {
	ui.mu.Lock()
	defer ui.mu.Unlock()
	
	// Verifica se o canal já existe
	for _, ch := range ui.channels {
		if ch == channel {
			return
		}
	}
	
	ui.channels = append(ui.channels, channel)
}

// SetPeers atualiza a lista de peers (não faz nada no terminal)
func (ui *TerminalUI) SetPeers(peers []string) {
	// Não faz nada no terminal
}

// Run inicia a interface de terminal
func (ui *TerminalUI) Run() error {
	fmt.Println("P2P-IRC Terminal iniciado. Digite /help para ver os comandos disponíveis.")
	
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		input := scanner.Text()
		
		if ui.inputHandler != nil {
			ui.inputHandler(input)
		}
	}
	
	return scanner.Err()
}
