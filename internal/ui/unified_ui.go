package ui

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// UnifiedUI representa a interface unificada do cliente IRC
type UnifiedUI struct {
	// Componentes da aplicação
	app           *tview.Application
	mainLayout    *tview.Flex
	
	// Áreas principais
	channelList   *tview.TextView
	peerList      *tview.TextView
	chatView      *tview.TextView
	logView       *tview.TextView
	statusBar     *tview.TextView
	input         *tview.InputField
	
	// Dados
	channels      map[string]*tview.TextView
	activeChannel string
	debugMode     bool
	unreadMsgs    map[string]int
	lastMsgTime   map[string]time.Time
	logBuffer     []string
	maxLogLines   int
	
	// Handlers
	inputHandler  func(string)
}

// NewUnifiedUI cria uma nova instância da interface unificada
func NewUnifiedUI() *UnifiedUI {
	app := tview.NewApplication()

	// Cria os componentes da interface
	channelList := tview.NewTextView().
		SetDynamicColors(true).
		SetChangedFunc(func() { app.Draw() })
	channelList.SetBorder(true).SetTitle("Canais")

	peerList := tview.NewTextView().
		SetDynamicColors(true).
		SetChangedFunc(func() { app.Draw() })
	peerList.SetBorder(true).SetTitle("Peers")

	chatView := tview.NewTextView().
		SetDynamicColors(true).
		SetChangedFunc(func() { app.Draw() })
	chatView.SetBorder(true).SetTitle("Chat (#general)")
	chatView.SetScrollable(true)

	logView := tview.NewTextView().
		SetDynamicColors(true).
		SetChangedFunc(func() { app.Draw() })
	logView.SetBorder(true).SetTitle("Logs")
	logView.SetScrollable(true)

	statusBar := tview.NewTextView().
		SetDynamicColors(true).
		SetChangedFunc(func() { app.Draw() })
	statusBar.SetTextColor(tcell.ColorYellow)

	input := tview.NewInputField().
		SetLabel("Mensagem: ").
		SetFieldWidth(0)
	input.SetBorder(true)

	// Cria o layout principal
	// Painel esquerdo: canais e peers
	leftPanel := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(channelList, 0, 2, false).
		AddItem(peerList, 0, 1, false)

	// Painel direito: chat e logs
	rightPanel := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(chatView, 0, 3, false).
		AddItem(logView, 0, 1, false)

	// Layout horizontal principal
	contentArea := tview.NewFlex().
		AddItem(leftPanel, 25, 1, false).
		AddItem(rightPanel, 0, 3, false)

	// Layout vertical principal com barra de status e entrada
	mainLayout := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(contentArea, 0, 1, false).
		AddItem(statusBar, 1, 0, false).
		AddItem(input, 3, 0, true)

	// Inicializa os mapas
	channels := make(map[string]*tview.TextView)
	unreadMsgs := make(map[string]int)
	lastMsgTime := make(map[string]time.Time)

	// Cria a interface
	ui := &UnifiedUI{
		app:           app,
		mainLayout:    mainLayout,
		channelList:   channelList,
		peerList:      peerList,
		chatView:      chatView,
		logView:       logView,
		statusBar:     statusBar,
		input:         input,
		channels:      channels,
		activeChannel: "#general",
		debugMode:     false,
		unreadMsgs:    unreadMsgs,
		lastMsgTime:   lastMsgTime,
		logBuffer:     make([]string, 0),
		maxLogLines:   100,
	}

	// Configura atalhos de teclado globais
	app.SetInputCapture(ui.handleGlobalKeys)

	// Define o root da aplicação
	app.SetRoot(mainLayout, true)

	// Configura a barra de status inicial
	ui.updateStatusBar("Conectado. Use Alt+1-9 para alternar entre canais. F1 para ajuda.")

	// Configura o handler padrão para o campo de entrada
	ui.setupDefaultInputHandler()

	return ui
}

