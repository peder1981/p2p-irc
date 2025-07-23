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
// Agora utiliza IRCRouter para comandos e CommandHistory para histórico navegável.
// Para adicionar comandos customizados, registre-os no campo router do TerminalUI.
type TerminalUI struct {
	ChannelManager

	activeChannel string
	channels      []string
	debugMode     bool
	inputHandler  func(string)
	mu            sync.RWMutex
	quit          chan struct{}
	chatMessages  []string

	// Modularização
	router         *IRCRouter
	commandHistory *CommandHistory
}

// NewTerminalUI cria uma nova interface de terminal
func NewTerminalUI() *TerminalUI {
	ui := &TerminalUI{
		activeChannel: "#general",
		channels:      []string{"#general"},
		debugMode:     false,
		router:        NewIRCRouter(),
		commandHistory: NewCommandHistory(50),
	}
	// Registrar comandos básicos no roteador
	ui.router.Register("help", func(cmd *IRCCommand, _ *GUI, _ func(string)) bool {
		fmt.Println("=== Ajuda do P2P-IRC ===")
		fmt.Println("Comandos disponíveis:")
		fmt.Println("  /help - Exibe esta ajuda")
		fmt.Println("  /channels - Lista canais disponíveis")
		fmt.Println("  /quit - Encerra a aplicação")
		fmt.Println("=========================")
		return true
	})
	ui.router.Register("channels", func(cmd *IRCCommand, _ *GUI, _ func(string)) bool {
		ui.mu.RLock()
		defer ui.mu.RUnlock()
		fmt.Println("[INFO] Canais disponíveis:")
		for _, channel := range ui.channels {
			if channel == ui.activeChannel {
				fmt.Printf("  > %s (ativo)\n", channel)
			} else {
				fmt.Printf("  - %s\n", channel)
			}
		}
		return true
	})
	ui.router.Register("quit", func(cmd *IRCCommand, _ *GUI, _ func(string)) bool {
		fmt.Println("Encerrando aplicação...")
		os.Exit(0)
		return true
	})
	return ui
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
		if input == "" {
			continue
		}
		// Histórico de comandos
		ui.commandHistory.Add(input)
		if input == "/prev" {
			cmd := ui.commandHistory.Prev()
			if cmd != "" {
				fmt.Printf("[HISTÓRICO] %s\n", cmd)
			}
			continue
		} else if input == "/next" {
			cmd := ui.commandHistory.Next()
			if cmd != "" {
				fmt.Printf("[HISTÓRICO] %s\n", cmd)
			}
			continue
		}
		// Despacha comando via roteador
		if strings.HasPrefix(input, "/") {
			if ui.router.Dispatch(input, nil, func(fallback string) {
				if ui.inputHandler != nil {
					ui.inputHandler(fallback)
				}
			}) {
				continue
			}
		}
		// Se há um handler definido, chama-o
		if ui.inputHandler != nil {
			ui.inputHandler(input)
		}
	}
	return scanner.Err()
}
