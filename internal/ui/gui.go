//go:build !tui

// GUI representa a interface gráfica do cliente P2P-IRC.

package ui

import (
	"fmt"
	"log"
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
	
	chatOutput   *widget.TextGrid
	chatScroll   *container.Scroll
	inputField   *widget.Entry
	statusLabel  *widget.Label
	meshUI       *MeshUI
	
	// Dados
	channels      []string
	
	activeChannel string
	debugMode     bool
	chatContent   map[string][]string
	
	// Sincronização
	mu            sync.RWMutex
	
	// Callbacks
	inputHandler  func(string)
}

// NewGUI cria uma nova interface gráfica sem adicionar canais durante a construção
func NewGUI() *GUI {
	log.Printf("[DEBUG] Inicializando nova GUI")
	// Cria a aplicação Fyne
	a := app.New()
	// Carrega recurso do ícone
	var icon fyne.Resource
	if res, err := fyne.LoadResourceFromPath("assets/irc_p2p_icon.svg"); err != nil {
		log.Printf("[WARN] não foi possível carregar ícone: %v", err)
	} else {
		icon = res
		a.SetIcon(icon)
	}
	// Cria a janela principal
	w := a.NewWindow("P2P-IRC")
	// Define ícone na janela
	if icon != nil {
		w.SetIcon(icon)
	}
	w.Resize(fyne.NewSize(800, 600))
	
	// Cria a estrutura GUI com canal padrão
	gui := &GUI{
		app:           a,
		mainWindow:    w,
		channels:      []string{"#general"},
		
		activeChannel: "#general",
		debugMode:     false,
		chatContent:   make(map[string][]string),
	}
	
	// Inicializa componentes e layout
	gui.initComponents()
	gui.setupLayout()
	
	return gui
}

// initComponents inicializa os componentes da interface
func (g *GUI) initComponents() {
	log.Printf("[DEBUG] Inicializando componentes da GUI")
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
	if g.channelList == nil {
		log.Printf("[ERRO] Falha ao inicializar g.channelList")
	} else {
		log.Printf("[DEBUG] g.channelList inicializado com sucesso")
	}
	g.channelList.OnSelected = func(id widget.ListItemID) {
		g.SetActiveChannel(g.channels[id])
		g.channelList.Refresh()
	}
	

	
	// Área de chat
	g.chatOutput = widget.NewTextGrid()
	g.chatOutput.SetText("")
	g.chatScroll = container.NewScroll(g.chatOutput)
	
	// Campo de entrada
	g.inputField = widget.NewEntry()
	g.inputField.SetPlaceHolder("Digite uma mensagem ou comando...")
	g.inputField.OnSubmitted = func(text string) {
		if text == "" {
			return
		}
		
		// Limpa o campo de entrada
		g.inputField.SetText("")
		
		// Se há um handler definido, chama-o
		if g.inputHandler != nil {
			g.inputHandler(text)
		}
	}
	
	// Barra de status
	g.statusLabel = widget.NewLabel("P2P-IRC iniciado")

	// Inicializa a UI da Mesh
	g.meshUI = NewMeshUI(g)
}