// handleGlobalKeys processa teclas de atalho globais
func (ui *UnifiedUI) handleGlobalKeys(event *tcell.EventKey) *tcell.EventKey {
	// F1: Exibe ajuda
	if event.Key() == tcell.KeyF1 {
		ui.showHelp()
		return nil
	}

	// Alt+1, Alt+2, etc. para alternar entre canais
	if event.Modifiers() == tcell.ModAlt {
		switch event.Key() {
		case tcell.KeyRune:
			switch event.Rune() {
			case '1', '2', '3', '4', '5', '6', '7', '8', '9':
				idx := int(event.Rune() - '1')
				// Obtém a lista de canais
				var channelsList []string
				for channel := range ui.channels {
					channelsList = append(channelsList, channel)
				}
				sort.Strings(channelsList)

				// Verifica se o índice é válido
				if idx >= 0 && idx < len(channelsList) {
					ui.SetActiveChannel(channelsList[idx])
				}
				return nil
			}
		}
	}

	// Ctrl+L: Limpa a área de logs
	if event.Key() == tcell.KeyCtrlL {
		ui.ClearLogs()
		return nil
	}

	// Ctrl+P: Alterna entre painéis (canais, peers, chat, logs)
	if event.Key() == tcell.KeyCtrlP {
		ui.cycleFocus()
		return nil
	}

	// Ctrl+N: Cria um novo canal
	if event.Key() == tcell.KeyCtrlN {
		ui.promptNewChannel()
		return nil
	}

	// Escape: Volta para o input
	if event.Key() == tcell.KeyEscape {
		ui.app.SetFocus(ui.input)
		return nil
	}

	// Passa o evento para o próximo handler
	return event
}

// showHelp exibe uma tela de ajuda com os atalhos disponíveis
func (ui *UnifiedUI) showHelp() {
	helpText := tview.NewTextView().
		SetDynamicColors(true).
		SetText(`
[yellow]== Atalhos de Teclado ==[white]

[green]Navegação:[white]
F1             : Exibe esta ajuda
Alt+1-9        : Alternar entre canais
Ctrl+P         : Alternar foco entre painéis
Esc            : Voltar para o campo de entrada

[green]Ações:[white]
Ctrl+L         : Limpar logs
Ctrl+N         : Criar novo canal
Enter          : Enviar mensagem/comando

[green]Comandos:[white]
/nick <nome>   : Alterar nickname
/join <#canal> : Entrar em um canal
/part [#canal] : Sair do canal atual ou especificado
/msg <alvo>    : Enviar mensagem privada
/who [canal]   : Listar usuários
/away [msg]    : Definir status de ausência
/back          : Remover status de ausência
/me <ação>     : Enviar ação
/topic [tópico]: Ver ou definir tópico do canal
/help          : Mostrar ajuda de comandos
/quit          : Sair da aplicação

Pressione ESC para fechar esta ajuda.
`).
		SetScrollable(true)
	helpText.SetBorder(true).SetTitle("Ajuda - Atalhos de Teclado")

	// Cria um modal com o texto de ajuda
	modal := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(
			tview.NewFlex().
				SetDirection(tview.FlexRow).
				AddItem(nil, 0, 1, false).
				AddItem(helpText, 20, 1, false).
				AddItem(nil, 0, 1, false),
			60, 1, false,
		).
		AddItem(nil, 0, 1, false)

	// Configura o handler para fechar o modal com ESC
	oldHandler := ui.app.GetInputCapture()
	ui.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			ui.app.SetRoot(ui.mainLayout, true)
			ui.app.SetInputCapture(oldHandler)
			return nil
		}
		return event
	})

	// Exibe o modal
	ui.app.SetRoot(modal, true)
}

// cycleFocus alterna o foco entre os diferentes painéis
func (ui *UnifiedUI) cycleFocus() {
	currentFocus := ui.app.GetFocus()
	
	switch currentFocus {
	case ui.input:
		ui.app.SetFocus(ui.channelList)
	case ui.channelList:
		ui.app.SetFocus(ui.peerList)
	case ui.peerList:
		ui.app.SetFocus(ui.chatView)
	case ui.chatView:
		ui.app.SetFocus(ui.logView)
	case ui.logView:
		ui.app.SetFocus(ui.input)
	default:
		ui.app.SetFocus(ui.input)
	}
	
	ui.updateStatusBar(fmt.Sprintf("Foco alterado para %s", ui.getFocusName()))
}

// getFocusName retorna o nome do componente atualmente em foco
func (ui *UnifiedUI) getFocusName() string {
	currentFocus := ui.app.GetFocus()
	
	switch currentFocus {
	case ui.input:
		return "Campo de Entrada"
	case ui.channelList:
		return "Lista de Canais"
	case ui.peerList:
		return "Lista de Peers"
	case ui.chatView:
		return "Área de Chat"
	case ui.logView:
		return "Área de Logs"
	default:
		return "Desconhecido"
	}
}

