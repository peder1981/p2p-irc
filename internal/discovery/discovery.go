// Package discovery implementa o serviço de descoberta de peers
// Copyright (c) 2025 Peder Munksgaard
// Licenciado sob MIT License

package discovery

import (
    "bufio"
    "context"
    "crypto/rand"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "net"
    "strings"
    "sync"
    "time"

    "github.com/grandcat/zeroconf"
    "github.com/huin/goupnp/dcps/internetgateway2"
    "github.com/peder1981/p2p-irc/internal/dht"
)

const (
    serviceType = "_p2p-irc._tcp"
    domain      = "local."
    cacheExpiry = 1 * time.Hour
    maxRetries  = 3
    retryDelay  = 5 * time.Second
    // Rate limiting
    maxReconnectRate = 0.2 // 1 reconexão a cada 5 segundos
    burstLimit       = 3   // permite burst de até 3 reconexões
)

// Metrics contém métricas do sistema de descoberta
type Metrics struct {
    ActivePeers        int
    TotalDiscovered    int64
    LastDiscovery      time.Time
    FailedAttempts     int64
    Uptime            time.Duration
    StartTime         time.Time
    BrowseAttempts    int64
    BrowseErrors      int64
    LastBrowseError   time.Time
    ReconnectCount    int64
    LastReconnect     time.Time
    TotalPeersFound   int64
    PeersIgnored      int64
}

// Discovery é o serviço de descoberta de peers
type Discovery struct {
    serviceName    string
    port          int
    instanceID    string
    dht           *dht.RoutingTable
    zeroconf      *zeroconf.Server
    resolver      *zeroconf.Resolver
    mu            sync.RWMutex
    ctx           context.Context
    cancel        context.CancelFunc
    metrics       Metrics
    retryCount    int
    lastRetry     time.Time
    // Rate limiting para reconexões
    lastReconnectTime time.Time
    reconnectCount    int
    // Gerenciamento de conexões
    connections    map[string]*PeerConnection
    connMu         sync.RWMutex
    // Canais
    channels       map[string]bool
    channelMu      sync.RWMutex
    // Callbacks
    messageHandler func(msg Message)
    // Cache de mensagens processadas para evitar loops
    processedMessages     map[string]bool // Cache de IDs de mensagens já processadas
    processedMessagesMu   sync.RWMutex    // Mutex para proteger o cache
    processedMessagesTime map[string]time.Time // Timestamp de quando a mensagem foi processada
    lastCleanupTime      time.Time
}

// New cria um novo serviço de descoberta
func New(bootstrapPeers []string, port int) (*Discovery, error) {
    ctx, cancel := context.WithCancel(context.Background())

    // Gera um ID único para esta instância
    instanceID := generateInstanceID()

    // Tenta encontrar uma porta disponível
    if port == 0 {
        port = findAvailablePort(49152, 65535) // Portas dinâmicas/privadas
    }

    // Gera o ID do nó local
    nodeID := dht.GenerateNodeID(fmt.Sprintf("%s:%d", instanceID, port))

    // Cria o resolver uma única vez
    resolver, err := zeroconf.NewResolver(nil)
    if err != nil {
        cancel()
        return nil, fmt.Errorf("erro ao criar resolver: %w", err)
    }

    d := &Discovery{
        serviceName: "p2p-irc",
        port:        port,
        instanceID:  instanceID,
        dht:         dht.NewRoutingTable(nodeID, 20),
        resolver:    resolver,
        ctx:         ctx,
        cancel:      cancel,
        metrics: Metrics{
            StartTime: time.Now(),
        },
        connections:          make(map[string]*PeerConnection),
        channels:             make(map[string]bool),
        processedMessages:     make(map[string]bool),
        processedMessagesTime: make(map[string]time.Time),
    }

    return d, nil
}

// findAvailablePort encontra uma porta disponível no intervalo especificado
func findAvailablePort(start, end int) int {
    for port := start; port <= end; port++ {
        addr := fmt.Sprintf(":%d", port)
        listener, err := net.Listen("tcp", addr)
        if err == nil {
            listener.Close()
            return port
        }
    }
    return start // Retorna a primeira porta se nenhuma estiver disponível
}

// canReconnect implementa rate limiting simples para reconexões
func (d *Discovery) canReconnect() bool {
    now := time.Now()
    
    // Se é a primeira reconexão ou passou tempo suficiente desde a última
    if d.reconnectCount == 0 || now.Sub(d.lastReconnectTime) >= time.Second*5 {
        d.reconnectCount = 1
        d.lastReconnectTime = now
        return true
    }

    // Permite burst de até 3 reconexões rápidas
    if d.reconnectCount < burstLimit && now.Sub(d.lastReconnectTime) < time.Second*5 {
        d.reconnectCount++
        return true
    }

    // Rate limiting em efeito
    return false
}

