package ui

import (
    "bufio"
    "fmt"
    "os"
    "regexp"
    "sort"
    "strings"
    "time"

    "github.com/gdamore/tcell/v2"
    "github.com/rivo/tview"
    "path/filepath"
)

// ChatUI representa a interface do usuário do chat.
type ChatUI struct {
    app           *tview.Application
    chatView      *tview.TextView
    input         *tview.InputField
    channelList   *tview.TextView
    peerList      *tview.TextView
    channels      map[string]*tview.TextView
    activeChannel string
    debugMode     bool
    unreadMsgs    map[string]int
    lastMsgTime   map[string]time.Time
    logView       *tview.TextView
    logBuffer     []string      // Buffer para armazenar mensagens de log
    maxLogLines   int           // Número máximo de linhas de log a manter
    windows       map[string]*tview.Flex
    activeWindow  string
}

// NewChatUI cria uma nova instância de ChatUI com layout de canais e chat.
func NewChatUI() *ChatUI {
    app := tview.NewApplication()

    // Lista de canais
    channelList := tview.NewTextView().
        SetDynamicColors(true).
        SetChangedFunc(func() { app.Draw() })
    channelList.SetBorder(true).SetTitle("Canais")

    // Área de chat
    chatView := tview.NewTextView().
        SetDynamicColors(true).
        SetChangedFunc(func() { app.Draw() })
    chatView.SetBorder(true).SetTitle("Chat")

    // Campo de input
    input := tview.NewInputField().
        SetLabel("Mensagem: ").
        SetFieldWidth(0)
    input.SetBorder(true)

    // Lista de peers
    peerList := tview.NewTextView().
        SetDynamicColors(true).
        SetChangedFunc(func() { app.Draw() })
    peerList.SetBorder(true).SetTitle("Peers")

    // Área de logs separada
    logView := tview.NewTextView().
        SetDynamicColors(true).
        SetChangedFunc(func() { app.Draw() })
    logView.SetBorder(true).SetTitle("Logs")

    // Layout principal: dividido em duas colunas
    // Coluna esquerda: canais e peers
    leftColumn := tview.NewFlex().
        SetDirection(tview.FlexRow).
        AddItem(channelList, 0, 1, false).
        AddItem(peerList, 0, 1, false)

    // Coluna direita: chat e logs
    rightColumn := tview.NewFlex().
        SetDirection(tview.FlexRow).
        AddItem(chatView, 0, 3, false).
        AddItem(logView, 10, 1, false)

    // Layout horizontal principal
    mainFlex := tview.NewFlex().
        AddItem(leftColumn, 25, 1, false).
        AddItem(rightColumn, 0, 3, false)

    // Layout vertical principal
    layout := tview.NewFlex().
        SetDirection(tview.FlexRow).
        AddItem(mainFlex, 0, 1, false).
        AddItem(input, 3, 0, true)

    // Inicializa os mapas
    channels := make(map[string]*tview.TextView)
    unreadMsgs := make(map[string]int)
    lastMsgTime := make(map[string]time.Time)
    windows := make(map[string]*tview.Flex)

    // Adiciona a janela principal
    windows["main"] = layout

    // Cria a interface
    c := &ChatUI{
        app:           app,
        chatView:      chatView,
        input:         input,
        channelList:   channelList,
        peerList:      peerList,
        channels:      channels,
        activeChannel: "#general",
        debugMode:     false,
        unreadMsgs:    unreadMsgs,
        lastMsgTime:   lastMsgTime,
        logView:       logView,
        logBuffer:     make([]string, 0),
        maxLogLines:   100,  // Mantém até 100 linhas de log
        windows:       windows,
        activeWindow:  "main",
    }

    // Configura atalhos de teclado para alternar entre canais e janelas
    app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
        // Alt+1, Alt+2, etc. para alternar entre canais
        if event.Modifiers() == tcell.ModAlt {
            switch event.Key() {
            case tcell.KeyRune:
                switch event.Rune() {
                case '1', '2', '3', '4', '5', '6', '7', '8', '9':
                    idx := int(event.Rune() - '1')
                    // Obtém a lista de canais
                    var channelsList []string
                    for channel := range c.channels {
                        channelsList = append(channelsList, channel)
                    }

                    // Verifica se o índice é válido
                    if idx >= 0 && idx < len(channelsList) {
                        c.SetActiveChannel(channelsList[idx])
                    }
                    return nil
                }
            }
        }

        // Ctrl+1, Ctrl+2, etc. para alternar entre janelas
        if event.Modifiers() == tcell.ModCtrl {
            switch event.Key() {
            case tcell.KeyRune:
                switch event.Rune() {
                case '1', '2', '3', '4', '5', '6', '7', '8', '9':
                    idx := int(event.Rune() - '1')
                    // Obtém a lista de janelas
                    var windowsList []string
                    for window := range c.windows {
                        windowsList = append(windowsList, window)
                    }

                    // Verifica se o índice é válido
                    if idx >= 0 && idx < len(windowsList) {
                        c.SetActiveWindow(windowsList[idx])
                    }
                    return nil
                }
            }
        }

        // Ctrl+N para criar uma nova janela
        if event.Key() == tcell.KeyCtrlN {
            c.CreateNewWindow()
            return nil
        }

        // Ctrl+W para fechar a janela atual
        if event.Key() == tcell.KeyCtrlW {
            c.CloseActiveWindow()
            return nil
        }

        return event
    })

    app.SetRoot(layout, true)
    return c
}

