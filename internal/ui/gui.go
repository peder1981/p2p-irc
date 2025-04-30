// Package ui implementa as interfaces de usuário para o P2P-IRC
// Copyright (c) 2025 Peder Munksgaard
// Licenciado sob MIT License

package ui

import (
	"fmt"
	"strings"
	"sync"
	"time"
	
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// GUI representa a interface gráfica do cliente P2P-IRC
type GUI struct {
	// Aplicação Fyne
	app        fyne.App
	mainWindow fyne.Window
	
	// Componentes da interface
	channelList  *widget.List
	peerList     *widget.List
	chatOutput   *widget.TextGrid
	inputField   *widget.Entry
	statusLabel  *widget.Label
	
	// Dados
	channels      []string
	peers         []string
	activeChannel string
	debugMode     bool
	chatContent   map[string][]string
	
	// Sincronização
	mu            sync.RWMutex
	
	// Callbacks
	inputHandler  func(string)
}

// NewGUI cria uma nova interface gráfica
func NewGUI() *GUI {
	// Cria a aplicação Fyne
	a := app.New()
	
	// Define o ícone da aplicação usando um caminho relativo
	iconPath := "/home/peder/p2p-irc/internal/ui/static/icon.svg"
	
	// Carrega o ícone de forma segura
	resource, err := fyne.LoadResourceFromPath(iconPath)
	if err == nil {
		a.SetIcon(resource)
	} else {
		fmt.Printf("[ERRO] Falha ao carregar ícone: %v\n", err)
	}
	
	// Cria a janela principal
	w := a.NewWindow("P2P-IRC")
	w.Resize(fyne.NewSize(800, 600))
	
	// Cria a interface
	gui := &GUI{
		app:          a,
		mainWindow:   w,
		channels:     []string{},
		peers:        []string{},
		debugMode:    false,
		chatContent:  make(map[string][]string),
	}
	
	// Inicializa os componentes
	gui.initComponents()
	
	// Configura o layout
	gui.setupLayout()
	
	// Adiciona o canal padrão após a inicialização completa da interface
	gui.AddChannel("#general")
	gui.activeChannel = "#general"
	
	return gui
}

// initComponents inicializa os componentes da interface
func (g *GUI) initComponents() {
	// Lista de canais
	g.channelList = widget.NewList(
		func() int { return len(g.channels) },
		func() fyne.CanvasObject { return widget.NewLabel("Template") },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			label := obj.(*widget.Label)
			channel := g.channels[id]
			if channel == g.activeChannel {
				label.SetText("▶ " + channel)
			} else {
				label.SetText("  " + channel)
			}
		},
	)
	g.channelList.OnSelected = func(id widget.ListItemID) {
		g.SetActiveChannel(g.channels[id])
		g.channelList.Refresh()
	}
	
	// Lista de peers
	g.peerList = widget.NewList(
		func() int { return len(g.peers) },
		func() fyne.CanvasObject { return widget.NewLabel("Template") },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			label := obj.(*widget.Label)
			label.SetText(g.peers[id])
		},
	)
	
	// Área de chat
	g.chatOutput = widget.NewTextGrid()
	g.chatOutput.SetText("")
	
	// Campo de entrada
	g.inputField = widget.NewEntry()
	g.inputField.SetPlaceHolder("Digite uma mensagem ou comando...")
	g.inputField.OnSubmitted = func(text string) {
		if text == "" {
			return
		}
		
		// Limpa o campo de entrada
		g.inputField.SetText("")
		
		// Adiciona log para depuração
		if g.debugMode {
			g.AddLogMessage(fmt.Sprintf("Processando entrada: %s", text))
		}
		
		// Captura o handler em uma variável local para evitar condições de corrida
		var handler func(string)
		g.mu.RLock()
		handler = g.inputHandler
		g.mu.RUnlock()
		
		// Se há um handler definido, chama-o
		if handler != nil {
			// Executa o handler em uma goroutine para evitar bloqueios na UI
			go func(input string) {
				handler(input)
			}(text)
		}
	}
	
	// Barra de status
	g.statusLabel = widget.NewLabel("P2P-IRC iniciado")
}

// setupLayout configura o layout da interface
func (g *GUI) setupLayout() {
	// Painel esquerdo: canais e peers
	channelsContainer := container.NewBorder(
		widget.NewLabel("Canais"), nil, nil, nil,
		container.NewScroll(g.channelList),
	)
	
	peersContainer := container.NewBorder(
		widget.NewLabel("Peers"), nil, nil, nil,
		container.NewScroll(g.peerList),
	)
	
	leftPanel := container.NewVSplit(
		channelsContainer,
		peersContainer,
	)
	leftPanel.SetOffset(0.7) // 70% para canais, 30% para peers
	
	// Painel direito: chat e entrada
	chatContainer := container.NewBorder(
		widget.NewLabel(fmt.Sprintf("Chat (%s)", g.activeChannel)), nil, nil, nil,
		container.NewScroll(g.chatOutput),
	)
	
	inputContainer := container.NewBorder(
		nil, nil, nil, nil,
		g.inputField,
	)
	
	rightPanel := container.NewVSplit(
		chatContainer,
		inputContainer,
	)
	rightPanel.SetOffset(0.9) // 90% para chat, 10% para entrada
	
	// Layout principal
	mainSplit := container.NewHSplit(
		leftPanel,
		rightPanel,
	)
	mainSplit.SetOffset(0.2) // 20% para o painel esquerdo, 80% para o painel direito
	
	// Layout final com barra de status
	mainLayout := container.NewBorder(
		nil, g.statusLabel, nil, nil,
		mainSplit,
	)
	
	// Define o conteúdo da janela
	g.mainWindow.SetContent(mainLayout)
	
	// Adiciona menu
	g.setupMenu()
}