// discoverPeers procura continuamente por novos peers
func (d *Discovery) discoverPeers() {
    done := make(chan struct{})

    fmt.Printf("[INFO] Iniciando serviço de descoberta de peers (instanceID: %s, porta: %d)\n", 
        d.instanceID, d.port)

    // Goroutine para procurar peers
    go func() {
        defer close(done)
        
        // Primeira busca imediata
        d.performBrowse()
        
        // Ticker para buscas periódicas
        // Inicialmente com intervalo curto para descobrir peers rapidamente
        ticker := time.NewTicker(5 * time.Second)
        defer ticker.Stop()
        
        // Contador para ajustar o intervalo
        count := 0
        
        for {
            select {
            case <-d.ctx.Done():
                return
            case <-ticker.C:
                count++
                
                // Após 5 buscas, aumenta o intervalo para reduzir o tráfego
                if count == 5 {
                    ticker.Stop()
                    ticker = time.NewTicker(30 * time.Second)
                }
                
                // Realiza a busca
                d.performBrowse()
            }
        }
    }()

    <-done // Espera pelo sinal de encerramento
}

// performBrowse realiza uma única busca por peers
func (d *Discovery) performBrowse() {
    d.mu.Lock()
    d.metrics.BrowseAttempts++
    d.mu.Unlock()

    // Cria um novo resolver para cada busca
    resolver, err := zeroconf.NewResolver(nil)
    if err != nil {
        d.mu.Lock()
        d.metrics.BrowseErrors++
        d.metrics.LastBrowseError = time.Now()
        d.mu.Unlock()
        fmt.Printf("[ERRO] Falha ao criar resolver: %v (instanceID: %s)\n", err, d.instanceID)
        return
    }

    // Cria um novo contexto para esta busca com timeout reduzido para 2 segundos
    browseCtx, browseCancel := context.WithTimeout(d.ctx, time.Second*2)
    defer browseCancel()
    
    // Canal para receber entradas
    entries := make(chan *zeroconf.ServiceEntry)
    
    // Canal para sinalizar término
    browseDone := make(chan struct{})
    
    // Goroutine para processar entradas
    go func() {
        defer close(browseDone)
        
        for entry := range entries {
            // Ignora própria instância
            peerInstanceID := ""
            for _, txt := range entry.Text {
                if len(txt) > 9 && txt[:9] == "instance=" {
                    peerInstanceID = txt[9:]
                    break
                }
            }

            if peerInstanceID == d.instanceID {
                d.mu.Lock()
                d.metrics.PeersIgnored++
                d.mu.Unlock()
                continue
            }

            // Atualiza métricas
            d.mu.Lock()
            d.metrics.TotalPeersFound++
            d.metrics.LastDiscovery = time.Now()
            d.mu.Unlock()

            // Processa todos os IPs do peer
            for _, ip := range entry.AddrIPv4 {
                if !ip.IsLoopback() {
                    // Gera o ID do nó
                    nodeID := dht.GenerateNodeID(fmt.Sprintf("%s:%d", peerInstanceID, entry.Port))
                    
                    // Verifica se o peer já existe no DHT
                    isNewNode := !d.nodeExists(nodeID)
                    
                    // Adiciona ou atualiza o peer
                    node := dht.Node{
                        ID:         nodeID,
                        Addr:       &net.UDPAddr{IP: ip, Port: entry.Port},
                        LastSeen:   time.Now(),
                        InstanceID: peerInstanceID,
                    }
                    
                    d.dht.AddNode(node)
                    
                    // Log apenas para novos peers e apenas uma vez
                    if isNewNode {
                        fmt.Printf("[INFO] Novo peer encontrado: %s:%d (instanceID: %s)\n", 
                            ip.String(), entry.Port, peerInstanceID)
                    }
                }
            }
        }
    }()
    
    // Inicia o browse
    err = resolver.Browse(browseCtx, serviceType, domain, entries)
    if err != nil {
        d.mu.Lock()
        d.metrics.BrowseErrors++
        d.metrics.LastBrowseError = time.Now()
        d.mu.Unlock()
        fmt.Printf("[ERRO] Falha ao procurar serviços: %v (instanceID: %s)\n", err, d.instanceID)
        close(entries)
        <-browseDone
        return
    }
    
    // Aguarda o timeout sem logs
    select {
    case <-browseCtx.Done():
    case <-browseDone:
    }
    
    // Não fechamos o canal entries aqui, deixamos isso para o resolver
}

// nodeExists verifica se um nó existe no DHT
func (d *Discovery) nodeExists(nodeID dht.NodeID) bool {
    return d.dht.NodeExists(nodeID)
}

