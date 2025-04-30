// Package ui implementa as interfaces de usuário para o P2P-IRC
// Copyright (c) 2025 Peder Munksgaard
// Licenciado sob MIT License

package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// BasicUI representa uma interface de terminal simples sem dependências externas
type BasicUI struct {
	chatMessages  []string
	channels      []string
	peers         []string
	activeChannel string
	debugMode     bool
	inputHandler  func(string)
	
	// Sincronização
	mu            sync.RWMutex
	
	// Controle de execução
	running       bool
	done          chan struct{}
}

// NewBasicUI cria uma nova interface de terminal simples
func NewBasicUI() *BasicUI {
	return &BasicUI{
		chatMessages:  make([]string, 0),
		channels:      []string{"#general"},
		peers:         []string{},
		activeChannel: "#general",
		debugMode:     false,
		done:          make(chan struct{}),
	}
}

// SetInputHandler define a função chamada ao enviar mensagem
func (ui *BasicUI) SetInputHandler(handler func(string)) {
	ui.inputHandler = handler
}

// AddMessage adiciona uma mensagem ao chat
func (ui *BasicUI) AddMessage(msg string) {
	ui.mu.Lock()
	defer ui.mu.Unlock()
	
	// Adiciona timestamp se não houver
	if !strings.Contains(msg, "[2") { // Verifica se já tem timestamp
		timestamp := time.Now().Format("[2006-01-02 15:04:05]")
		msg = fmt.Sprintf("%s %s", timestamp, msg)
	}
	
	// Adiciona a mensagem à lista
	ui.chatMessages = append(ui.chatMessages, msg)
	
	// Exibe a mensagem imediatamente
	fmt.Println(msg)
}

// AddMessageToChannel adiciona uma mensagem a um canal específico
func (ui *BasicUI) AddMessageToChannel(channel, msg string) {
	// Nesta implementação simplificada, apenas adiciona a mensagem se for o canal ativo
	if channel == ui.activeChannel {
		ui.AddMessage(msg)
	}
}

// AddLogMessage adiciona uma mensagem de log
func (ui *BasicUI) AddLogMessage(msg string) {
	// Se o modo de depuração estiver ativado, adiciona ao chat
	if ui.debugMode {
		ui.AddMessage(fmt.Sprintf("[DEBUG] %s", msg))
	}
}

// ClearLogs limpa os logs (método vazio para compatibilidade)
func (ui *BasicUI) ClearLogs() {
	// Não faz nada, apenas para compatibilidade
}

// SetDebugMode ativa ou desativa o modo de depuração
func (ui *BasicUI) SetDebugMode(enabled bool) {
	ui.mu.Lock()
	defer ui.mu.Unlock()
	
	ui.debugMode = enabled
	
	if enabled {
		fmt.Println("[INFO] Modo de depuração ativado")
	} else {
		fmt.Println("[INFO] Modo de depuração desativado")
	}
}

// SetActiveChannel define o canal ativo
func (ui *BasicUI) SetActiveChannel(channel string) {
	ui.mu.Lock()
	defer ui.mu.Unlock()
	
	ui.activeChannel = channel
	fmt.Printf("[INFO] Canal ativo alterado para: %s\n", channel)
}

// GetActiveChannel retorna o canal ativo
func (ui *BasicUI) GetActiveChannel() string {
	ui.mu.RLock()
	defer ui.mu.RUnlock()
	
	return ui.activeChannel
}

// SetChannels atualiza a lista de canais
func (ui *BasicUI) SetChannels(channels []string) {
	ui.mu.Lock()
	defer ui.mu.Unlock()
	
	ui.channels = channels
	
	// Exibe a lista de canais
	fmt.Println("[INFO] Canais disponíveis:")
	for _, channel := range channels {
		if channel == ui.activeChannel {
			fmt.Printf("  > %s (ativo)\n", channel)
		} else {
			fmt.Printf("  - %s\n", channel)
		}
	}
}

// SetPeers atualiza a lista de peers
func (ui *BasicUI) SetPeers(peers []string) {
	ui.mu.Lock()
	defer ui.mu.Unlock()
	
	// Verifica se a lista mudou
	if len(peers) != len(ui.peers) {
		ui.peers = peers
		
		// Exibe a lista de peers
		fmt.Println("[INFO] Peers conectados:")
		for _, peer := range peers {
			fmt.Printf("  - %s\n", peer)
		}
	} else {
		// Verifica se algum peer mudou
		changed := false
		for i, peer := range peers {
			if i >= len(ui.peers) || peer != ui.peers[i] {
				changed = true
				break
			}
		}
		
		if changed {
			ui.peers = peers
			
			// Exibe a lista de peers
			fmt.Println("[INFO] Peers conectados:")
			for _, peer := range peers {
				fmt.Printf("  - %s\n", peer)
			}
		}
	}
}

// showHelp exibe a ajuda
func (ui *BasicUI) showHelp() {
	fmt.Println("\n=== Ajuda do P2P-IRC ===")
	fmt.Println("Comandos disponíveis:")
	fmt.Println("  /help - Exibe esta ajuda")
	fmt.Println("  /nick <apelido> - Altera seu apelido")
	fmt.Println("  /join <#canal> - Entra em um canal")
	fmt.Println("  /part [#canal] - Sai de um canal (usa o atual se não especificado)")
	fmt.Println("  /msg <destino> <mensagem> - Envia mensagem privada")
	fmt.Println("  /who - Lista usuários conectados")
	fmt.Println("  /peers - Lista peers conectados")
	fmt.Println("  /channels - Lista canais disponíveis")
	fmt.Println("  /quit - Encerra a aplicação")
	fmt.Println("=========================\n")
}

// Run inicia a interface do usuário
func (ui *BasicUI) Run() error {
	ui.running = true
	
	// Exibe mensagem de boas-vindas
	fmt.Println("=== P2P-IRC ===")
	fmt.Println("Digite /help para ver a lista de comandos disponíveis.")
	fmt.Println("Canal ativo: " + ui.activeChannel)
	fmt.Println("===============")
	
	// Inicia o loop de leitura de entrada
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		
		for ui.running {
			// Lê a entrada do usuário
			if scanner.Scan() {
				input := scanner.Text()
				
				// Processa comandos especiais da interface
				if input == "/help" {
					ui.showHelp()
					continue
				} else if input == "/channels" {
					ui.mu.RLock()
					fmt.Println("[INFO] Canais disponíveis:")
					for _, channel := range ui.channels {
						if channel == ui.activeChannel {
							fmt.Printf("  > %s (ativo)\n", channel)
						} else {
							fmt.Printf("  - %s\n", channel)
						}
					}
					ui.mu.RUnlock()
					continue
				} else if input == "/quit" {
					ui.running = false
					close(ui.done)
					return
				}
				
				// Se há um handler definido, chama-o
				if ui.inputHandler != nil {
					ui.inputHandler(input)
				}
			}
		}
	}()
	
	// Aguarda o sinal de encerramento
	<-ui.done
	
	return nil
}
