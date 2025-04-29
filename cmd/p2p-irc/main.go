package main

import (
    "bufio"
    "flag"
    "fmt"
    "net"
    "os"
    "strings"
    "crypto/rand"
    "encoding/hex"
    "sync"
    "github.com/pion/webrtc/v3"
    "time"

    "github.com/peder1981/p2p-irc/internal/config"
    "github.com/peder1981/p2p-irc/internal/crypto"
    "github.com/peder1981/p2p-irc/internal/discovery"
    "github.com/peder1981/p2p-irc/internal/transport"
    "github.com/peder1981/p2p-irc/internal/ui"
    "github.com/peder1981/p2p-irc/internal/portmanager"
    "github.com/peder1981/p2p-irc/internal/dcc"
)

func main() {
    cfg := flag.String("config", "", "caminho para o arquivo de configuração (TOML)")
    flag.Parse()

    // Load configuration
    conf, err := config.Load(*cfg)
    if err != nil {
        fmt.Fprintf(os.Stderr, "erro ao carregar config: %v\n", err)
        os.Exit(1)
    }

    // Initialize crypto: load or create identity
    privKey, err := crypto.LoadOrCreateIdentity()
    if err != nil {
        fmt.Fprintf(os.Stderr, "erro ao carregar identidade: %v\n", err)
        os.Exit(1)
    }
    pubKey := crypto.PublicKeyFromPrivate(privKey)
    fmt.Printf("Chave pública de identidade: %x\n", pubKey)

    // Inicializa o gerenciador de portas
    pm := portmanager.New()
    port, err := pm.GetAvailablePort()
    if err != nil {
        fmt.Fprintf(os.Stderr, "erro ao obter porta disponível: %v\n", err)
        os.Exit(1)
    }
    defer pm.ReleasePort(port)

    fmt.Printf("p2p-irc iniciado na porta %d usando config %q\n", port, *cfg)

    // Inicializa estado IRC local
    nick, user := "anon", ""
    channels := []string{"#general"}
    activeChannel := "#general"
    peerNick := make(map[string]string)
    peerUser := make(map[string]string)
    peerChannels := make(map[string]map[string]bool)

    // track seen message IDs to avoid loops
    var seenMu sync.Mutex
    seen := make(map[string]bool)

    // Inicializa tópicos e modos locais
    topics := make(map[string]string)
    channelModes := make(map[string]string)

    // Build ICE servers list for P2P transport
    iceServers := make([]webrtc.ICEServer, 0, len(conf.Network.IceServers)+len(conf.Network.StunServers))
    for _, s := range conf.Network.IceServers {
        iceServers = append(iceServers, webrtc.ICEServer{
            URLs:       s.URLs,
            Username:   s.Username,
            Credential: s.Credential,
        })
    }
    for _, url := range conf.Network.StunServers {
        iceServers = append(iceServers, webrtc.ICEServer{URLs: []string{url}})
    }

    // Inicializa o sistema de descoberta
    discoveryService, err := discovery.New(conf.Network.BootstrapPeers, port)
    if err != nil {
        fmt.Fprintf(os.Stderr, "erro ao inicializar descoberta: %v\n", err)
        os.Exit(1)
    }

    // Inicia o serviço de descoberta
    if err := discoveryService.Start(); err != nil {
        fmt.Fprintf(os.Stderr, "erro ao iniciar descoberta: %v\n", err)
        os.Exit(1)
    }
    defer discoveryService.Stop()

    // TCP P2P usando transport (ICE/STUN/TURN)
    listenAddr := fmt.Sprintf(":%d", port)
    listener, err := transport.Listen(listenAddr, iceServers)
    if err != nil {
        fmt.Fprintf(os.Stderr, "erro iniciar listener: %v\n", err)
        os.Exit(1)
    }
    defer listener.Close()

    var mu sync.Mutex
    conns := make([]net.Conn, 0)
    type peerMessage struct { Conn net.Conn; Text string }
    recvCh := make(chan peerMessage, 10)

    addConn := func(c net.Conn) {
        mu.Lock()
        defer mu.Unlock()
        conns = append(conns, c)
    }

    // addAndListen adds a connection and starts a goroutine to read incoming messages
    addAndListen := func(c net.Conn) {
        addConn(c)
        go func(c net.Conn) {
            r := bufio.NewReader(c)
            for {
                line, err := r.ReadString('\n')
                if err != nil {
                    return
                }
                recvCh <- peerMessage{Conn: c, Text: strings.TrimSpace(line)}
            }
        }(c)
    }

    // Accept incoming
    go func() {
        for c := range listener.AcceptCh {
            addAndListen(c)
        }
    }()

    // Rotina de descoberta e conexão com peers
    go func() {
        for {
            peers := discoveryService.GetPeers()
            for _, peer := range peers {
                // Verifica se já estamos conectados
                mu.Lock()
                connected := false
                for _, conn := range conns {
                    if conn.RemoteAddr().String() == peer.Addr.String() {
                        connected = true
                        break
                    }
                }
                mu.Unlock()

                if !connected {
                    // Tenta conectar ao peer
                    conn, err := transport.Dial(peer.Addr.String(), iceServers)
                    if err != nil {
                        continue
                    }
                    addAndListen(conn)
                }
            }
            time.Sleep(30 * time.Second)
        }
    }()

    // Inicializa UI
    chatUI := ui.NewChatUI()
    
    dccManager := dcc.NewManager("downloads")

    chatUI.SetInputHandler(func(msg string) {
        if strings.HasPrefix(msg, "/dcc ") {
            handleDCCCommand(msg[5:], dccManager, chatUI)
            return
        }

        if strings.HasPrefix(msg, "/") {
            parts := strings.Fields(msg)
            cmd := strings.ToUpper(strings.TrimPrefix(parts[0], "/"))
            switch cmd {
            case "NICK":
                if len(parts) == 2 {
                    nick = parts[1]
                    chatUI.AddMessage(fmt.Sprintf("Seu nick agora é %s", nick))
                    // Broadcast para todos os peers
                    mu.Lock()
                    for _, c := range conns {
                        fmt.Fprintf(c, "NICK %s\n", nick)
                    }
                    mu.Unlock()
                }
            case "USER":
                if len(parts) == 2 {
                    user = parts[1]
                    chatUI.AddMessage(fmt.Sprintf("Seu user agora é %s", user))
                    mu.Lock()
                    for _, c := range conns {
                        fmt.Fprintf(c, "USER %s\n", user)
                    }
                    mu.Unlock()
                }
            case "JOIN":
                if len(parts) == 2 {
                    ch := parts[1]
                    if !contains(channels, ch) {
                        channels = append(channels, ch)
                        activeChannel = ch
                        chatUI.AddMessage(fmt.Sprintf("Entrando em %s", ch))
                        mu.Lock()
                        for _, c := range conns {
                            fmt.Fprintf(c, "JOIN %s\n", ch)
                        }
                        mu.Unlock()
                    }
                }
            case "PART":
                if len(parts) == 2 {
                    ch := parts[1]
                    if contains(channels, ch) {
                        channels = remove(channels, ch)
                        if activeChannel == ch && len(channels) > 0 {
                            activeChannel = channels[0]
                        }
                        chatUI.AddMessage(fmt.Sprintf("Saindo de %s", ch))
                        mu.Lock()
                        for _, c := range conns {
                            fmt.Fprintf(c, "PART %s\n", ch)
                        }
                        mu.Unlock()
                    }
                }
            case "TOPIC":
                if len(parts) >= 3 {
                    ch := parts[1]
                    topic := strings.Join(parts[2:], " ")
                    if contains(channels, ch) {
                        topics[ch] = topic
                        chatUI.AddMessage(fmt.Sprintf("Definindo tópico de %s para %s", ch, topic))
                        mu.Lock()
                        for _, c := range conns {
                            fmt.Fprintf(c, "TOPIC %s :%s\n", ch, topic)
                        }
                        mu.Unlock()
                    }
                }
            case "MODE":
                if len(parts) >= 3 {
                    ch := parts[1]
                    mode := parts[2]
                    if contains(channels, ch) {
                        channelModes[ch] = mode
                        chatUI.AddMessage(fmt.Sprintf("Definindo modo de %s para %s", ch, mode))
                        mu.Lock()
                        for _, c := range conns {
                            fmt.Fprintf(c, "MODE %s %s\n", ch, mode)
                        }
                        mu.Unlock()
                    }
                }
            case "ME":
                if len(parts) >= 2 {
                    action := strings.Join(parts[1:], " ")
                    chatUI.AddMessage(fmt.Sprintf("* %s %s", nick, action))
                    mu.Lock()
                    for _, c := range conns {
                        fmt.Fprintf(c, "PRIVMSG %s :\x01ACTION %s\x01\n", activeChannel, action)
                    }
                    mu.Unlock()
                }
            default:
                chatUI.AddMessage(fmt.Sprintf("Comando desconhecido: %s", cmd))
            }
            return
        }

        // Mensagem normal - envia para o canal ativo
        msgID := newID()
        seenMu.Lock()
        seen[msgID] = true
        seenMu.Unlock()

        mu.Lock()
        for _, c := range conns {
            fmt.Fprintf(c, "PRIVMSG %s :%s\n", activeChannel, msg)
        }
        mu.Unlock()
        chatUI.AddMessage(fmt.Sprintf("%s: %s", nick, msg))
    })

    // Loop principal de processamento de mensagens
    go func() {
        for pm := range recvCh {
            body := pm.Text
            parts := strings.Fields(body)
            if len(parts) == 0 {
                continue
            }
            cmd := parts[0]
            switch cmd {
            case "NICK":
                if len(parts) == 2 {
                    peerNick[pm.Conn.RemoteAddr().String()] = parts[1]
                    chatUI.AddMessage(fmt.Sprintf("%s agora é %s", pm.Conn.RemoteAddr().String(), parts[1]))
                }
            case "USER":
                if len(parts) == 2 {
                    peerUser[pm.Conn.RemoteAddr().String()] = parts[1]
                }
            case "JOIN":
                if len(parts) == 2 {
                    ch := parts[1]
                    if peerChannels[pm.Conn.RemoteAddr().String()] == nil {
                        peerChannels[pm.Conn.RemoteAddr().String()] = make(map[string]bool)
                    }
                    peerChannels[pm.Conn.RemoteAddr().String()][ch] = true
                    chatUI.AddMessage(fmt.Sprintf("%s entrou em %s", peerNick[pm.Conn.RemoteAddr().String()], ch))
                }
            case "PART":
                if len(parts) == 2 {
                    ch := parts[1]
                    if peerChannels[pm.Conn.RemoteAddr().String()] != nil {
                        delete(peerChannels[pm.Conn.RemoteAddr().String()], ch)
                    }
                    chatUI.AddMessage(fmt.Sprintf("%s saiu de %s", peerNick[pm.Conn.RemoteAddr().String()], ch))
                }
            case "TOPIC":
                if len(parts) >= 3 {
                    ch := parts[1]
                    topic := strings.TrimPrefix(strings.Join(parts[2:], " "), ":")
                    topics[ch] = topic
                    chatUI.AddMessage(fmt.Sprintf("Tópico de %s agora é %s", ch, topic))
                }
            case "MODE":
                if len(parts) >= 3 {
                    ch := parts[1]
                    mode := parts[2]
                    channelModes[ch] = mode
                    chatUI.AddMessage(fmt.Sprintf("Modo de %s agora é %s", ch, mode))
                }
            case "PRIVMSG":
                if len(parts) >= 3 {
                    target := parts[1]
                    raw := strings.Join(parts[2:], " ")
                    msg := strings.TrimPrefix(raw, ":")
                    sender := peerNick[pm.Conn.RemoteAddr().String()]
                    // CTCP handling
                    if strings.HasPrefix(msg, "\x01") && strings.HasSuffix(msg, "\x01") {
                        ctcp := strings.TrimSuffix(strings.TrimPrefix(msg, "\x01"), "\x01")
                        chatUI.AddMessage(fmt.Sprintf("CTCP de %s: %s", sender, ctcp))
                        break
                    }
                    if target == activeChannel {
                        chatUI.AddMessage(fmt.Sprintf("%s: %s", sender, msg))
                    }
                }
            case "DCC":
                if len(parts) >= 2 {
                    dccCmd := strings.Join(parts[1:], " ")
                    handleDCCCommand(dccCmd, dccManager, chatUI)
                }
            default:
                chatUI.AddMessage(body)
            }
        }
    }()

    // roda TUI
    if err := chatUI.Run(); err != nil {
        fmt.Fprintf(os.Stderr, "erro UI: %v\n", err)
    }
}