// Stop para todos os serviços de descoberta
func (d *Discovery) Stop() {
    d.cancel()
    if d.zeroconf != nil {
        d.zeroconf.Shutdown()
    }
}

// Start inicia o serviço de descoberta
func (d *Discovery) Start() error {
    var err error

    // Registra o serviço no mDNS
    err = d.startMDNS()
    if err != nil {
        return fmt.Errorf("erro ao iniciar mDNS: %w", err)
    }

    // Configura UPnP
    if err := d.setupUPnP(); err != nil {
        fmt.Printf("Aviso: falha ao configurar UPnP: %v\n", err)
    }

    // Inicia descoberta de peers
    go d.discoverPeers()

    // Inicia limpeza de cache
    go d.cleanCache()

    // Inicia o timer para limpar o cache de mensagens processadas
    go d.startCacheCleanupTimer()

    return nil
}

// startMDNS inicia o serviço mDNS com retry
func (d *Discovery) startMDNS() error {
    var err error
    for i := 0; i < maxRetries; i++ {
        // Gera um ID único para esta instância
        txt := []string{
            fmt.Sprintf("port=%d", d.port),
            fmt.Sprintf("instance=%s", d.instanceID),
        }

        // Registra o serviço em todas as interfaces
        interfaces := getNetInterfaces()
        d.zeroconf, err = zeroconf.Register(
            "p2p-irc-"+d.instanceID,
            serviceType,
            domain,
            d.port,
            txt,
            interfaces,
        )

        if err == nil {
            fmt.Printf("[INFO] Serviço registrado em %d interfaces (instanceID: %s, porta: %d)\n", 
                len(interfaces), d.instanceID, d.port)
            return nil
        }

        d.metrics.FailedAttempts++
        d.lastRetry = time.Now()
        d.retryCount++

        if i < maxRetries-1 {
            time.Sleep(retryDelay)
        }
    }

    return fmt.Errorf("falha após %d tentativas: %w", maxRetries, err)
}

// getNetInterfaces retorna todas as interfaces de rede não loopback
func getNetInterfaces() []net.Interface {
    var interfaces []net.Interface
    ifaces, err := net.Interfaces()
    if err != nil {
        return []net.Interface{}
    }

    for _, iface := range ifaces {
        // Ignora interfaces down ou loopback
        if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
            continue
        }

        interfaces = append(interfaces, iface)
    }

    return interfaces
}

// getAllLocalIPs retorna todos os IPs locais não loopback
func getAllLocalIPs() []net.IP {
    var ips []net.IP
    ifaces, err := net.Interfaces()
    if err != nil {
        return []net.IP{net.ParseIP("127.0.0.1")}
    }

    for _, iface := range ifaces {
        // Ignora interfaces down ou loopback
        if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
            continue
        }

        addrs, err := iface.Addrs()
        if err != nil {
            continue
        }

        for _, addr := range addrs {
            switch v := addr.(type) {
            case *net.IPNet:
                if !v.IP.IsLoopback() {
                    if ip4 := v.IP.To4(); ip4 != nil {
                        ips = append(ips, ip4)
                    }
                }
            case *net.IPAddr:
                if !v.IP.IsLoopback() {
                    if ip4 := v.IP.To4(); ip4 != nil {
                        ips = append(ips, ip4)
                    }
                }
            }
        }
    }

    // Se nenhum IP foi encontrado, usa localhost
    if len(ips) == 0 {
        ips = append(ips, net.ParseIP("127.0.0.1"))
    }

    return ips
}

// cleanCache remove peers expirados periodicamente
func (d *Discovery) cleanCache() {
    ticker := time.NewTicker(5 * time.Minute)
    defer ticker.Stop()

    for {
        select {
        case <-d.ctx.Done():
            return
        case <-ticker.C:
            d.mu.Lock()
            now := time.Now()
            activePeers := 0

            peers := d.dht.FindClosestNodes(d.dht.LocalID(), 100)
            for _, peer := range peers {
                if now.Sub(peer.LastSeen) > cacheExpiry {
                    d.dht.RemoveNode(peer.ID)
                } else {
                    activePeers++
                }
            }

            d.metrics.ActivePeers = activePeers
            d.mu.Unlock()
        }
    }
}

// GetMetrics retorna as métricas atuais
func (d *Discovery) GetMetrics() Metrics {
    d.mu.RLock()
    defer d.mu.RUnlock()

    d.metrics.Uptime = time.Since(d.metrics.StartTime)
    return d.metrics
}