// setupLayout configura o layout da interface
func (g *GUI) setupLayout() {
	// Painel esquerdo: canais e peers
	channelsContainer := container.NewBorder(
		widget.NewLabel("Canais"), nil, nil, nil,
		container.NewScroll(g.channelList),
	)
	
	leftPanel := channelsContainer

	
	// Painel direito: chat e entrada
	chatContainer := container.NewBorder(
		widget.NewLabel(fmt.Sprintf("Chat (%s)", g.activeChannel)), nil, nil, nil,
		g.chatScroll,
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

	// Cria as abas para alternar entre Chat e Mesh Status
	tabs := container.NewAppTabs(
		container.NewTabItem("Chat", rightPanel),
		container.NewTabItem("Mesh Status", g.meshUI.GetView()),
	)

	// Layout principal com painel esquerdo e abas à direita
	mainSplit := container.NewHSplit(
		leftPanel,
		tabs,
	)
	mainSplit.SetOffset(0.25) // 25% para o painel esquerdo

	// Layout final com a barra de status na parte inferior
	finalLayout := container.NewBorder(
		nil,          // Top
		g.statusLabel, // Bottom
		nil,          // Left
		nil,          // Right
		mainSplit,    // Center
	)

	g.mainWindow.SetContent(finalLayout)
	
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
/peers - Lista peers conectados na rede principal (mDNS)
/quit - Encerra a aplicação
/help - Exibe esta ajuda

Comandos Mesh:
/mesh status - Exibe o status da rede Bitchat
/mesh peers - Lista os peers conectados via Bitchat
/mesh send <peer_id> <mensagem> - Envia uma mensagem direta a um peer mesh`

	dialog.ShowInformation("Comandos P2P-IRC", helpContent, g.mainWindow)
}

// SetInputHandler define a função chamada ao enviar mensagem
func (g *GUI) SetInputHandler(handler func(string)) {
	g.inputHandler = handler
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
	if g.chatScroll != nil {
		g.chatScroll.ScrollToBottom()
	}
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
		if g.chatScroll != nil {
			g.chatScroll.ScrollToBottom()
		}
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
	normalizedChannel := strings.ToLower(channel)
	g.mu.Lock()
	g.activeChannel = normalizedChannel
	
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
	if g.chatScroll != nil {
		g.chatScroll.ScrollToBottom()
	}
	g.statusLabel.SetText(fmt.Sprintf("Canal ativo: %s", channel))
}

// GetActiveChannel retorna o canal ativo
func (g *GUI) GetActiveChannel() string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	
	return g.activeChannel
}

// SetChannels atualiza a lista de canais sem forçar refresh prematuro
func (g *GUI) SetChannels(channels []string) {
	g.mu.Lock()
	g.channels = channels
	g.mu.Unlock()
}

// AddChannel adiciona um canal à lista e atualiza a UI
func (g *GUI) AddChannel(channel string) {
	normalizedChannel := strings.ToLower(channel)
	log.Printf("[DEBUG] Adicionando canal: %s (normalizado: %s)", channel, normalizedChannel)
	g.mu.Lock()
	if !contains(g.channels, normalizedChannel) {
		g.channels = append(g.channels, normalizedChannel)
		log.Printf("[DEBUG] Canal adicionado: %s, tamanho da lista: %d", normalizedChannel, len(g.channels))
	}
	g.mu.Unlock()
	g.updateChannelList()
}

// RemoveChannel remove um canal da lista de canais
func (g *GUI) RemoveChannel(channel string) {
	normalizedChannel := strings.ToLower(channel)
	g.mu.Lock()
	defer g.mu.Unlock()

	// Remove o canal da lista de canais
	newChannels := make([]string, 0, len(g.channels))
	for _, ch := range g.channels {
		if ch != normalizedChannel {
			newChannels = append(newChannels, ch)
		}
	}
	g.channels = newChannels

	// Remove o conteúdo do canal
	delete(g.chatContent, normalizedChannel)

	// Se o canal ativo foi removido, define o primeiro canal como ativo
	if g.activeChannel == normalizedChannel {
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
// GetMeshUI retorna a instância da UI da mesh
func (g *GUI) GetMeshUI() *MeshUI {
	return g.meshUI
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



// ClearLogs limpa os logs (método vazio para compatibilidade)
func (g *GUI) ClearLogs() {
	// Não faz nada, apenas para compatibilidade com a interface
}

// Run exibe a janela principal e inicia o loop de eventos
func (g *GUI) Run() {
	g.mainWindow.ShowAndRun()
}

// RunOnMain executa uma função na thread principal da UI de forma segura.
func (g *GUI) RunOnMain(f func()) {
	fyne.Do(f)
}

// Stop encerra a aplicação
func (g *GUI) Stop() {
	g.app.Quit()
}

// updateChannelList atualiza a lista de canais na interface
func (g *GUI) updateChannelList() {
	if g.channelList == nil {
		return
	}
	// Executa na thread principal para garantir a segurança
	g.RunOnMain(func() {
		g.channelList.Refresh()
	})
}



func contains(s []string, e string) bool {
	normalizedE := strings.ToLower(e)
	for _, a := range s {
		if strings.ToLower(a) == normalizedE {
			return true
		}
	}
	return false
}
