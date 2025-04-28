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
    "github.com/peder1981/p2p-irc/internal/dht"
    "github.com/peder1981/p2p-irc/internal/transport"
    "github.com/peder1981/p2p-irc/internal/ui"
    "github.com/peder1981/p2p-irc/internal/scripts"
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

    // track seen message IDs to avoid loops
    var seenMu sync.Mutex
    seen := make(map[string]bool)

    // Inicializa DHT stub para descoberta de canais
    dhtMgr := dht.NewDHT()
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

    // TCP P2P usando transport (ICE/STUN/TURN)
    listenAddr := fmt.Sprintf(":%d", *port)
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

    // Peer discovery on local network via UDP broadcast
    go func() {
        // Determine local IP for announcing
        myIP := getOutboundIP()
        myAddr := fmt.Sprintf("%s:%d", myIP, *port)
        // Listen for discovery messages on UDP port same as TCP port
        udpAddr := net.UDPAddr{IP: net.IPv4zero, Port: *port}
        uc, err := net.ListenUDP("udp4", &udpAddr)
        if err != nil {
            fmt.Fprintf(os.Stderr, "erro iniciar discovery UDP: %v\n", err)
            return
        }
        defer uc.Close()
        // Broadcast our address periodically
        ticker := time.NewTicker(3 * time.Second)
        defer ticker.Stop()
        baddr := net.UDPAddr{IP: net.IPv4bcast, Port: *port}
        go func() {
            for range ticker.C {
                uc.WriteToUDP([]byte(myAddr), &baddr)
            }
        }()
        buf := make([]byte, 1024)
        for {
            n, _, err := uc.ReadFromUDP(buf)
            if err != nil {
                continue
            }
            peerAddr := string(buf[:n])
            if peerAddr == myAddr {
                continue
            }
            mu.Lock()
            already := false
            for _, c := range conns {
                if c.RemoteAddr().String() == peerAddr {
                    already = true
                    break
                }
            }
            mu.Unlock()
            if already {
                continue
            }
            c, err := transport.Dial(peerAddr, iceServers)
            if err != nil {
                continue
            }
            addAndListen(c)
        }
    }()

    // Dial bootstrap
    for _, p := range conf.Network.BootstrapPeers {
        if c, err := transport.Dial(p, iceServers); err == nil {
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

    // Registrar canais iniciais no DHT
    for _, ch0 := range channels {
        dhtMgr.Register(ch0, listener.Addr().String())
    }
    // Chat TUI
    chatUI := ui.NewChatUI()
    // Initialize scripting/alias engine
    scriptsPath, err := scripts.DefaultScriptsPath()
    if err != nil {
        fmt.Fprintf(os.Stderr, "erro determinar scripts path: %v\n", err)
    }
    scriptEngine, err := scripts.NewEngine(scriptsPath)
    if err != nil {
        fmt.Fprintf(os.Stderr, "erro carregar scripts: %v\n", err)
    }
    // Exibe canais iniciais
    chatUI.SetChannels(channels)

    chatUI.SetInputHandler(func(text string) {
        // alias expansion
        text = scriptEngine.Expand(text)
        // generate unique ID for this message
        id := newID()
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
                    fmt.Fprintf(c, "%s NICK %s\n", id, nick)
                }
                chatUI.AddMessage("Nick definido: " + nick)
            case "/user":
                if len(parts) != 2 {
                    chatUI.AddMessage("Uso: /user <ident>")
                    return
                }
                user = parts[1]
                for _, c := range conns {
                    fmt.Fprintf(c, "%s USER %s\n", id, user)
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
                    fmt.Fprintf(c, "%s JOIN %s\n", id, ch)
                }
                chatUI.AddMessage("Entrou em " + ch)
                // Registrar canal no DHT
                dhtMgr.Register(ch, listener.Addr().String())
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
                    fmt.Fprintf(c, "%s PART %s\n", id, ch)
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
                    fmt.Fprintf(c, "%s PRIVMSG %s :%s\n", id, target, msg)
                }
                chatUI.AddMessage(fmt.Sprintf("[%s] %s: %s", target, nick, msg))
            case "/peers":
                peerAddrs := []string{}
                for _, c := range conns {
                    peerAddrs = append(peerAddrs, c.RemoteAddr().String())
                }
                chatUI.SetPeers(peerAddrs)
                chatUI.AddMessage(fmt.Sprintf("Peers: %v", peerAddrs))
            case "/topic":
                if len(parts) < 3 {
                    chatUI.AddMessage("Uso: /topic <#canal> <tópico>")
                    return
                }
                ch := parts[1]
                topic := strings.Join(parts[2:], " ")
                topics[ch] = topic
                for _, c := range conns {
                    fmt.Fprintf(c, "%s TOPIC %s :%s\n", id, ch, topic)
                }
                chatUI.AddMessage(fmt.Sprintf("Tópico de %s definido: %s", ch, topic))
            case "/list":
                if len(parts) == 1 {
                    chans := dhtMgr.Channels()
                    chatUI.AddMessage(fmt.Sprintf("Canais disponíveis: %v", chans))
                } else if len(parts) == 2 {
                    ch := parts[1]
                    peers := dhtMgr.Peers(ch)
                    chatUI.AddMessage(fmt.Sprintf("Peers em %s: %v", ch, peers))
                } else {
                    chatUI.AddMessage("Uso: /list [#canal]")
                }
            case "/who":
                ch := activeChannel
                if len(parts) == 2 {
                    ch = parts[1]
                }
                users := []string{nick}
                for addr, chans := range peerChannels {
                    if chans[ch] {
                        users = append(users, peerNick[addr])
                    }
                }
                chatUI.AddMessage(fmt.Sprintf("Usuários em %s: %v", ch, users))
            case "/mode":
                if len(parts) < 3 {
                    chatUI.AddMessage("Uso: /mode <#canal> <modo>")
                    return
                }
                ch := parts[1]
                mode := parts[2]
                channelModes[ch] = mode
                for _, c := range conns {
                    fmt.Fprintf(c, "%s MODE %s %s\n", id, ch, mode)
                }
                chatUI.AddMessage(fmt.Sprintf("Modo de %s definido: %s", ch, mode))
            case "/ctcp":
                if len(parts) < 3 {
                    chatUI.AddMessage("Uso: /ctcp <nick> <comando>")
                    return
                }
                target := parts[1]
                cmdctcp := strings.Join(parts[2:], " ")
                envelope := "\x01" + cmdctcp + "\x01"
                for _, c := range conns {
                    fmt.Fprintf(c, "%s PRIVMSG %s :%s\n", id, target, envelope)
                }
                chatUI.AddMessage(fmt.Sprintf("CTCP enviado a %s: %s", target, cmdctcp))
            case "/dcc":
                chatUI.AddMessage("DCC não implementado")
            case "/alias":
                if len(parts) == 1 {
                    aliases := scriptEngine.ListAliases()
                    if len(aliases) == 0 {
                        chatUI.AddMessage("Nenhum alias definido")
                    } else {
                        for name, exp := range aliases {
                            chatUI.AddMessage(fmt.Sprintf("%s -> %s", name, exp))
                        }
                    }
                } else if parts[1] == "add" && len(parts) >= 4 {
                    name := parts[2]
                    expansion := strings.Join(parts[3:], " ")
                    if err := scriptEngine.AddAlias(name, expansion); err != nil {
                        chatUI.AddMessage(fmt.Sprintf("Erro ao adicionar alias: %v", err))
                    } else {
                        chatUI.AddMessage(fmt.Sprintf("Alias '%s' -> '%s' adicionado", name, expansion))
                    }
                } else if parts[1] == "rm" && len(parts) == 3 {
                    name := parts[2]
                    if err := scriptEngine.RemoveAlias(name); err != nil {
                        chatUI.AddMessage(fmt.Sprintf("Erro ao remover alias: %v", err))
                    } else {
                        chatUI.AddMessage(fmt.Sprintf("Alias '%s' removido", name))
                    }
                } else {
                    chatUI.AddMessage("Uso: /alias [add <name> <expansion> | rm <name>]")
                }
            case "/script":
                if len(parts) == 2 && parts[1] == "reload" {
                    if err := scriptEngine.Reload(); err != nil {
                        chatUI.AddMessage(fmt.Sprintf("Erro ao recarregar scripts: %v", err))
                    } else {
                        chatUI.AddMessage("Scripts recarregados")
                    }
                } else {
                    chatUI.AddMessage("Uso: /script reload")
                }
            case "/connect":
                if len(parts) != 2 {
                    chatUI.AddMessage("Uso: /connect <host:port>")
                    return
                }
                addr := parts[1]
                c, err := transport.Dial(addr, iceServers)
                if err != nil {
                    chatUI.AddMessage(fmt.Sprintf("Erro ao conectar %s: %v", addr, err))
                    return
                }
                addConn(c)
                chatUI.AddMessage(fmt.Sprintf("Conectado a %s", addr))
            case "/help":
                chatUI.AddMessage("Comandos: /nick, /user, /join, /part, /msg, /peers, /topic, /list, /who, /mode, /ctcp, /dcc, /alias, /script, /connect, /help, /quit, /exit")
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
            fmt.Fprintf(c, "%s PRIVMSG %s :%s\n", id, activeChannel, text)
        }
        chatUI.AddMessage(fmt.Sprintf("[%s] %s: %s", activeChannel, nick, text))
    })

    go func() {
        for pm := range recvCh {
            line := pm.Text
            idx := strings.Index(line, " ")
            if idx <= 0 {
                continue
            }
            id := line[:idx]
            body := line[idx+1:]

            // dedupe já vistas mensagens
            seenMu.Lock()
            if seen[id] {
                seenMu.Unlock()
                continue
            }
            seen[id] = true
            seenMu.Unlock()

            // retransmite para outros peers
            mu.Lock()
            for _, c := range conns {
                if c == pm.Conn {
                    continue
                }
                fmt.Fprintf(c, "%s\n", line)
            }
            mu.Unlock()

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
// getOutboundIP retrieves the preferred outbound IP of this machine
func getOutboundIP() string {
    conn, err := net.Dial("udp", "8.8.8.8:80")
    if err != nil {
        return "127.0.0.1"
    }
    defer conn.Close()
    localAddr := conn.LocalAddr().(*net.UDPAddr)
    return localAddr.IP.String()
}