// setupUPnP configura o mapeamento de porta usando UPnP
func (d *Discovery) setupUPnP() error {
    clients, _, err := internetgateway2.NewWANIPConnection1Clients()
    if err != nil {
        return err
    }
    if len(clients) == 0 {
        return fmt.Errorf("nenhum cliente UPnP encontrado")
    }

    // Tenta mapear a porta em cada cliente
    for _, c := range clients {
        err := c.AddPortMapping("", uint16(d.port), "TCP", uint16(d.port), getLocalIP(), true, "p2p-irc", 0)
        if err == nil {
            return nil
        }
    }

    return fmt.Errorf("falha ao mapear porta em todos os clientes")
}

// GetPeers retorna todos os peers conhecidos pelo serviço de descoberta
func (d *Discovery) GetPeers() []dht.Node {
    d.mu.RLock()
    defer d.mu.RUnlock()
    
    var peers []dht.Node
    for _, bucket := range d.dht.GetBuckets() {
        peers = append(peers, bucket.GetNodes()...)
    }
    
    return peers
}

// GetKnownPeers retorna os endereços de todos os peers conhecidos
func (d *Discovery) GetKnownPeers() []string {
    d.mu.RLock()
    defer d.mu.RUnlock()
    
    var addresses []string
    for _, bucket := range d.dht.GetBuckets() {
        for _, node := range bucket.GetNodes() {
            addr := node.Addr.String()
            if addr != "" {
                addresses = append(addresses, addr)
            }
        }
    }
    
    return addresses
}

// generateInstanceID gera um ID único para esta instância
func generateInstanceID() string {
    b := make([]byte, 16)
    if _, err := rand.Read(b); err != nil {
        panic(err)
    }
    return hex.EncodeToString(b)
}

// getLocalIP retorna o IP local da máquina
func getLocalIP() string {
    addrs, err := net.InterfaceAddrs()
    if err != nil {
        return "127.0.0.1"
    }
    for _, addr := range addrs {
        if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
            if ipnet.IP.To4() != nil {
                return ipnet.IP.String()
            }
        }
    }
    return "127.0.0.1"
}

// GetInstanceID retorna o ID da instância
func (d *Discovery) GetInstanceID() string {
    return d.instanceID
}

// GetPort retorna a porta em uso
func (d *Discovery) GetPort() int {
    return d.port
}

// SetMessageHandler define a função de callback para processar mensagens recebidas
func (d *Discovery) SetMessageHandler(handler func(msg Message)) {
    d.messageHandler = handler
}

// JoinChannel adiciona um canal à lista de canais
func (d *Discovery) JoinChannel(channel string) {
    d.channelMu.Lock()
    d.channels[channel] = true
    d.channelMu.Unlock()
    
    fmt.Printf("[INFO] Entrando no canal %s\n", channel)
    
    // Notifica todos os peers que entramos no canal
    joinMsg := Message{
        Type:      "join",
        Sender:    d.instanceID,
        Channel:   channel,
        Timestamp: time.Now(),
    }
    
    // Envia para todos os peers
    d.BroadcastToAllPeers(joinMsg)
}

// PartChannel remove um canal da lista de canais
func (d *Discovery) PartChannel(channel string) {
    d.channelMu.Lock()
    delete(d.channels, channel)
    d.channelMu.Unlock()
    
    fmt.Printf("[INFO] Saindo do canal %s\n", channel)
    
    // Notifica todos os peers que saímos do canal
    partMsg := Message{
        Type:      "part",
        Sender:    d.instanceID,
        Channel:   channel,
        Timestamp: time.Now(),
    }
    
    // Envia para todos os peers
    d.BroadcastToAllPeers(partMsg)
}

// BroadcastToAllPeers envia uma mensagem para todos os peers conectados
func (d *Discovery) BroadcastToAllPeers(msg Message) {
    d.connMu.RLock()
    defer d.connMu.RUnlock()
    
    fmt.Printf("[INFO] Broadcast de mensagem tipo=%s para todos os peers (%d conectados)\n", 
        msg.Type, len(d.connections))
    
    // Conta quantos peers receberam a mensagem
    sentCount := 0
    failCount := 0
    
    for addr, peerConn := range d.connections {
        // Envia a mensagem
        if err := peerConn.SendMessage(msg); err != nil {
            fmt.Printf("[ERRO] Falha ao enviar mensagem para peer %s: %v\n", addr, err)
            failCount++
        } else {
            fmt.Printf("[DEBUG] Mensagem enviada com sucesso para peer %s\n", addr)
            sentCount++
        }
    }
    
    fmt.Printf("[INFO] Mensagem enviada para %d peers (falhas: %d)\n", sentCount, failCount)
}

// IsInChannel verifica se estamos em um canal
func (d *Discovery) IsInChannel(channel string) bool {
    d.channelMu.RLock()
    defer d.channelMu.RUnlock()
    
    return d.channels[channel]
}