// setupMenu configura o menu da aplicação
func (g *GUI) setupMenu() {
	// Menu de arquivo
	fileMenu := fyne.NewMenu("Arquivo",
		fyne.NewMenuItem("Sair", func() {
			g.mainWindow.Close()
		}),
	)
	
	// Menu de ajuda
	helpMenu := fyne.NewMenu("Ajuda",
		fyne.NewMenuItem("Comandos", func() {
			g.showHelp()
		}),
		fyne.NewMenuItem("Sobre", func() {
			dialog.ShowInformation("Sobre P2P-IRC", 
				"P2P-IRC é um cliente de chat IRC descentralizado.\n"+
				"Desenvolvido como projeto experimental.", 
				g.mainWindow)
		}),
	)
	
	// Barra de menu principal
	mainMenu := fyne.NewMainMenu(
		fileMenu,
		helpMenu,
	)
	
	g.mainWindow.SetMainMenu(mainMenu)
}

// showHelp exibe uma tela de ajuda
func (g *GUI) showHelp() {
	helpContent := `Comandos disponíveis:
/nick <novo> - Define seu nickname
/join <#canal> - Entra em um canal
/part [#canal] - Sai de um canal (usa o atual se não especificado)
/msg <destino> <mensagem> - Envia mensagem privada
/who - Lista usuários conectados
/peers - Lista peers conectados
/quit - Encerra a aplicação
/help - Exibe esta ajuda`

	dialog.ShowInformation("Comandos P2P-IRC", helpContent, g.mainWindow)
}

// SetInputHandler define a função chamada ao enviar mensagem
func (g *GUI) SetInputHandler(handler func(string)) {
	g.mu.Lock()
	g.inputHandler = handler
	g.mu.Unlock()
}

// AddMessage adiciona uma mensagem ao chat
func (g *GUI) AddMessage(msg string) {
	// Adiciona timestamp se não houver
	if !strings.Contains(msg, "[2") { // Verifica se já tem timestamp
		timestamp := time.Now().Format("[2006-01-02 15:04:05]")
		msg = fmt.Sprintf("%s %s", timestamp, msg)
	}
	
	// Adiciona a mensagem ao canal ativo de forma segura
	g.mu.Lock()
	if _, exists := g.chatContent[g.activeChannel]; !exists {
		g.chatContent[g.activeChannel] = []string{}
	}
	g.chatContent[g.activeChannel] = append(g.chatContent[g.activeChannel], msg)
	
	// Copia o conteúdo para atualização segura
	content := make([]string, len(g.chatContent[g.activeChannel]))
	copy(content, g.chatContent[g.activeChannel])
	g.mu.Unlock()
	
	// Atualiza a visualização de forma segura
	text := strings.Join(content, "\n")
	g.chatOutput.SetText(text)
	g.mainWindow.Canvas().Refresh(g.chatOutput)
}

// AddMessageToChannel adiciona uma mensagem a um canal específico
func (g *GUI) AddMessageToChannel(channel, msg string) {
	// Adiciona timestamp se não houver
	if !strings.Contains(msg, "[2") { // Verifica se já tem timestamp
		timestamp := time.Now().Format("[2006-01-02 15:04:05]")
		msg = fmt.Sprintf("%s %s", timestamp, msg)
	}
	
	// Adiciona a mensagem ao canal de forma segura
	g.mu.Lock()
	if _, exists := g.chatContent[channel]; !exists {
		g.chatContent[channel] = []string{}
	}
	g.chatContent[channel] = append(g.chatContent[channel], msg)
	isActiveChannel := (channel == g.activeChannel)
	
	// Se for o canal ativo, prepara o conteúdo para atualização
	var content []string
	if isActiveChannel {
		content = make([]string, len(g.chatContent[channel]))
		copy(content, g.chatContent[channel])
	}
	g.mu.Unlock()
	
	// Se for o canal ativo, atualiza a visualização
	if isActiveChannel {
		text := strings.Join(content, "\n")
		g.chatOutput.SetText(text)
		g.mainWindow.Canvas().Refresh(g.chatOutput)
	}
}

// AddLogMessage adiciona uma mensagem de log
func (g *GUI) AddLogMessage(msg string) {
	// Se o modo de depuração estiver ativado, adiciona ao chat
	g.mu.RLock()
	debugMode := g.debugMode
	g.mu.RUnlock()
	
	if debugMode {
		g.AddMessage(fmt.Sprintf("[DEBUG] %s", msg))
	}
}

