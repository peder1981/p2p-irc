package main

import (
    "bufio"
    "flag"
    "fmt"
    "net"
    "os"
    "strings"
    "sync"

    "github.com/peder1981/p2p-irc/internal/config"
    "github.com/peder1981/p2p-irc/internal/crypto"
    "github.com/peder1981/p2p-irc/internal/transport"
    "github.com/peder1981/p2p-irc/internal/ui"
)

func main() {
    port := flag.Int("port", 0, "porta para conexões entrantes")
    cfg := flag.String("config", "", "caminho para o arquivo de configuração (TOML)")
    flag.Parse()

    // Load configuration
    conf, err := config.Load(*cfg)
    if err != nil {
        fmt.Fprintf(os.Stderr, "erro ao carregar config: %v\n", err)
        os.Exit(1)
    }
    fmt.Printf("p2p-irc iniciado na porta %d usando config %q\n", *port, *cfg)
    // Exibe configuração carregada (para debug)
    fmt.Printf("Config: %+v\n", conf)
    // Initialize crypto: load or create identity
    privKey, err := crypto.LoadOrCreateIdentity()
    if err != nil {
        fmt.Fprintf(os.Stderr, "erro ao carregar identidade: %v\n", err)
        os.Exit(1)
    }
    pubKey := crypto.PublicKeyFromPrivate(privKey)
    fmt.Printf("Chave pública de identidade: %x\n", pubKey)

    // Inicializa estado IRC local
    nick, user := "anon", ""
    channels := []string{"#general"}
    activeChannel := "#general"
    peerNick := make(map[string]string)
    peerUser := make(map[string]string)
    peerChannels := make(map[string]map[string]bool)

    // TCP P2P using transport
    listenAddr := fmt.Sprintf(":%d", *port)
    listener, err := transport.Listen(listenAddr)
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

    // Accept incoming
    go func() {
        for c := range listener.AcceptCh {
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
    }()

    // Dial bootstrap
    for _, p := range conf.Network.BootstrapPeers {
        if c, err := transport.Dial(p); err == nil {
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
    }

    // Chat TUI
    chatUI := ui.NewChatUI()
    // Exibe canais iniciais
    chatUI.SetChannels(channels)

    chatUI.SetInputHandler(func(text string) {
        parts := strings.Fields(text)
        if len(parts) == 0 {
            return
        }
        cmd := parts[0]
        if strings.HasPrefix(cmd, "/") {
            switch cmd {
            case "/nick":
                if len(parts) != 2 {
                    chatUI.AddMessage("Uso: /nick <novo>")
                    return
                }
                nick = parts[1]
                for _, c := range conns {
                    fmt.Fprintf(c, "NICK %s\n", nick)
                }
                chatUI.AddMessage("Nick definido: " + nick)
            case "/user":
                if len(parts) != 2 {
                    chatUI.AddMessage("Uso: /user <ident>")
                    return
                }
                user = parts[1]
                for _, c := range conns {
                    fmt.Fprintf(c, "USER %s\n", user)
                }
                chatUI.AddMessage("User definido: " + user)
            case "/join":
                if len(parts) != 2 {
                    chatUI.AddMessage("Uso: /join <#canal>")
                    return
                }
                ch := parts[1]
                if !contains(channels, ch) {
                    channels = append(channels, ch)
                }
                activeChannel = ch
                chatUI.SetChannels(channels)
                for _, c := range conns {
                    fmt.Fprintf(c, "JOIN %s\n", ch)
                }
                chatUI.AddMessage("Entrou em " + ch)
            case "/part":
                if len(parts) != 2 {
                    chatUI.AddMessage("Uso: /part <#canal>")
                    return
                }
                ch := parts[1]
                channels = remove(channels, ch)
                if activeChannel == ch {
                    if len(channels) > 0 {
                        activeChannel = channels[0]
                    } else {
                        activeChannel = ""
                    }
                }
                chatUI.SetChannels(channels)
                for _, c := range conns {
                    fmt.Fprintf(c, "PART %s\n", ch)
                }
                chatUI.AddMessage("Saiu de " + ch)
            case "/msg":
                if len(parts) < 3 {
                    chatUI.AddMessage("Uso: /msg <canal> <mensagem>")
                    return
                }
                target := parts[1]
                msg := strings.Join(parts[2:], " ")
                for _, c := range conns {
                    fmt.Fprintf(c, "PRIVMSG %s :%s\n", target, msg)
                }
                chatUI.AddMessage(fmt.Sprintf("[%s] %s: %s", target, nick, msg))
            case "/peers":
                peerAddrs := []string{}
                for _, c := range conns {
                    peerAddrs = append(peerAddrs, c.RemoteAddr().String())
                }
                chatUI.SetPeers(peerAddrs)
                chatUI.AddMessage(fmt.Sprintf("Peers: %v", peerAddrs))
            case "/connect":
                if len(parts) != 2 {
                    chatUI.AddMessage("Uso: /connect <host:port>")
                    return
                }
                addr := parts[1]
                c, err := transport.Dial(addr)
                if err != nil {
                    chatUI.AddMessage(fmt.Sprintf("Erro ao conectar %s: %v", addr, err))
                    return
                }
                addConn(c)
                chatUI.AddMessage(fmt.Sprintf("Conectado a %s", addr))
            case "/help":
                chatUI.AddMessage("Comandos: /nick, /user, /join, /part, /msg, /peers, /connect, /help, /quit, /exit")
            case "/quit", "/exit":
                chatUI.AddMessage("Saindo...")
                os.Exit(0)
            default:
                chatUI.AddMessage("Comando desconhecido: " + cmd)
            }
            return
        }
        // Mensagem normal para canal ativo
        for _, c := range conns {
            fmt.Fprintf(c, "PRIVMSG %s :%s\n", activeChannel, text)
        }
        chatUI.AddMessage(fmt.Sprintf("[%s] %s: %s", activeChannel, nick, text))
    })

    go func() {
        for pm := range recvCh {
            parts := strings.Fields(pm.Text)
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
            case "PRIVMSG":
                if len(parts) >= 3 {
                    target := parts[1]
                    msg := strings.TrimPrefix(strings.Join(parts[2:], " "), ":")
                    sender := peerNick[pm.Conn.RemoteAddr().String()]
                    if target == activeChannel {
                        chatUI.AddMessage(fmt.Sprintf("%s: %s", sender, msg))
                    }
                }
            default:
                chatUI.AddMessage(pm.Text)
            }
        }
    }()

    // roda TUI
    if err := chatUI.Run(); err != nil {
        fmt.Fprintf(os.Stderr, "erro UI: %v\n", err)
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