// GetChannels retorna todos os canais em que estamos
func (d *Discovery) GetChannels() []string {
    d.channelMu.RLock()
    defer d.channelMu.RUnlock()
    
    channels := make([]string, 0, len(d.channels))
    for channel := range d.channels {
        channels = append(channels, channel)
    }
    return channels
}

// ConnectToPeer estabelece uma conexão com um peer
func (d *Discovery) ConnectToPeer(addr string) error {
    // Verifica se já estamos conectados a este peer
    d.connMu.RLock()
    if _, exists := d.connections[addr]; exists {
        d.connMu.RUnlock()
        return nil // Já conectado
    }
    d.connMu.RUnlock()
    
    fmt.Printf("[INFO] Tentando conectar ao peer %s\n", addr)
    
    // Estabelece a conexão
    conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
    if err != nil {
        return fmt.Errorf("erro ao conectar ao peer %s: %w", addr, err)
    }
    
    // Cria a conexão de peer
    peerConn := NewPeerConnection(conn, "")
    
    // Adiciona à lista de conexões
    d.connMu.Lock()
    d.connections[addr] = peerConn
    d.connMu.Unlock()
    
    // Inicia a leitura de mensagens em uma goroutine
    go d.readMessages(addr, peerConn)
    
    // Inicia o ping periódico
    go d.pingPeer(addr, peerConn)
    
    fmt.Printf("[INFO] Conectado ao peer %s\n", addr)
    
    // Envia informações sobre os canais em que estamos
    d.channelMu.RLock()
    for channel := range d.channels {
        joinMsg := Message{
            Type:      "join",
            Sender:    d.instanceID,
            Channel:   channel,
            Timestamp: time.Now(),
        }
        peerConn.SendMessage(joinMsg)
        fmt.Printf("[INFO] Enviando informação de canal %s para peer %s\n", channel, addr)
    }
    d.channelMu.RUnlock()
    
    return nil
}

// readMessages lê mensagens de um peer
func (d *Discovery) readMessages(addr string, peerConn *PeerConnection) {
    defer func() {
        // Remove a conexão quando a goroutine terminar
        d.connMu.Lock()
        delete(d.connections, addr)
        d.connMu.Unlock()
        
        peerConn.Conn.Close()
        fmt.Printf("[INFO] Desconectado do peer %s\n", addr)
    }()
    
    // Buffer para ler as mensagens
    reader := bufio.NewReader(peerConn.Conn)
    
    for {
        // Verifica se o contexto foi cancelado
        select {
        case <-d.ctx.Done():
            return
        default:
            // Continua
        }
        
        // Define um timeout para a leitura
        peerConn.Conn.SetReadDeadline(time.Now().Add(30 * time.Second))
        
        // Lê uma linha (mensagem delimitada por \n)
        line, err := reader.ReadString('\n')
        if err != nil {
            if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
                // Timeout, verifica se o peer ainda está conectado
                if time.Since(peerConn.LastPing) > 60*time.Second {
                    fmt.Printf("[INFO] Timeout ao ler do peer %s\n", addr)
                    return
                }
                continue
            }
            
            fmt.Printf("[INFO] Erro ao ler do peer %s: %v\n", addr, err)
            return
        }
        
        // Remove o delimitador
        line = strings.TrimSpace(line)
        if len(line) == 0 {
            continue
        }
        
        // Deserializa a mensagem
        var msg Message
        if err := json.Unmarshal([]byte(line), &msg); err != nil {
            fmt.Printf("[ERRO] Erro ao deserializar mensagem do peer %s: %v\n", addr, err)
            continue
        }
        
        fmt.Printf("[INFO] Mensagem recebida do peer %s: tipo=%s, canal=%s, conteúdo=%s\n", 
            addr, msg.Type, msg.Channel, msg.Content)
        
        // Verifica se a mensagem já foi processada (usando o mecanismo de cache global)
        if msg.MessageID != "" && d.isMessageProcessed(msg.MessageID) {
            fmt.Printf("[DEBUG] Mensagem %s já foi processada, ignorando\n", msg.MessageID)
            continue
        }
        
        // Adiciona a mensagem ao cache de processadas
        if msg.MessageID != "" {
            d.processedMessagesMu.Lock()
            d.processedMessages[msg.MessageID] = true
            d.processedMessagesMu.Unlock()
        }
        
        // Processa a mensagem
        d.processMessage(addr, peerConn, msg)
    }
}

