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
    nickname := "anon"
    channels := []string{"#general"}
    activeChannel := "#general"
    peerNick := make(map[string]string)
    peerChannels := make(map[string]map[string]bool)

    // Mapa para rastrear conexões por endereço
    peerConns := make(map[string]net.Conn)
    
    // track seen message IDs to avoid loops
    var seenMu sync.Mutex
    seen := make(map[string]bool)

    // Inicializa modos locais
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

    // Inicializa a interface TUI
    chatUI := ui.NewChatUI()
    chatUI.SetChannels(channels)

    // Função para sincronizar estado com um peer
    syncStateToPeer := func(c net.Conn) {
        // Envia NICK para identificação
        fmt.Fprintf(c, "NICK %s\r\n", nickname)
        
        // Envia JOIN para todos os canais em que o usuário está
        for _, channel := range channels {
            fmt.Fprintf(c, ":%s JOIN %s\r\n", nickname, channel)
        }
        
        // Envia uma mensagem de identificação explícita com todos os detalhes
        fmt.Fprintf(c, "IDENTIFY %s %s\r\n", nickname, strings.Join(channels, ","))
    }

    addConn := func(c net.Conn) {
        mu.Lock()
        defer mu.Unlock()
        conns = append(conns, c)
        
        // Adiciona ao mapa de conexões por endereço
        addr := c.RemoteAddr().String()
        peerConns[addr] = c
        
        // Sincroniza estado com o novo peer
        syncStateToPeer(c)
    }

    // addAndListen adds a connection and starts a goroutine to read incoming messages
    addAndListen := func(c net.Conn) {
        addConn(c)
        go func(c net.Conn) {
            r := bufio.NewReader(c)
            for {
                line, err := r.ReadString('\n')
                if err != nil {
                    mu.Lock()
                    // Remove do mapa de conexões
                    addr := c.RemoteAddr().String()
                    delete(peerConns, addr)
                    
                    // Remove da lista de conexões
                    for i, conn := range conns {
                        if conn == c {
                            conns = append(conns[:i], conns[i+1:]...)
                            break
                        }
                    }
                    mu.Unlock()
                    
                    // Registra a desconexão
                    nick := peerNick[addr]
                    if nick == "" {
                        nick = addr
                    }
                    chatUI.AddMessage(fmt.Sprintf("Peer desconectado: %s", nick))
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

    // Connect to bootstrap peers
    for _, peerAddr := range conf.Network.BootstrapPeers {
        if peerAddr == "" {
            continue
        }
        go func(addr string) {
            conn, err := transport.Dial(addr, iceServers)
            if err != nil {
                fmt.Printf("Erro ao conectar a %s: %v\n", addr, err)
                return
            }
            fmt.Printf("Conectado a %s\n", addr)
            
            // Adiciona e começa a ouvir a conexão
            addAndListen(conn)
        }(peerAddr)
    }

    // Conecta aos peers descobertos pelo serviço de descoberta
    go func() {
        ticker := time.NewTicker(10 * time.Second)
        defer ticker.Stop()
        
        for {
            select {
            case <-ticker.C:
                // Obtém peers do serviço de descoberta
                peers := discoveryService.GetPeers()
                
                for _, peer := range peers {
                    if peer.Addr == nil {
                        continue
                    }
                    
                    peerAddr := fmt.Sprintf("%s:%d", peer.Addr.IP.String(), peer.Addr.Port)
                    
                    // Verifica se já estamos conectados a este peer
                    mu.Lock()
                    alreadyConnected := false
                    for _, c := range conns {
                        if c.RemoteAddr().String() == peerAddr {
                            alreadyConnected = true
                            break
                        }
                    }
                    mu.Unlock()
                    
                    if !alreadyConnected {
                        go func(addr string) {
                            conn, err := transport.Dial(addr, iceServers)
                            if err != nil {
                                // Não loga erro para não poluir a interface
                                return
                            }
                            
                            // Adiciona e começa a ouvir a conexão
                            addAndListen(conn)
                            
                            // Envia uma mensagem de identificação explícita
                            fmt.Fprintf(conn, "IDENTIFY %s %s\r\n", nickname, strings.Join(channels, ","))
                            
                            // Solicita informações sobre o peer remoto
                            fmt.Fprintf(conn, "WHO\r\n")
                        }(peerAddr)
                    }
                }
            }
        }
    }()

    dccManager := dcc.NewManager("downloads")

    // Manipulador de input do usuário
    chatUI.SetInputHandler(func(msg string) {
        if strings.HasPrefix(msg, "/") {
            // Comando IRC
            cmd := strings.TrimPrefix(msg, "/")
            parts := strings.Fields(cmd)
            if len(parts) == 0 {
                return
            }

            switch strings.ToLower(parts[0]) {
            case "nick":
                if len(parts) > 1 {
                    oldNick := nickname
                    nickname = parts[1]
                    chatUI.AddMessage(fmt.Sprintf("Nickname alterado para: %s", nickname))
                    
                    // Anuncia mudança de nickname para todos os peers
                    mu.Lock()
                    for _, c := range conns {
                        fmt.Fprintf(c, ":%s NICK %s\r\n", oldNick, nickname)
                    }
                    mu.Unlock()
                }
            case "join":
                if len(parts) > 1 {
                    ch := parts[1]
                    if !strings.HasPrefix(ch, "#") {
                        ch = "#" + ch
                    }
                    if !contains(channels, ch) {
                        channels = append(channels, ch)
                    }
                    activeChannel = ch
                    chatUI.AddMessage(fmt.Sprintf("Entrou no canal: %s", ch))
                    chatUI.SetChannels(channels)
                    
                    // Anuncia entrada no canal para todos os peers
                    mu.Lock()
                    for _, c := range conns {
                        fmt.Fprintf(c, ":%s JOIN %s\r\n", nickname, ch)
                    }
                    mu.Unlock()
                }
            case "part":
                if len(parts) > 1 {
                    ch := parts[1]
                    if !strings.HasPrefix(ch, "#") {
                        ch = "#" + ch
                    }
                    channels = remove(channels, ch)
                    if activeChannel == ch && len(channels) > 0 {
                        activeChannel = channels[0]
                    }
                    chatUI.AddMessage(fmt.Sprintf("Saiu do canal: %s", ch))
                    chatUI.SetChannels(channels)
                    
                    // Anuncia saída do canal para todos os peers
                    mu.Lock()
                    for _, c := range conns {
                        fmt.Fprintf(c, ":%s PART %s\r\n", nickname, ch)
                    }
                    mu.Unlock()
                }
            case "msg":
                if len(parts) >= 3 {
                    target := parts[1]
                    content := strings.Join(parts[2:], " ")
                    content = addMessageID(content)
                    
                    mu.Lock()
                    for _, c := range conns {
                        fmt.Fprintf(c, ":%s PRIVMSG %s :%s\r\n", nickname, target, content)
                    }
                    mu.Unlock()
                    
                    chatUI.AddMessage(fmt.Sprintf("Mensagem privada para %s: %s", target, content))
                }
            case "who":
                if len(parts) > 1 {
                    // Comando WHO para um canal específico
                    channel := parts[1]
                    if !strings.HasPrefix(channel, "#") {
                        channel = "#" + channel
                    }
                    
                    chatUI.AddMessage(fmt.Sprintf("Usuários no canal %s:", channel))
                    
                    // Adiciona o usuário local se estiver no canal
                    if contains(channels, channel) {
                        chatUI.AddMessage(fmt.Sprintf("- %s (você)", nickname))
                    }
                    
                    // Lista peers no canal
                    mu.Lock()
                    for addr, chans := range peerChannels {
                        if chans[channel] {
                            nick := peerNick[addr]
                            if nick == "" {
                                nick = addr
                            }
                            chatUI.AddMessage(fmt.Sprintf("- %s", nick))
                        }
                    }
                    mu.Unlock()
                    
                    // Envia comando WHO para todos os peers para atualizar a lista
                    mu.Lock()
                    for _, c := range conns {
                        fmt.Fprintf(c, "WHO %s\r\n", channel)
                    }
                    mu.Unlock()
                } else {
                    // Comando WHO para listar todos os usuários conhecidos
                    chatUI.AddMessage("Usuários conhecidos na rede:")
                    chatUI.AddMessage(fmt.Sprintf("- %s (você)", nickname))
                    
                    // Lista todos os peers conhecidos
                    mu.Lock()
                    listedPeers := make(map[string]bool)
                    for addr, nick := range peerNick {
                        if nick != "" {
                            chatUI.AddMessage(fmt.Sprintf("- %s (%s)", nick, addr))
                            listedPeers[addr] = true
                        }
                    }
                    
                    // Adiciona peers do DHT que não foram listados
                    for _, node := range discoveryService.GetPeers() {
                        addr := node.Addr.String()
                        if !listedPeers[addr] && !listedPeers[node.InstanceID] {
                            chatUI.AddMessage(fmt.Sprintf("- %s (descoberto)", addr))
                        }
                    }
                    mu.Unlock()
                    
                    // Envia comando WHO para todos os peers para atualizar a lista
                    mu.Lock()
                    for _, c := range conns {
                        fmt.Fprintf(c, "WHO\r\n")
                    }
                    mu.Unlock()
                }
            case "info":
                if len(parts) > 1 {
                    // Comando INFO para obter informações de um usuário específico
                    target := parts[1]
                    
                    // Procura o usuário pelo nickname
                    mu.Lock()
                    var targetAddr string
                    for addr, nick := range peerNick {
                        if nick == target {
                            targetAddr = addr
                            break
                        }
                    }
                    mu.Unlock()
                    
                    if targetAddr != "" {
                        // Usuário encontrado, exibe informações
                        chatUI.AddMessage(fmt.Sprintf("Informações sobre %s:", target))
                        chatUI.AddMessage(fmt.Sprintf("Endereço: %s", targetAddr))
                        
                        // Lista canais em que o usuário está
                        mu.Lock()
                        if peerChannels[targetAddr] != nil {
                            var userChannels []string
                            for ch, joined := range peerChannels[targetAddr] {
                                if joined {
                                    userChannels = append(userChannels, ch)
                                }
                            }
                            if len(userChannels) > 0 {
                                chatUI.AddMessage(fmt.Sprintf("Canais: %s", strings.Join(userChannels, ", ")))
                            } else {
                                chatUI.AddMessage("Canais: nenhum")
                            }
                        } else {
                            chatUI.AddMessage("Canais: desconhecido")
                        }
                        mu.Unlock()
                        
                        // Envia comando INFO para o peer específico
                        mu.Lock()
                        if conn, ok := peerConns[targetAddr]; ok {
                            fmt.Fprintf(conn, "INFO %s\r\n", target)
                        }
                        mu.Unlock()
                    } else if target == nickname {
                        // Informações sobre o usuário local
                        chatUI.AddMessage(fmt.Sprintf("Informações sobre %s (você):", nickname))
                        chatUI.AddMessage(fmt.Sprintf("ID da instância: %s", discoveryService.GetInstanceID()))
                        chatUI.AddMessage(fmt.Sprintf("Porta: %d", discoveryService.GetPort()))
                        chatUI.AddMessage(fmt.Sprintf("Canais: %s", strings.Join(channels, ", ")))
                    } else {
                        // Usuário não encontrado
                        chatUI.AddMessage(fmt.Sprintf("Usuário %s não encontrado.", target))
                        
                        // Envia comando INFO para todos os peers para tentar encontrar o usuário
                        mu.Lock()
                        for _, c := range conns {
                            fmt.Fprintf(c, "INFO %s\r\n", target)
                        }
                        mu.Unlock()
                    }
                } else {
                    chatUI.AddMessage("Uso: /info <nickname>")
                }
            case "peers":
                mu.Lock()
                chatUI.AddMessage("Peers conectados:")
                for _, c := range conns {
                    addr := c.RemoteAddr().String()
                    nick := peerNick[addr]
                    if nick == "" {
                        nick = "desconhecido"
                    }
                    chatUI.AddMessage(fmt.Sprintf("- %s (%s)", nick, addr))
                }
                mu.Unlock()
            case "help":
                chatUI.AddMessage("Comandos disponíveis:")
                chatUI.AddMessage("/nick <novo>: define seu nickname")
                chatUI.AddMessage("/join <#canal>: entra em um canal")
                chatUI.AddMessage("/part <#canal>: sai de um canal")
                chatUI.AddMessage("/msg <canal> <mensagem>: envia mensagem ao canal")
                chatUI.AddMessage("/who [canal]: lista usuários na rede ou em um canal específico")
                chatUI.AddMessage("/info <nickname>: exibe informações sobre um usuário")
                chatUI.AddMessage("/peers: lista peers conectados")
                chatUI.AddMessage("/dcc send <arquivo> <usuário>: envia arquivo")
                chatUI.AddMessage("/dcclist: lista transferências")
                chatUI.AddMessage("/help: mostra esta ajuda")
                chatUI.AddMessage("/quit: sai da aplicação")
            case "dcc":
                if len(parts) >= 2 {
                    dccCmd := strings.Join(parts[1:], " ")
                    handleDCCCommand(dccCmd, dccManager, chatUI)
                }
            case "dcclist":
                if dccManager != nil {
                    transfers := dccManager.ListTransfers()
                    if len(transfers) == 0 {
                        chatUI.AddMessage("Nenhuma transferência ativa")
                    } else {
                        chatUI.AddMessage("Transferências ativas:")
                        for _, t := range transfers {
                            chatUI.AddMessage(fmt.Sprintf("- %s -> %s: %s (%d bytes)", t.Sender, t.Receiver, t.Filename, t.Size))
                        }
                    }
                }
            case "quit":
                chatUI.AddMessage("Saindo da aplicação...")
                os.Exit(0)
            default:
                chatUI.AddMessage(fmt.Sprintf("Comando desconhecido: %s. Use /help para ajuda.", parts[0]))
            }
        } else {
            // Mensagem normal para o canal ativo
            if activeChannel == "" {
                chatUI.AddMessage("Erro: nenhum canal ativo. Use /join para entrar em um canal.")
                return
            }

            // Adiciona ID único à mensagem para rastreamento
            msgWithID := addMessageID(msg)
            
            // Registra o ID como visto para evitar loops
            id := extractMessageID(msgWithID)
            if id != "" {
                seenMu.Lock()
                seen[id] = true
                seenMu.Unlock()
            }

            // Mostra localmente
            chatUI.AddMessage(fmt.Sprintf("%s: %s", nickname, msg))
            
            // Envia para todos os peers conectados usando o formato correto de IRC
            mu.Lock()
            for _, c := range conns {
                // Formato IRC: :<origem> PRIVMSG <destino> :<mensagem>
                fmt.Fprintf(c, ":%s PRIVMSG %s :%s\r\n", nickname, activeChannel, msgWithID)
                
                // Força o envio imediato (flush)
                if flusher, ok := c.(interface{ Flush() error }); ok {
                    flusher.Flush()
                }
            }
            mu.Unlock()
        }
    })

    // Loop principal de processamento de mensagens
    go func() {
        for pm := range recvCh {
            body := pm.Text
            parts := strings.Fields(body)
            if len(parts) == 0 {
                continue
            }

            // Verifica se é um comando IRC com prefixo (ex: ":user COMMAND")
            var prefix string
            var command string
            
            if strings.HasPrefix(parts[0], ":") {
                prefix = strings.TrimPrefix(parts[0], ":")
                if len(parts) > 1 {
                    command = strings.ToUpper(parts[1])
                    parts = append([]string{parts[0]}, parts[2:]...)
                }
            } else {
                command = strings.ToUpper(parts[0])
                parts = parts[1:]
            }

            switch command {
            case "NICK":
                if len(parts) > 0 {
                    addr := pm.Conn.RemoteAddr().String()
                    oldNick := peerNick[addr]
                    newNick := parts[0]
                    peerNick[addr] = newNick
                    chatUI.AddMessage(fmt.Sprintf("%s agora é conhecido como %s", oldNick, newNick))
                }
            case "JOIN":
                if len(parts) > 0 {
                    channel := parts[0]
                    addr := pm.Conn.RemoteAddr().String()
                    nick := peerNick[addr]
                    if nick == "" {
                        nick = addr
                    }
                    
                    // Adiciona o peer ao canal
                    if peerChannels[addr] == nil {
                        peerChannels[addr] = make(map[string]bool)
                    }
                    peerChannels[addr][channel] = true
                    
                    chatUI.AddMessage(fmt.Sprintf("%s entrou no canal %s", nick, channel))
                }
            case "PART":
                if len(parts) > 0 {
                    channel := parts[0]
                    addr := pm.Conn.RemoteAddr().String()
                    nick := peerNick[addr]
                    if nick == "" {
                        nick = addr
                    }
                    
                    // Remove o peer do canal
                    if peerChannels[addr] != nil {
                        delete(peerChannels[addr], channel)
                    }
                    
                    chatUI.AddMessage(fmt.Sprintf("%s saiu do canal %s", nick, channel))
                }
            case "WHO":
                // Responde ao comando WHO com informações sobre os usuários
                addr := pm.Conn.RemoteAddr().String()
                senderNick := peerNick[addr]
                if senderNick == "" {
                    senderNick = addr
                }
                
                if len(parts) > 0 {
                    // WHO para um canal específico
                    channel := parts[0]
                    
                    // Verifica se o usuário local está no canal
                    if contains(channels, channel) {
                        // Envia informação sobre o usuário local
                        fmt.Fprintf(pm.Conn, ":%s WHO_REPLY %s %s\r\n", nickname, channel, nickname)
                    }
                } else {
                    // WHO geral - envia informação sobre o usuário local
                    fmt.Fprintf(pm.Conn, ":%s WHO_REPLY %s\r\n", nickname, nickname)
                }
            case "WHO_REPLY":
                // Recebe resposta de um comando WHO
                if len(parts) >= 1 {
                    userNick := parts[0]
                    
                    // Se for WHO para um canal específico
                    if len(parts) >= 2 {
                        channel := parts[1]
                        chatUI.AddMessage(fmt.Sprintf("- %s (respondeu ao WHO para %s)", userNick, channel))
                    } else {
                        // WHO geral
                        chatUI.AddMessage(fmt.Sprintf("- %s (respondeu ao WHO)", userNick))
                    }
                }
            case "INFO":
                // Responde ao comando INFO com informações detalhadas
                if len(parts) > 0 {
                    requestedNick := parts[0]
                    
                    // Verifica se é o usuário local
                    if requestedNick == nickname {
                        // Envia informações sobre o usuário local
                        channelsStr := strings.Join(channels, ",")
                        fmt.Fprintf(pm.Conn, ":%s INFO_REPLY %s %s %d %s\r\n", 
                            nickname, nickname, discoveryService.GetInstanceID(), 
                            discoveryService.GetPort(), channelsStr)
                    }
                }
            case "INFO_REPLY":
                // Recebe resposta de um comando INFO
                if len(parts) >= 4 {
                    userNick := parts[0]
                    instanceID := parts[1]
                    port := parts[2]
                    channels := "nenhum"
                    if len(parts) >= 5 {
                        channels = parts[3]
                    }
                    
                    chatUI.AddMessage(fmt.Sprintf("Informações adicionais sobre %s:", userNick))
                    chatUI.AddMessage(fmt.Sprintf("ID da instância: %s", instanceID))
                    chatUI.AddMessage(fmt.Sprintf("Porta: %s", port))
                    chatUI.AddMessage(fmt.Sprintf("Canais: %s", channels))
                }
            case "PRIVMSG":
                if len(parts) >= 2 {
                    target := parts[0]
                    raw := strings.Join(parts[1:], " ")
                    msg := strings.TrimPrefix(raw, ":")
                    
                    // Obtém o remetente
                    var sender string
                    if prefix != "" {
                        sender = prefix
                    } else {
                        addr := pm.Conn.RemoteAddr().String()
                        sender = peerNick[addr]
                        if sender == "" {
                            sender = addr
                        }
                    }
                    
                    // CTCP handling
                    if strings.HasPrefix(msg, "\x01") && strings.HasSuffix(msg, "\x01") {
                        ctcp := strings.TrimSuffix(strings.TrimPrefix(msg, "\x01"), "\x01")
                        chatUI.AddMessage(fmt.Sprintf("CTCP de %s: %s", sender, ctcp))
                        break
                    }
                    
                    // Verifica se já vimos esta mensagem (para evitar loops)
                    msgID := extractMessageID(msg)
                    if msgID != "" {
                        seenMu.Lock()
                        alreadySeen := seen[msgID]
                        if !alreadySeen {
                            seen[msgID] = true
                        }
                        seenMu.Unlock()
                        
                        if alreadySeen {
                            // Já vimos esta mensagem, ignora
                            break
                        }
                    }
                    
                    // Exibe a mensagem se for para o canal ativo ou um privado para o usuário
                    if target == activeChannel || target == nickname {
                        // Remove o ID da mensagem antes de exibir
                        displayMsg := removeMessageID(msg)
                        
                        // Exibe a mensagem na interface
                        chatUI.AddMessage(fmt.Sprintf("%s: %s", sender, displayMsg))
                    }
                    
                    // Propaga a mensagem para outros peers
                    if msgID != "" {
                        mu.Lock()
                        for _, c := range conns {
                            if c != pm.Conn { // Não envie de volta para o remetente
                                var fromPrefix string
                                if prefix != "" {
                                    fromPrefix = ":" + prefix
                                } else {
                                    fromPrefix = ":" + sender
                                }
                                // Envia a mensagem com o formato correto
                                fmt.Fprintf(c, "%s PRIVMSG %s :%s\r\n", fromPrefix, target, msg)
                                
                                // Força o envio imediato (flush)
                                if flusher, ok := c.(interface{ Flush() error }); ok {
                                    flusher.Flush()
                                }
                            }
                        }
                        mu.Unlock()
                    }
                }
            case "DCC":
                if len(parts) >= 1 {
                    dccCmd := strings.Join(parts, " ")
                    handleDCCCommand(dccCmd, dccManager, chatUI)
                }
            case "MODE":
                if len(parts) >= 2 {
                    ch := parts[0]
                    mode := parts[1]
                    channelModes[ch] = mode
                    chatUI.AddMessage(fmt.Sprintf("Modo de %s agora é %s", ch, mode))
                }
            case "IDENTIFY":
                if len(parts) >= 2 {
                    nick := parts[0]
                    channelsStr := parts[1]
                    addr := pm.Conn.RemoteAddr().String()
                    peerNick[addr] = nick
                    chatUI.AddMessage(fmt.Sprintf("%s se identificou como %s", addr, nick))
                    
                    // Atualiza lista de canais do peer
                    peerChannels[addr] = make(map[string]bool)
                    for _, ch := range strings.Split(channelsStr, ",") {
                        peerChannels[addr][ch] = true
                    }
                }
            case "PING":
                // Responde ao PING para manter a conexão ativa
                if len(parts) > 0 {
                    // Responde com PONG
                    fmt.Fprintf(pm.Conn, "PONG %s\r\n", nickname)
                    
                    // Aproveita para sincronizar estado
                    syncStateToPeer(pm.Conn)
                }
            case "PONG":
                // Recebe resposta de PING, não precisa fazer nada
                // Apenas registra que o peer está ativo
                if len(parts) > 0 {
                    addr := pm.Conn.RemoteAddr().String()
                    
                    // Atualiza o nickname se necessário
                    if peerNick[addr] == "" {
                        peerNick[addr] = parts[0]
                    }
                }
            default:
                chatUI.AddMessage(body)
            }
        }
    }()

    // Atualiza periodicamente a lista de peers e canais na UI
    go func() {
        ticker := time.NewTicker(5 * time.Second)
        defer ticker.Stop()
        
        for {
            select {
            case <-ticker.C:
                // Atualiza lista de peers conectados
                mu.Lock()
                var peersList []string
                for _, conn := range conns {
                    addr := conn.RemoteAddr().String()
                    nick, ok := peerNick[addr]
                    if ok {
                        peersList = append(peersList, nick)
                    } else {
                        peersList = append(peersList, addr)
                    }
                }
                mu.Unlock()
                
                // Adiciona peers do DHT
                for _, node := range discoveryService.GetPeers() {
                    addr := node.Addr.String()
                    if !contains(peersList, addr) && !contains(peersList, node.InstanceID) {
                        peersList = append(peersList, addr)
                    }
                }
                
                // Atualiza UI
                chatUI.SetPeers(peersList)
                chatUI.SetChannels(channels)
                
                // Envia PING para todos os peers para manter conexões ativas
                mu.Lock()
                for _, c := range conns {
                    fmt.Fprintf(c, "PING %s\r\n", nickname)
                }
                mu.Unlock()
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
            chatUI.AddMessage(fmt.Sprintf("- %s -> %s: %s (%d bytes)", t.Sender, t.Receiver, t.Filename, t.Size))
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

// extractMessageID extrai o ID de mensagem de uma mensagem PRIVMSG
func extractMessageID(msg string) string {
    // Procura por um ID de mensagem no formato [ID:xxxx]
    if len(msg) < 10 {
        return ""
    }
    
    // Verifica se a mensagem contém um ID
    idStart := strings.Index(msg, "[ID:")
    if idStart == -1 {
        return ""
    }
    
    // Extrai o ID
    idEnd := strings.Index(msg[idStart:], "]")
    if idEnd == -1 {
        return ""
    }
    
    // Retorna o ID sem os delimitadores
    return msg[idStart+4 : idStart+idEnd]
}

// addMessageID adiciona um ID único a uma mensagem
func addMessageID(msg string) string {
    id := newID()
    return fmt.Sprintf("%s [ID:%s]", msg, id)
}

// removeMessageID remove o ID de uma mensagem
func removeMessageID(msg string) string {
    idStart := strings.Index(msg, "[ID:")
    if idStart != -1 {
        idEnd := strings.Index(msg[idStart:], "]")
        if idEnd != -1 {
            // Remove o ID e espaços extras
            return strings.TrimSpace(msg[:idStart] + msg[idStart+idEnd+1:])
        }
    }
    return msg
}