// promptNewChannel exibe um prompt para criar um novo canal
func (ui *UnifiedUI) promptNewChannel() {
	// Salva o estado atual
	oldText := ui.input.GetText()
	oldLabel := ui.input.GetLabel()
	
	// Configura o input para receber o nome do canal
	ui.input.SetLabel("Novo canal: #").SetText("")
	
	// Configura um novo handler para o input
	ui.input.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			channelName := ui.input.GetText()
			if channelName != "" {
				// Adiciona o prefixo # se não estiver presente
				if !strings.HasPrefix(channelName, "#") {
					channelName = "#" + channelName
				}
				
				// Executa o comando /join
				if ui.inputHandler != nil {
					ui.inputHandler("/join " + channelName)
				}
			}
			
			// Restaura o input
			ui.input.SetLabel(oldLabel).SetText(oldText)
			ui.setupDefaultInputHandler()
		} else if key == tcell.KeyEscape {
			// Cancela e restaura o input
			ui.input.SetLabel(oldLabel).SetText(oldText)
			ui.setupDefaultInputHandler()
		}
	})
	
	ui.updateStatusBar("Digite o nome do novo canal (sem #) e pressione Enter, ou ESC para cancelar")
}

// setupDefaultInputHandler configura o handler padrão para o campo de entrada
func (ui *UnifiedUI) setupDefaultInputHandler() {
	if ui.inputHandler != nil {
		ui.input.SetDoneFunc(func(key tcell.Key) {
			if key != tcell.KeyEnter {
				return
			}

			text := ui.input.GetText()
			if text != "" {
				// Limpa o campo de entrada antes de processar o comando
				ui.input.SetText("")

				// Chama o handler em uma goroutine separada para evitar bloqueios na UI
				go func(input string) {
					ui.inputHandler(input)
				}(text)
			}
		})
	}
}

// SetInputHandler define a função chamada ao enviar mensagem
func (ui *UnifiedUI) SetInputHandler(handler func(string)) {
	ui.inputHandler = handler
	ui.setupDefaultInputHandler()
}

// AddMessage adiciona uma nova mensagem ao chat
func (ui *UnifiedUI) AddMessage(msg string) {
	// Remove códigos de escape ANSI
	cleanMsg := cleanText(msg)

	// Verifica se é uma mensagem de depuração
	if strings.HasPrefix(cleanMsg, "DEBUG:") {
		if !ui.debugMode {
			// Ignora mensagens de depuração se o modo não estiver ativo
			return
		}

		// Adiciona à área de logs
		ui.AddLogMessage(cleanMsg)
		return
	}

	// Adiciona a mensagem ao canal #general
	if channel, exists := ui.channels["#general"]; exists {
		fmt.Fprintf(channel, "%s\n", cleanMsg)
	} else {
		// Cria o canal #general se não existir
		ui.channels["#general"] = tview.NewTextView().
			SetDynamicColors(true).
			SetChangedFunc(func() { ui.app.Draw() })
		fmt.Fprintf(ui.channels["#general"], "%s\n", cleanMsg)
	}

	// Se o canal ativo for #general, atualiza a visualização
	if ui.activeChannel == "#general" {
		ui.chatView.Clear()
		ui.chatView.SetText(ui.channels["#general"].GetText(true))
		ui.chatView.ScrollToEnd()
	} else {
		// Se não for o canal ativo, incrementa o contador de mensagens não lidas
		ui.unreadMsgs["#general"]++
		// Atualiza o timestamp da última mensagem
		ui.lastMsgTime["#general"] = time.Now()
		// Atualiza a lista de canais para mostrar indicadores
		ui.updateChannelList()
	}

	// Salva a mensagem no histórico
	ui.saveMessageToHistory("#general", cleanMsg)
}