// processMessage processa uma mensagem recebida
func (d *Discovery) processMessage(addr string, peerConn *PeerConnection, msg Message) {
    // Log detalhado para depuração
    fmt.Printf("[DEBUG] Processando mensagem do peer %s: tipo=%s, canal=%s, sender=%s\n", 
        addr, msg.Type, msg.Channel, msg.Sender)
    
    switch msg.Type {
    case "ping":
        // Responde com um pong
        pongMsg := Message{
            Type:      "pong",
            Sender:    d.instanceID,
            Timestamp: time.Now(),
        }
        peerConn.SendMessage(pongMsg)
        fmt.Printf("[INFO] Ping recebido do peer %s, enviando pong\n", addr)
        
    case "pong":
        // Atualiza o timestamp do último ping
        peerConn.LastPing = time.Now()
        fmt.Printf("[INFO] Pong recebido do peer %s\n", addr)
        
    case "join":
        // Peer entrou em um canal
        peerConn.AddChannel(msg.Channel)
        fmt.Printf("[INFO] Peer %s entrou no canal %s\n", addr, msg.Channel)
        
        // Sincroniza nossos canais com o peer para garantir que ele saiba em quais canais estamos
        d.SyncChannelsWithPeer(addr, peerConn)
        
    case "part":
        // Peer saiu de um canal
        peerConn.RemoveChannel(msg.Channel)
        fmt.Printf("[INFO] Peer %s saiu do canal %s\n", addr, msg.Channel)
        
    case "chat":
        // Verifica se estamos no canal da mensagem
        if d.IsInChannel(msg.Channel) {
            fmt.Printf("[INFO] Mensagem de chat recebida do peer %s para canal %s: %s\n", 
                addr, msg.Channel, msg.Content)
            
            // Repassa a mensagem para o handler
            if d.messageHandler != nil {
                d.messageHandler(msg)
            }
            
            // Propaga a mensagem para outros peers que estão no mesmo canal
            // mas não para o peer que enviou a mensagem
            d.propagateMessage(msg, addr)
        } else {
            fmt.Printf("[INFO] Ignorando mensagem para canal %s (não estamos neste canal)\n", msg.Channel)
        }
        
    case "identify":
        // Recebemos uma mensagem de identificação de um peer
        // Atualiza as informações do peer
        peerConn.InstanceID = msg.Sender
        
        // Sincroniza nossos canais com o peer
        d.SyncChannelsWithPeer(addr, peerConn)
        
        fmt.Printf("[INFO] Peer %s identificado como %s\n", addr, msg.Sender)
        
        // Envia nossa identificação de volta
        identifyMsg := Message{
            Type:      "identify",
            Sender:    d.instanceID,
            Timestamp: time.Now(),
        }
        peerConn.SendMessage(identifyMsg)
    }
}

// propagateMessage propaga uma mensagem para todos os peers que estão em um canal específico,
// exceto para o peer que enviou a mensagem
func (d *Discovery) propagateMessage(msg Message, excludeAddr string) {
    d.connMu.RLock()
    defer d.connMu.RUnlock()
    
    // Verifica se a mensagem já foi processada
    if msg.MessageID != "" && d.isMessageProcessed(msg.MessageID) {
        fmt.Printf("[DEBUG] Mensagem %s já foi processada, não propagando\n", msg.MessageID)
        return
    }
    
    // Marca a mensagem como processada para evitar loops
    if msg.MessageID != "" {
        d.MarkMessageAsProcessed(msg.MessageID)
    }
    
    // Lista de peers para os quais a mensagem foi propagada (para debug)
    var propagatedTo []string
    
    for addr, peerConn := range d.connections {
        // Não envia para o peer que enviou a mensagem
        if addr == excludeAddr {
            fmt.Printf("[DEBUG] Não propagando para o remetente original %s\n", addr)
            continue
        }
        
        // Verifica se o peer está no canal da mensagem
        if msg.Channel != "" && !peerConn.IsInChannel(msg.Channel) {
            fmt.Printf("[DEBUG] Peer %s não está no canal %s, não propagando\n", addr, msg.Channel)
            continue
        }
        
        // Envia a mensagem
        if err := peerConn.SendMessage(msg); err != nil {
            fmt.Printf("[ERRO] Falha ao propagar mensagem para peer %s: %v\n", addr, err)
        } else {
            fmt.Printf("[DEBUG] Mensagem propagada com sucesso para peer %s\n", addr)
            propagatedTo = append(propagatedTo, addr)
        }
    }
    
    fmt.Printf("[INFO] Mensagem propagada para %d peers: %v\n", len(propagatedTo), propagatedTo)
}

// MarkMessageAsProcessed marca uma mensagem como processada
func (d *Discovery) MarkMessageAsProcessed(messageID string) {
    d.processedMessagesMu.Lock()
    defer d.processedMessagesMu.Unlock()
    
    d.processedMessages[messageID] = true
    d.processedMessagesTime[messageID] = time.Now()
}