// CreateNewWindow cria uma nova janela
func (c *ChatUI) CreateNewWindow() {
    // Gera um nome único para a janela
    windowName := fmt.Sprintf("window_%d", len(c.windows)+1)
    
    // Cria os componentes da nova janela
    channelList := tview.NewTextView().
        SetDynamicColors(true).
        SetChangedFunc(func() { c.app.Draw() })
    channelList.SetBorder(true).SetTitle("Canais")

    chatView := tview.NewTextView().
        SetDynamicColors(true).
        SetChangedFunc(func() { c.app.Draw() })
    chatView.SetBorder(true).SetTitle("Chat")

    peerList := tview.NewTextView().
        SetDynamicColors(true).
        SetChangedFunc(func() { c.app.Draw() })
    peerList.SetBorder(true).SetTitle("Peers")
    
    logView := tview.NewTextView().
        SetDynamicColors(true).
        SetChangedFunc(func() { c.app.Draw() })
    logView.SetBorder(true).SetTitle("Logs")

    // Layout principal: dividido em duas colunas
    // Coluna esquerda: canais e peers
    leftColumn := tview.NewFlex().
        SetDirection(tview.FlexRow).
        AddItem(channelList, 0, 1, false).
        AddItem(peerList, 0, 1, false)

    // Coluna direita: chat e logs
    rightColumn := tview.NewFlex().
        SetDirection(tview.FlexRow).
        AddItem(chatView, 0, 3, false).
        AddItem(logView, 10, 1, false)

    // Layout horizontal principal
    mainFlex := tview.NewFlex().
        AddItem(leftColumn, 25, 1, false).
        AddItem(rightColumn, 0, 3, false)

    // Layout vertical principal
    layout := tview.NewFlex().
        SetDirection(tview.FlexRow).
        AddItem(mainFlex, 0, 1, false).
        AddItem(c.input, 3, 0, true)
    
    // Adiciona a nova janela
    c.windows[windowName] = layout
    
    // Ativa a nova janela
    c.SetActiveWindow(windowName)
    
    // Notifica o usuário
    c.AddMessage(fmt.Sprintf("Nova janela criada: %s (use Ctrl+<número> para alternar)", windowName))
}

// CloseActiveWindow fecha a janela ativa
func (c *ChatUI) CloseActiveWindow() {
    // Não permite fechar a janela principal
    if c.activeWindow == "main" {
        c.AddMessage("Não é possível fechar a janela principal")
        return
    }

    // Remove a janela
    delete(c.windows, c.activeWindow)

    // Ativa a janela principal
    c.SetActiveWindow("main")

    // Notifica o usuário
    c.AddMessage("Janela fechada. Voltando para a janela principal.")
}

// SetActiveWindow define a janela ativa
func (c *ChatUI) SetActiveWindow(window string) {
    // Verifica se a janela existe
    if _, exists := c.windows[window]; !exists {
        c.AddMessage(fmt.Sprintf("Janela %s não existe", window))
        return
    }

    // Atualiza a janela ativa
    c.activeWindow = window

    // Define o root da aplicação
    c.app.SetRoot(c.windows[window], true)

    // Notifica o usuário
    c.AddMessage(fmt.Sprintf("Janela ativa: %s", window))
}