func handleDCCCommand(cmd string, dccManager *dcc.Manager, chatUI *ui.ChatUI) {
    parts := strings.Fields(cmd)
    if len(parts) < 2 {
        chatUI.AddMessage("Uso: /dcc <send|list> [args...]")
        return
    }

    switch parts[0] {
    case "send":
        if len(parts) < 3 {
            chatUI.AddMessage("Uso: /dcc send <arquivo1,arquivo2,...> <destinatário>")
            return
        }

        // Separa lista de arquivos
        files := strings.Split(parts[1], ",")
        receiver := parts[2]

        // Envia múltiplos arquivos
        transfers, err := dccManager.SendFiles(files, receiver)
        if err != nil {
            chatUI.AddMessage(fmt.Sprintf("Erro ao enviar arquivos: %v", err))
            return
        }

        // Envia comando DCC SEND para cada arquivo
        for _, t := range transfers {
            dccCmd := dcc.FormatDCCSend(t.Filename, t.Size, t.Port)
            chatUI.AddMessage(fmt.Sprintf("Iniciando envio de %s para %s (use: %s)", t.Filename, receiver, dccCmd))
        }
    case "list":
        transfers := dccManager.ListTransfers()
        if len(transfers) == 0 {
            chatUI.AddMessage("Nenhuma transferência ativa")
            return
        }

        // Mostra lista de transferências
        chatUI.AddMessage("Transferências ativas:")
        for _, t := range transfers {
            status := "pendente"
            switch t.Status {
            case dcc.StatusInProgress:
                status = "em progresso"
            case dcc.StatusCompleted:
                status = "completa"
            case dcc.StatusFailed:
                status = fmt.Sprintf("falhou: %v", t.Error)
            case dcc.StatusCanceled:
                status = "cancelada"
            }

            msg := fmt.Sprintf("%s -> %s: %s (%d bytes) - %s",
                t.Sender, t.Receiver, t.Filename, t.Size, status)
            chatUI.AddMessage(msg)
        }

    default:
        chatUI.AddMessage("Comando DCC desconhecido. Use: send, list")
    }
}

// contains checa se slice contém elemento
func contains(ss []string, s string) bool {
    for _, v := range ss {
        if v == s {
            return true
        }
    }
    return false
}

// remove retira elemento de slice
func remove(ss []string, s string) []string {
    res := []string{}
    for _, v := range ss {
        if v != s {
            res = append(res, v)
        }
    }
    return res
}

// newID gera um identificador único para mensagens.
func newID() string {
    b := make([]byte, 16)
    if _, err := rand.Read(b); err != nil {
        panic("erro ao gerar ID de mensagem: " + err.Error())
    }
    return hex.EncodeToString(b)
}