// SyncChannelsWithPeer sincroniza nossos canais com um peer específico
func (d *Discovery) SyncChannelsWithPeer(addr string, peerConn *PeerConnection) {
    d.channelMu.RLock()
    channels := make([]string, 0, len(d.channels))
    for channel := range d.channels {
        channels = append(channels, channel)
    }
    d.channelMu.RUnlock()
    
    fmt.Printf("[INFO] Sincronizando %d canais com peer %s\n", len(channels), addr)
    
    // Envia mensagens JOIN para todos os canais
    for _, channel := range channels {
        joinMsg := Message{
            Type:      "join",
            Sender:    d.instanceID,
            Channel:   channel,
            Timestamp: time.Now(),
        }
        
        if err := peerConn.SendMessage(joinMsg); err != nil {
            fmt.Printf("[ERRO] Falha ao enviar mensagem JOIN para peer %s: %v\n", addr, err)
        } else {
            fmt.Printf("[DEBUG] Mensagem JOIN enviada com sucesso para peer %s (canal: %s)\n", 
                addr, channel)
        }
    }
}

// startCacheCleanupTimer inicia um timer para limpar periodicamente o cache de mensagens processadas
func (d *Discovery) startCacheCleanupTimer() {
    ticker := time.NewTicker(5 * time.Minute)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            d.cleanupProcessedMessages()
        case <-d.ctx.Done():
            return
        }
    }
}

// cleanupProcessedMessages limpa o cache de mensagens processadas
// removendo mensagens mais antigas que 1 hora
func (d *Discovery) cleanupProcessedMessages() {
    d.processedMessagesMu.Lock()
    defer d.processedMessagesMu.Unlock()
    
    now := time.Now()
    expireTime := 1 * time.Hour
    
    // Remove mensagens antigas
    for id, timestamp := range d.processedMessagesTime {
        if now.Sub(timestamp) > expireTime {
            delete(d.processedMessages, id)
            delete(d.processedMessagesTime, id)
        }
    }
    
    fmt.Printf("[DEBUG] Cache de mensagens processadas limpo, %d mensagens no cache\n", 
        len(d.processedMessages))
}

// isMessageProcessed verifica se uma mensagem já foi processada
func (d *Discovery) isMessageProcessed(messageID string) bool {
    d.processedMessagesMu.RLock()
    defer d.processedMessagesMu.RUnlock()
    
    _, exists := d.processedMessages[messageID]
    return exists
}

// DisconnectPeer desconecta um peer específico
func (d *Discovery) DisconnectPeer(addr string) {
    d.connMu.Lock()
    defer d.connMu.Unlock()
    
    // Verifica se o peer existe
    peerConn, exists := d.connections[addr]
    if !exists {
        return
    }
    
    // Fecha a conexão
    peerConn.Conn.Close()
    
    // Remove da lista de conexões
    delete(d.connections, addr)
    
    fmt.Printf("[INFO] Desconectado do peer %s\n", addr)
}

// FixChannelSync força a sincronização de canais com todos os peers
// Isso é útil para resolver problemas de comunicação entre peers no mesmo canal
func (d *Discovery) FixChannelSync() {
    d.connMu.RLock()
    defer d.connMu.RUnlock()
    
    fmt.Printf("[INFO] Forçando sincronização de canais com todos os peers\n")
    
    // Para cada peer conectado
    for addr, peerConn := range d.connections {
        // Sincroniza os canais
        d.SyncChannelsWithPeer(addr, peerConn)
        
        // Envia uma mensagem de identificação
        identifyMsg := Message{
            Type:      "identify",
            Sender:    d.instanceID,
            Timestamp: time.Now(),
        }
        
        if err := peerConn.SendMessage(identifyMsg); err != nil {
            fmt.Printf("[ERRO] Falha ao enviar mensagem de identificação para peer %s: %v\n", addr, err)
        } else {
            fmt.Printf("[DEBUG] Mensagem de identificação enviada para peer %s\n", addr)
        }
        
        // Força o flush da conexão para garantir que as mensagens sejam enviadas imediatamente
        if err := peerConn.Conn.(*net.TCPConn).SetNoDelay(true); err != nil {
            fmt.Printf("[ERRO] Falha ao configurar NoDelay para peer %s: %v\n", addr, err)
        }
    }
}