// AddMessage adiciona uma nova mensagem ao chat.
func (c *ChatUI) AddMessage(msg string) {
    // Remove códigos de escape ANSI que podem causar problemas de formatação
    cleanMsg := removeAnsiEscapes(msg)

    // Verifica se é uma mensagem de depuração
    if strings.HasPrefix(cleanMsg, "DEBUG:") {
        if !c.debugMode {
            // Ignora mensagens de depuração se o modo não estiver ativo
            return
        }

        // Usa o método AddLogMessage para adicionar à área de logs
        c.AddLogMessage(cleanMsg)
        return
    }

    // Adiciona a mensagem ao canal #general
    if channel, exists := c.channels["#general"]; exists {
        fmt.Fprintf(channel, "%s\n", cleanMsg)
    } else {
        // Cria o canal #general se não existir
        c.channels["#general"] = tview.NewTextView().
            SetDynamicColors(true).
            SetChangedFunc(func() { c.app.Draw() })
        fmt.Fprintf(c.channels["#general"], "%s\n", cleanMsg)
    }

    // Se o canal ativo for #general, atualiza a visualização
    if c.activeChannel == "#general" {
        c.chatView.Clear()
        c.chatView.SetText(c.channels["#general"].GetText(true))
    } else {
        // Se não for o canal ativo, incrementa o contador de mensagens não lidas
        c.unreadMsgs["#general"]++
        // Atualiza o timestamp da última mensagem
        c.lastMsgTime["#general"] = time.Now()
        // Atualiza a lista de canais para mostrar indicadores
        c.updateChannelList()
    }

    // Salva a mensagem no histórico
    c.saveMessageToHistory("#general", cleanMsg)
}

// AddMessageToChannel adiciona uma mensagem a um canal específico.
func (c *ChatUI) AddMessageToChannel(channel, msg string) {
    // Remove códigos de escape ANSI
    cleanMsg := removeAnsiEscapes(msg)

    // Verifica se o canal existe
    if _, exists := c.channels[channel]; !exists {
        // Cria o canal se não existir
        c.channels[channel] = tview.NewTextView().
            SetDynamicColors(true).
            SetChangedFunc(func() { c.app.Draw() })
    }

    // Adiciona a mensagem ao canal
    fmt.Fprintf(c.channels[channel], "%s\n", cleanMsg)

    // Se for o canal ativo, atualiza a visualização
    if channel == c.activeChannel {
        c.chatView.Clear()
        c.chatView.SetText(c.channels[channel].GetText(true))
    } else {
        // Se não for o canal ativo, incrementa o contador de mensagens não lidas
        c.unreadMsgs[channel]++
        // Atualiza o timestamp da última mensagem
        c.lastMsgTime[channel] = time.Now()
        // Atualiza a lista de canais para mostrar indicadores
        c.updateChannelList()
    }

    // Salva a mensagem no histórico
    c.saveMessageToHistory(channel, cleanMsg)
}

// AddLogMessage adiciona uma mensagem à área de logs
func (c *ChatUI) AddLogMessage(msg string) {
    // Remove códigos de escape ANSI
    cleanMsg := removeAnsiEscapes(msg)
    
    // Adiciona a mensagem ao buffer de logs
    c.logBuffer = append(c.logBuffer, cleanMsg)
    
    // Limita o tamanho do buffer
    if len(c.logBuffer) > c.maxLogLines {
        c.logBuffer = c.logBuffer[len(c.logBuffer)-c.maxLogLines:]
    }
    
    // Reconstrói a área de logs
    c.updateLogView()
}

// updateLogView atualiza a visualização da área de logs
func (c *ChatUI) updateLogView() {
    // Limpa a área de logs
    c.logView.Clear()
    
    // Adiciona todas as mensagens do buffer
    for _, msg := range c.logBuffer {
        fmt.Fprintf(c.logView, "%s\n", msg)
    }
    
    // Rola para o final
    c.logView.ScrollToEnd()
    
    // Força a atualização da interface
    c.app.Draw()
}

// updateChannelList atualiza a lista de canais com indicadores de mensagens não lidas
func (c *ChatUI) updateChannelList() {
    c.channelList.Clear()

    // Obtém a lista de canais ordenados
    var channelsList []string
    for channel := range c.channels {
        channelsList = append(channelsList, channel)
    }

    // Ordena os canais por timestamp da última mensagem (mais recente primeiro)
    sort.Slice(channelsList, func(i, j int) bool {
        timeI := c.lastMsgTime[channelsList[i]]
        timeJ := c.lastMsgTime[channelsList[j]]
        return timeI.After(timeJ)
    })

    // Exibe os canais com indicadores de mensagens não lidas
    for i, channel := range channelsList {
        unread := c.unreadMsgs[channel]

        if channel == c.activeChannel {
            // Canal ativo em destaque
            fmt.Fprintf(c.channelList, "[green]%d: %s[white]\n", i+1, channel)
        } else if unread > 0 {
            // Canal com mensagens não lidas
            fmt.Fprintf(c.channelList, "[red]%d: %s (%d)[white]\n", i+1, channel, unread)
        } else {
            // Canal normal
            fmt.Fprintf(c.channelList, "%d: %s\n", i+1, channel)
        }
    }
}

