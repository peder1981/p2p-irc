package ui

import (
    "fmt"

    "github.com/gdamore/tcell/v2"
    "github.com/rivo/tview"
)

// ChatUI representa a interface TUI estilo mIRC.
type ChatUI struct {
    app         *tview.Application
    chatView    *tview.TextView
    input       *tview.InputField
    channelList *tview.List
    peerList    *tview.List
}

// NewChatUI cria uma nova instância de ChatUI com layout de canais e chat.
func NewChatUI() *ChatUI {
    app := tview.NewApplication()

    // Lista de canais
    channels := tview.NewList().
        ShowSecondaryText(false).
        AddItem("#general", "Canal padrão", 'g', nil)
    channels.SetBorder(true).SetTitle("Canais")

    // Área de chat
    chat := tview.NewTextView().
        SetDynamicColors(true).
        SetRegions(true).
        SetChangedFunc(func() { app.Draw() })
    chat.SetBorder(true).SetTitle("Chat")

    // Campo de input
    input := tview.NewInputField().
        SetLabel("Mensagem: ").
        SetFieldWidth(0)
    input.SetBorder(true)

    // Lista de peers
    peers := tview.NewList().
        ShowSecondaryText(false)
    peers.SetBorder(true).SetTitle("Peers")

    // Layout: canais | peers | chat
    mainFlex := tview.NewFlex().
        AddItem(channels, 20, 1, false).
        AddItem(peers, 20, 1, false).
        AddItem(chat, 0, 3, false)

    // Layout vertical principal
    layout := tview.NewFlex().
        SetDirection(tview.FlexRow).
        AddItem(mainFlex, 0, 1, false).
        AddItem(input, 3, 0, true)

    app.SetRoot(layout, true)
    return &ChatUI{app: app, chatView: chat, input: input, channelList: channels, peerList: peers}
}

// AddMessage adiciona uma nova mensagem ao chat.
func (c *ChatUI) AddMessage(msg string) {
    fmt.Fprintf(c.chatView, "%s\n", msg)
    c.app.Draw()
}

// SetInputHandler define a função chamada ao enviar mensagem.
func (c *ChatUI) SetInputHandler(handler func(string)) {
    c.input.SetDoneFunc(func(key tcell.Key) {
        text := c.input.GetText()
        if text != "" {
            handler(text)
            c.input.SetText("")
        }
    })
}

// SetPeers atualiza a lista de peers no painel.
func (c *ChatUI) SetPeers(peers []string) {
    // limpa existentes
    count := c.peerList.GetItemCount()
    for i := count - 1; i >= 0; i-- {
        c.peerList.RemoveItem(i)
    }
    // adiciona novos
    for _, p := range peers {
        c.peerList.AddItem(p, "", 0, nil)
    }
}

// SetChannels atualiza a lista de canais no painel.
func (c *ChatUI) SetChannels(channels []string) {
    // limpa canais existentes
    count := c.channelList.GetItemCount()
    for i := count - 1; i >= 0; i-- {
        c.channelList.RemoveItem(i)
    }
    // adiciona canais
    for _, ch := range channels {
        c.channelList.AddItem(ch, "", 0, nil)
    }
}

// Run inicia a aplicação TUI.
func (c *ChatUI) Run() error {
    return c.app.Run()
}