// DebugConnections exibe informações de depuração sobre as conexões
func (d *Discovery) DebugConnections() {
    d.connMu.RLock()
    defer d.connMu.RUnlock()
    
    fmt.Printf("[DEBUG] === Informações de conexões ===\n")
    fmt.Printf("[DEBUG] Total de conexões: %d\n", len(d.connections))
    
    for addr, peerConn := range d.connections {
        // Obtém os canais do peer
        peerConn.channelsMu.RLock()
        channels := make([]string, 0, len(peerConn.Channels))
        for channel := range peerConn.Channels {
            channels = append(channels, channel)
        }
        peerConn.channelsMu.RUnlock()
        
        fmt.Printf("[DEBUG] Peer %s (ID: %s):\n", addr, peerConn.InstanceID)
        fmt.Printf("[DEBUG]   - Último ping: %s\n", peerConn.LastPing.Format(time.RFC3339))
        fmt.Printf("[DEBUG]   - Canais: %v\n", channels)
    }
    
    // Exibe informações sobre nossos canais
    d.channelMu.RLock()
    channels := make([]string, 0, len(d.channels))
    for channel := range d.channels {
        channels = append(channels, channel)
    }
    d.channelMu.RUnlock()
    
    fmt.Printf("[DEBUG] Nossos canais: %v\n", channels)
    fmt.Printf("[DEBUG] ===========================\n")
}

// HandleNewConnection gerencia uma nova conexão de peer
func (d *Discovery) HandleNewConnection(addr string, peerConn *PeerConnection) {
    // Adiciona o peer à lista de conexões
    d.connMu.Lock()
    d.connections[addr] = peerConn
    d.connMu.Unlock()
    
    fmt.Printf("[INFO] Nova conexão gerenciada: %s\n", addr)
    
    // Envia uma mensagem de identificação para o peer
    identifyMsg := Message{
        Type:      "identify",
        Sender:    d.instanceID,
        Timestamp: time.Now(),
    }
    
    if err := peerConn.SendMessage(identifyMsg); err != nil {
        fmt.Printf("[ERRO] Falha ao enviar mensagem de identificação para peer %s: %v\n", addr, err)
    } else {
        fmt.Printf("[DEBUG] Mensagem de identificação enviada para peer %s\n", addr)
    }
    
    // Sincroniza os canais com o peer
    d.SyncChannelsWithPeer(addr, peerConn)
    
    // Inicia a leitura de mensagens do peer
    go d.readMessages(addr, peerConn)
    
    // Inicia o envio periódico de pings para o peer
    go d.startPingPeer(addr, peerConn)
}

// pingPeer envia um ping para um peer específico
func (d *Discovery) pingPeer(addr string, peerConn *PeerConnection) error {
    // Envia um ping
    pingMsg := Message{
        Type:      "ping",
        Sender:    d.instanceID,
        Timestamp: time.Now(),
    }
    
    if err := peerConn.SendMessage(pingMsg); err != nil {
        return fmt.Errorf("falha ao enviar ping para peer %s: %v", addr, err)
    }
    
    fmt.Printf("[DEBUG] Ping enviado para peer %s\n", addr)
    return nil
}

// startPingPeer inicia o envio periódico de pings para um peer
func (d *Discovery) startPingPeer(addr string, peerConn *PeerConnection) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            // Verifica se o peer ainda está na lista de conexões
            d.connMu.RLock()
            _, exists := d.connections[addr]
            d.connMu.RUnlock()
            
            if !exists {
                return
            }
            
            // Envia um ping
            pingMsg := Message{
                Type:      "ping",
                Sender:    d.instanceID,
                Timestamp: time.Now(),
            }
            
            if err := peerConn.SendMessage(pingMsg); err != nil {
                fmt.Printf("[ERRO] Falha ao enviar ping para peer %s: %v\n", addr, err)
                
                // Remove o peer da lista de conexões
                d.DisconnectPeer(addr)
                return
            }
            
            fmt.Printf("[DEBUG] Ping enviado para peer %s\n", addr)
        case <-d.ctx.Done():
            return
        }
    }
}

// ConnectToAllPeers tenta conectar a todos os peers conhecidos
func (d *Discovery) ConnectToAllPeers() {
    peers := d.GetPeers()
    
    for _, peer := range peers {
        addr := peer.Addr.String()
        
        // Verifica se já estamos conectados
        d.connMu.RLock()
        _, exists := d.connections[addr]
        d.connMu.RUnlock()
        
        if !exists {
            // Tenta conectar em uma goroutine para não bloquear
            go func(addr string) {
                if err := d.ConnectToPeer(addr); err != nil {
                    fmt.Printf("[INFO] Erro ao conectar ao peer %s: %v\n", addr, err)
                }
            }(addr)
        }
    }
}

// SendChatMessage envia uma mensagem de chat para um canal
func (d *Discovery) SendChatMessage(channel, content, sender string) {
    // Cria a mensagem
    msg := Message{
        Type:      "chat",
        Sender:    sender,
        Channel:   channel,
        Content:   content,
        Timestamp: time.Now(),
    }
    
    fmt.Printf("[INFO] Enviando mensagem para canal %s: %s\n", channel, content)
    
    // Envia para todos os peers que estão no canal
    d.broadcastToChannel(msg, channel)
}