// SetDebugMode ativa ou desativa o modo de depuração
func (g *GUI) SetDebugMode(enabled bool) {
	g.mu.Lock()
	g.debugMode = enabled
	g.mu.Unlock()
	
	if enabled {
		g.statusLabel.SetText("Modo de depuração ativado")
	} else {
		g.statusLabel.SetText("Modo de depuração desativado")
	}
}

// SetActiveChannel define o canal ativo
func (g *GUI) SetActiveChannel(channel string) {
	g.mu.Lock()
	g.activeChannel = channel
	
	// Prepara o conteúdo para atualização
	var content []string
	if messages, exists := g.chatContent[channel]; exists {
		content = make([]string, len(messages))
		copy(content, messages)
	} else {
		content = []string{}
	}
	g.mu.Unlock()
	
	// Atualiza a interface
	g.updateChannelList()
	text := strings.Join(content, "\n")
	g.chatOutput.SetText(text)
	g.mainWindow.Canvas().Refresh(g.chatOutput)
	g.statusLabel.SetText(fmt.Sprintf("Canal ativo: %s", channel))
}

// GetActiveChannel retorna o canal ativo
func (g *GUI) GetActiveChannel() string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	
	return g.activeChannel
}

// SetChannels atualiza a lista de canais
func (g *GUI) SetChannels(channels []string) {
	g.mu.Lock()
	g.channels = channels
	g.mu.Unlock()
	
	g.updateChannelList()
}

// AddChannel adiciona um canal à lista
func (g *GUI) AddChannel(channel string) {
	g.mu.Lock()
	
	// Verifica se o canal já existe
	channelExists := false
	for _, ch := range g.channels {
		if ch == channel {
			channelExists = true
			break
		}
	}
	
	// Adiciona o canal se não existir
	if !channelExists {
		g.channels = append(g.channels, channel)
		
		// Cria o conteúdo do canal se não existir
		if _, exists := g.chatContent[channel]; !exists {
			g.chatContent[channel] = []string{}
		}
	}
	g.mu.Unlock()
	
	// Atualiza a interface
	g.updateChannelList()
}

// RemoveChannel remove um canal da lista de canais
func (g *GUI) RemoveChannel(channel string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	
	// Remove o canal da lista de canais
	newChannels := make([]string, 0, len(g.channels))
	for _, ch := range g.channels {
		if ch != channel {
			newChannels = append(newChannels, ch)
		}
	}
	g.channels = newChannels
	
	// Remove o conteúdo do canal
	delete(g.chatContent, channel)
	
	// Se o canal ativo foi removido, define o primeiro canal como ativo
	if g.activeChannel == channel {
		if len(g.channels) > 0 {
			g.activeChannel = g.channels[0]
		} else {
			g.activeChannel = ""
		}
	}
	
	// Atualiza a lista de canais
	g.updateChannelList()
}

// GetChannelList retorna a lista de canais
func (g *GUI) GetChannelList() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	
	// Cria uma cópia da lista de canais
	channels := make([]string, len(g.channels))
	copy(channels, g.channels)
	
	return channels
}

// SetPeers atualiza a lista de peers
func (g *GUI) SetPeers(peers []string) {
	g.mu.Lock()
	g.peers = peers
	g.mu.Unlock()
	
	// Atualiza a interface de forma segura
	g.mainWindow.Canvas().Refresh(g.peerList)
}

// ClearLogs limpa os logs (método vazio para compatibilidade)
func (g *GUI) ClearLogs() {
	// Não faz nada, apenas para compatibilidade com a interface
}

// Run inicia a aplicação GUI
func (g *GUI) Run() error {
	// Exibe a janela principal
	g.mainWindow.ShowAndRun()
	
	return nil
}

// updateChatOutput atualiza a exibição do chat
func (g *GUI) updateChatOutput() {
	// Prepara o conteúdo para atualização
	g.mu.RLock()
	content := make([]string, len(g.chatContent[g.activeChannel]))
	copy(content, g.chatContent[g.activeChannel])
	g.mu.RUnlock()
	
	// Atualiza a exibição do chat usando a thread principal
	text := strings.Join(content, "\n")
	g.chatOutput.SetText(text)
	g.mainWindow.Canvas().Refresh(g.chatOutput)
}

// updateChannelList atualiza a lista de canais na interface
func (g *GUI) updateChannelList() {
	// Verifica se a lista de canais foi inicializada
	if g.channelList == nil {
		return
	}
	
	// Atualiza a lista de canais de forma segura
	g.mainWindow.Canvas().Refresh(g.channelList)
}

// updatePeerList atualiza a lista de peers na interface
func (g *GUI) updatePeerList() {
	// Verifica se a lista de peers foi inicializada
	if g.peerList == nil {
		return
	}
	
	// Atualiza a lista de peers de forma segura
	g.mainWindow.Canvas().Refresh(g.peerList)
}