// saveMessageToHistory salva uma mensagem no histórico persistente
func (c *ChatUI) saveMessageToHistory(channel, message string) {
    // Cria o diretório de histórico se não existir
    historyDir := "history"
    if _, err := os.Stat(historyDir); os.IsNotExist(err) {
        os.Mkdir(historyDir, 0755)
    }

    // Nome do arquivo: history/channel_name.log
    filename := filepath.Join(historyDir, strings.TrimPrefix(channel, "#") + ".log")

    // Abre o arquivo em modo append
    f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        return
    }
    defer f.Close()

    // Formato: [timestamp] mensagem
    timestamp := time.Now().Format("2006-01-02 15:04:05")
    fmt.Fprintf(f, "[%s] %s\n", timestamp, message)
}

// loadChannelHistory carrega o histórico de mensagens de um canal
func (c *ChatUI) loadChannelHistory(channel string) {
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
    if _, exists := c.channels[channel]; !exists {
        c.channels[channel] = tview.NewTextView().
            SetDynamicColors(true).
            SetChangedFunc(func() { c.app.Draw() })
    }

    // Limpa o conteúdo atual do canal
    c.channels[channel].Clear()

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
        fmt.Fprintf(c.channels[channel], "%s\n", line)
    }

    // Se for o canal ativo, atualiza a visualização
    if channel == c.activeChannel {
        c.chatView.Clear()
        c.chatView.SetText(c.channels[channel].GetText(true))
    }
}

// removeAnsiEscapes remove códigos de escape ANSI de uma string
func removeAnsiEscapes(s string) string {
    // Regex para identificar códigos de escape ANSI
    re := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
    return re.ReplaceAllString(s, "")
}

// SetInputHandler define a função chamada ao enviar mensagem.
func (c *ChatUI) SetInputHandler(handler func(string)) {
    c.input.SetDoneFunc(func(key tcell.Key) {
        if key != tcell.KeyEnter {
            return
        }

        text := c.input.GetText()
        if text != "" {
            // Limpa o campo de entrada antes de processar o comando
            c.input.SetText("")

            // Chama o handler em uma goroutine separada para evitar bloqueios na UI
            go func(input string) {
                handler(input)
            }(text)
        }
    })
}

// SetPeers atualiza a lista de peers no painel.
func (c *ChatUI) SetPeers(peers []string) {
    c.peerList.Clear()
    for _, p := range peers {
        fmt.Fprintf(c.peerList, "%s\n", p)
    }
}

// SetChannels atualiza a lista de canais no painel.
func (c *ChatUI) SetChannels(channels []string) {
    // Inicializa os canais
    for _, ch := range channels {
        // Cria o canal se não existir
        if _, exists := c.channels[ch]; !exists {
            c.channels[ch] = tview.NewTextView().
                SetDynamicColors(true).
                SetChangedFunc(func() { c.app.Draw() })

            // Carrega o histórico do canal
            c.loadChannelHistory(ch)
        }
    }

    // Atualiza a lista de canais
    c.updateChannelList()
}

// SetActiveChannel define o canal ativo.
func (c *ChatUI) SetActiveChannel(channel string) {
    // Verifica se o canal existe
    if _, exists := c.channels[channel]; !exists {
        // Cria o canal se não existir
        c.channels[channel] = tview.NewTextView().
            SetDynamicColors(true).
            SetChangedFunc(func() { c.app.Draw() })

        // Carrega o histórico do canal
        c.loadChannelHistory(channel)
    }

    // Atualiza o canal ativo
    c.activeChannel = channel

    // Atualiza o título do chat
    c.chatView.SetTitle(fmt.Sprintf("Chat (%s)", channel))

    // Limpa e atualiza a visualização com o conteúdo do canal
    c.chatView.Clear()
    if content, exists := c.channels[channel]; exists {
        c.chatView.SetText(content.GetText(true))
    }

    // Reseta o contador de mensagens não lidas para este canal
    c.unreadMsgs[channel] = 0

    // Atualiza a lista de canais
    c.updateChannelList()
}

// SetDebugMode ativa ou desativa o modo de depuração.
func (c *ChatUI) SetDebugMode(enabled bool) {
    c.debugMode = enabled
}

// Run inicia a aplicação TUI.
func (c *ChatUI) Run() error {
    return c.app.Run()
}