// AddMessageToChannel adiciona uma mensagem a um canal específico
func (ui *UnifiedUI) AddMessageToChannel(channel, msg string) {
	// Remove códigos de escape ANSI
	cleanMsg := cleanText(msg)

	// Verifica se o canal existe
	if _, exists := ui.channels[channel]; !exists {
		// Cria o canal se não existir
		ui.channels[channel] = tview.NewTextView().
			SetDynamicColors(true).
			SetChangedFunc(func() { ui.app.Draw() })
	}

	// Adiciona a mensagem ao canal
	fmt.Fprintf(ui.channels[channel], "%s\n", cleanMsg)

	// Se for o canal ativo, atualiza a visualização
	if channel == ui.activeChannel {
		ui.chatView.Clear()
		ui.chatView.SetText(ui.channels[channel].GetText(true))
		ui.chatView.ScrollToEnd()
	} else {
		// Se não for o canal ativo, incrementa o contador de mensagens não lidas
		ui.unreadMsgs[channel]++
		// Atualiza o timestamp da última mensagem
		ui.lastMsgTime[channel] = time.Now()
		// Atualiza a lista de canais para mostrar indicadores
		ui.updateChannelList()
	}

	// Salva a mensagem no histórico
	ui.saveMessageToHistory(channel, cleanMsg)
}

// AddLogMessage adiciona uma mensagem à área de logs
func (ui *UnifiedUI) AddLogMessage(msg string) {
	// Remove códigos de escape ANSI
	cleanMsg := cleanText(msg)
	
	// Adiciona a mensagem ao buffer de logs
	ui.logBuffer = append(ui.logBuffer, cleanMsg)
	
	// Limita o tamanho do buffer
	if len(ui.logBuffer) > ui.maxLogLines {
		ui.logBuffer = ui.logBuffer[len(ui.logBuffer)-ui.maxLogLines:]
	}
	
	// Atualiza a visualização dos logs
	ui.updateLogView()
}

// ClearLogs limpa a área de logs
func (ui *UnifiedUI) ClearLogs() {
	ui.logBuffer = make([]string, 0)
	ui.updateLogView()
	ui.updateStatusBar("Logs limpos")
}

// updateLogView atualiza a visualização da área de logs
func (ui *UnifiedUI) updateLogView() {
	// Limpa a área de logs
	ui.logView.Clear()
	
	// Adiciona todas as mensagens do buffer
	for _, msg := range ui.logBuffer {
		fmt.Fprintf(ui.logView, "%s\n", msg)
	}
	
	// Rola para o final
	ui.logView.ScrollToEnd()
}

// updateStatusBar atualiza a barra de status
func (ui *UnifiedUI) updateStatusBar(msg string) {
	ui.statusBar.Clear()
	fmt.Fprintf(ui.statusBar, "%s", msg)
	ui.app.Draw()
}

// updateChannelList atualiza a lista de canais com indicadores de mensagens não lidas
func (ui *UnifiedUI) updateChannelList() {
	ui.channelList.Clear()
	
	// Obtém a lista de canais
	var channelsList []string
	for channel := range ui.channels {
		channelsList = append(channelsList, channel)
	}
	
	// Ordena os canais
	sort.Strings(channelsList)
	
	// Adiciona cada canal à lista
	for i, channel := range channelsList {
		prefix := fmt.Sprintf("%d: ", i+1)
		
		// Destaca o canal ativo
		if channel == ui.activeChannel {
			fmt.Fprintf(ui.channelList, "[green]%s%s[white]", prefix, channel)
		} else {
			// Verifica se há mensagens não lidas
			unread := ui.unreadMsgs[channel]
			if unread > 0 {
				// Mostra o número de mensagens não lidas
				fmt.Fprintf(ui.channelList, "[yellow]%s%s (%d)[white]", prefix, channel, unread)
			} else {
				fmt.Fprintf(ui.channelList, "%s%s", prefix, channel)
			}
		}
		
		fmt.Fprintf(ui.channelList, "\n")
	}
}

// SetActiveChannel define o canal ativo
func (ui *UnifiedUI) SetActiveChannel(channel string) {
	// Verifica se o canal existe
	if _, exists := ui.channels[channel]; !exists {
		// Cria o canal se não existir
		ui.channels[channel] = tview.NewTextView().
			SetDynamicColors(true).
			SetChangedFunc(func() { ui.app.Draw() })
		
		// Carrega o histórico do canal
		ui.loadChannelHistory(channel)
	}
	
	// Atualiza o canal ativo
	ui.activeChannel = channel
	
	// Atualiza o título do chat
	ui.chatView.SetTitle(fmt.Sprintf("Chat (%s)", channel))
	
	// Limpa e atualiza a visualização com o conteúdo do canal
	ui.chatView.Clear()
	if content, exists := ui.channels[channel]; exists {
		ui.chatView.SetText(content.GetText(true))
	}
	ui.chatView.ScrollToEnd()
	
	// Reseta o contador de mensagens não lidas para este canal
	ui.unreadMsgs[channel] = 0
	
	// Atualiza a lista de canais
	ui.updateChannelList()
	
	// Atualiza a barra de status
	ui.updateStatusBar(fmt.Sprintf("Canal ativo: %s", channel))
}

// GetActiveChannel retorna o canal ativo
func (ui *UnifiedUI) GetActiveChannel() string {
	return ui.activeChannel
}

// SetPeers atualiza a lista de peers no painel
func (ui *UnifiedUI) SetPeers(peers []string) {
	ui.peerList.Clear()
	for _, p := range peers {
		fmt.Fprintf(ui.peerList, "%s\n", p)
	}
}

// SetChannels atualiza a lista de canais
func (ui *UnifiedUI) SetChannels(channels []string) {
	// Inicializa os canais
	for _, ch := range channels {
		// Cria o canal se não existir
		if _, exists := ui.channels[ch]; !exists {
			ui.channels[ch] = tview.NewTextView().
				SetDynamicColors(true).
				SetChangedFunc(func() { ui.app.Draw() })
			
			// Carrega o histórico do canal
			ui.loadChannelHistory(ch)
		}
	}
	
	// Atualiza a lista de canais
	ui.updateChannelList()
}

// SetDebugMode ativa ou desativa o modo de depuração
func (ui *UnifiedUI) SetDebugMode(enabled bool) {
	ui.debugMode = enabled
	if enabled {
		ui.updateStatusBar("Modo de depuração ativado")
	} else {
		ui.updateStatusBar("Modo de depuração desativado")
	}
}

// saveMessageToHistory salva uma mensagem no histórico persistente
func (ui *UnifiedUI) saveMessageToHistory(channel, message string) {
	// Cria o diretório history se não existir
	historyDir := "history"
	if _, err := os.Stat(historyDir); os.IsNotExist(err) {
		os.Mkdir(historyDir, 0755)
	}
	
	// Nome do arquivo: history/channel_name.log (sem o prefixo #)
	filename := filepath.Join(historyDir, strings.TrimPrefix(channel, "#") + ".log")
	
	// Abre o arquivo para append
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	
	// Adiciona timestamp à mensagem
	timestamp := time.Now().Format("[2006-01-02 15:04:05]")
	fmt.Fprintf(f, "%s %s\n", timestamp, message)
}

// loadChannelHistory carrega o histórico de mensagens de um canal
func (ui *UnifiedUI) loadChannelHistory(channel string) {
	historyDir := "history"
	filename := filepath.Join(historyDir, strings.TrimPrefix(channel, "#") + ".log")
	
	// Verifica se o arquivo existe
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return
	}
	
	// Abre o arquivo para leitura
	f, err := os.Open(filename)
	if err != nil {
		return
	}
	defer f.Close()
	
	// Cria o canal se não existir
	if _, exists := ui.channels[channel]; !exists {
		ui.channels[channel] = tview.NewTextView().
			SetDynamicColors(true).
			SetChangedFunc(func() { ui.app.Draw() })
	}
	
	// Limpa o conteúdo atual do canal
	ui.channels[channel].Clear()
	
	// Lê as últimas 100 linhas do arquivo
	scanner := bufio.NewScanner(f)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) > 100 {
			lines = lines[1:]
		}
	}
	
	// Adiciona as linhas ao canal
	for _, line := range lines {
		fmt.Fprintf(ui.channels[channel], "%s\n", line)
	}
	
	// Se for o canal ativo, atualiza a visualização
	if channel == ui.activeChannel {
		ui.chatView.Clear()
		ui.chatView.SetText(ui.channels[channel].GetText(true))
		ui.chatView.ScrollToEnd()
	}
}

// cleanText remove códigos de escape ANSI de uma string
func cleanText(s string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	return re.ReplaceAllString(s, "")
}

// Run inicia a aplicação TUI
func (ui *UnifiedUI) Run() error {
	return ui.app.Run()
}
