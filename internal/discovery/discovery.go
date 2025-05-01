package discovery

import (
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
    "bufio"
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
        connections: make(map[string]*PeerConnection),
        channels:    make(map[string]bool),
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
                
                // Log apenas para novos peers
                if isNewNode {
                    fmt.Printf("[INFO] Novo peer encontrado: %s:%d (instanceID: %s)\n",
                        ip.String(), entry.Port, peerInstanceID)
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
        // Ignora apenas interfaces down
        if iface.Flags&net.FlagUp == 0 {
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
    defer d.channelMu.Unlock()
    
    d.channels[channel] = true
    
    // Notifica todos os peers que entramos no canal
    d.BroadcastJoinChannel(channel)
    
    // Força uma sincronização com todos os peers
    go d.SyncPeers()
}

// PartChannel remove um canal da lista de canais
func (d *Discovery) PartChannel(channel string) {
    d.channelMu.Lock()
    defer d.channelMu.Unlock()
    
    delete(d.channels, channel)
    
    // Notifica todos os peers que saímos do canal
    d.BroadcastPartChannel(channel)
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
        fmt.Printf("[INFO] Já conectado ao peer %s, ignorando nova conexão\n", addr)
        return nil
    }
    d.connMu.RUnlock()
    
    // Vamos tentar conectar independentemente do IP/porta
    // A verificação de identidade será feita após a conexão
    fmt.Printf("[INFO] Tentando conectar ao peer %s\n", addr)

    // Estabelece a conexão
    conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
    if err != nil {
        return fmt.Errorf("erro ao conectar ao peer: %w", err)
    }
    
    fmt.Printf("[INFO] Conectado ao peer %s\n", addr)
    
    // Cria a conexão do peer
    peerConn := NewPeerConnection(conn)
    
    // Adiciona à lista de conexões
    remoteAddr := conn.RemoteAddr().String()
    d.connMu.Lock()
    d.connections[remoteAddr] = peerConn
    d.connMu.Unlock()
    
    // Enviar IDENTIFY imediatamente
    identifyMsg := Message{
        Type:      TypeIdentify,
        Sender:    d.instanceID,
        Content:   d.instanceID,
        Timestamp: time.Now(),
    }
    if err := peerConn.SendMessage(identifyMsg); err != nil {
        fmt.Printf("[ERRO] Falha ao enviar identificação para peer %s: %v\n", remoteAddr, err)
    }
    
    // Inicia a leitura de mensagens
    go d.readMessages(remoteAddr, peerConn)
    
    // Inicia ping periódico para verificar a conexão
    go d.pingPeer(remoteAddr, peerConn)
    
    // Envia informações sobre os canais em que estamos
    d.channelMu.RLock()
    for channel := range d.channels {
        joinMsg := Message{
            Type:      TypeJoinChannel,
            Sender:    d.instanceID,
            Channel:   channel,
            Timestamp: time.Now(),
        }
        peerConn.SendMessage(joinMsg)
        fmt.Printf("[INFO] Enviando informação de canal %s para peer %s\n", channel, remoteAddr)
    }
    d.channelMu.RUnlock()
    
    return nil
}

// readMessages lê mensagens de um peer
func (d *Discovery) readMessages(addr string, peerConn *PeerConnection) {
    defer func() {
        // Verificação de pânico para evitar crash
        if r := recover(); r != nil {
            fmt.Printf("[ERRO] Pânico na leitura de mensagens do peer %s: %v\n", addr, r)
        }
        
        // Remove a conexão quando a goroutine terminar
        d.connMu.Lock()
        delete(d.connections, addr)
        d.connMu.Unlock()
        
        peerConn.Close()
        fmt.Printf("[INFO] Desconectado do peer %s\n", addr)
    }()
    
    // Buffer para ler as mensagens
    reader := bufio.NewReader(peerConn.Conn)
    
    // Configure timeout para leitura
    peerConn.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
    
    // Flag para indicar se já verificamos o instanceID
    instanceIDVerified := false
    
    for {
        // Lê uma mensagem do peer
        data := make([]byte, 4096)
        n, err := reader.Read(data)
        if err != nil {
            if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
                fmt.Printf("[INFO] Timeout ao ler do peer %s\n", addr)
            } else {
                fmt.Printf("[INFO] Erro ao ler do peer %s: %v\n", addr, err)
            }
            return
        }
        
        // Reset do timeout para a próxima leitura
        peerConn.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
        
        // Verifica se recebemos dados
        if n == 0 {
            fmt.Printf("[DEBUG] Nenhum dado recebido do peer %s\n", addr)
            continue
        }
        
        // Processa os dados recebidos
        dataStr := string(data[:n])
        messages := strings.Split(dataStr, "\n")
        
        for _, msgStr := range messages {
            msgStr = strings.TrimSpace(msgStr)
            if len(msgStr) == 0 {
                continue
            }
            
            // Deserializa a mensagem
            var msg Message
            if err := json.Unmarshal([]byte(msgStr), &msg); err != nil {
                fmt.Printf("[ERRO] Erro ao deserializar mensagem do peer %s: %v\n", addr, err)
                continue
            }
            
            // Verificação prioritária de instanceID para evitar conexões duplicadas
            if msg.Type == TypeIdentify && !instanceIDVerified {
                instanceIDVerified = true
                if msg.Content == d.instanceID {
                    fmt.Printf("[INFO] Detectada conexão com nossa própria instância, desconectando %s\n", addr)
                    return // Isso vai chamar o defer e limpar a conexão
                }
            }
            
            fmt.Printf("[INFO] Mensagem recebida do peer %s: tipo=%s, canal=%s, conteúdo=%s\n", 
                addr, msg.Type, msg.Channel, msg.Content)
            
            // Processa a mensagem
            d.processMessage(addr, peerConn, msg)
        }
    }
}

// processMessage processa mensagens recebidas de peers
func (d *Discovery) processMessage(addr string, peerConn *PeerConnection, msg Message) {
    switch msg.Type {
    case TypePing:
        // Responde com um pong
        pongMsg := Message{
            Type:      TypePong,
            Sender:    d.instanceID,
            Timestamp: time.Now(),
        }
        peerConn.SendMessage(pongMsg)
        fmt.Printf("[INFO] Ping recebido do peer %s, enviando pong\n", addr)
        
    case TypePong:
        // Atualiza o timestamp do último ping
        peerConn.LastPing = time.Now()
        fmt.Printf("[INFO] Pong recebido do peer %s\n", addr)
        
    case TypeIdentify:
        // Recebemos a identificação do peer
        peerInstanceID := msg.Content
        fmt.Printf("[INFO] Identificação recebida do peer %s: instanceID=%s\n", addr, peerInstanceID)
        
        // Verifica se estamos tentando conectar a nós mesmos
        if peerInstanceID == d.instanceID {
            fmt.Printf("[INFO] Detectada conexão com nossa própria instância, desconectando %s\n", addr)
            go d.DisconnectPeer(addr)
            return
        }
        
        // Se estamos em algum canal, envia informações sobre eles
        d.channelMu.RLock()
        hasChannels := len(d.channels) > 0
        d.channelMu.RUnlock()
        
        if hasChannels {
            // Envia informações sobre nossos canais para este peer
            go d.SyncPeers()
        }
        
    case TypeJoinChannel:
        if msg.Channel != "" {
            // Verificar se o peer já está no canal para evitar loops de confirmação
            alreadyInChannel := false
            peerConn.mu.RLock()
            alreadyInChannel = peerConn.channels[msg.Channel]
            peerConn.mu.RUnlock()
            
            if alreadyInChannel {
                // O peer já está no canal, não precisamos reconfirmar
                fmt.Printf("[DEBUG] Peer %s já está no canal %s, ignorando JOIN redundante\n", addr, msg.Channel)
                return
            }
            
            peerConn.mu.Lock()
            peerConn.channels[msg.Channel] = true
            fmt.Printf("[SYNC] Peer %s entrou no canal %s\n", addr, msg.Channel)
            peerConn.mu.Unlock()
            
            // Se também estamos neste canal, confirmamos o JOIN
            if d.IsInChannel(msg.Channel) {
                // Confirmamos nossa presença no canal para o peer
                joinMsg := Message{
                    Type:      TypeJoinChannel,
                    Sender:    d.instanceID,
                    Channel:   msg.Channel,
                    Timestamp: time.Now(),
                }
                
                if err := peerConn.SendMessage(joinMsg); err != nil {
                    fmt.Printf("[ERRO] Erro ao confirmar JOIN para canal %s com peer %s: %v\n", 
                        msg.Channel, addr, err)
                } else {
                    fmt.Printf("[DEBUG] JOIN confirmado para canal %s com peer %s\n", 
                        msg.Channel, addr)
                }
            }
        }
        
    case TypePartChannel:
        // Peer saiu de um canal
        peerConn.mu.Lock()
        delete(peerConn.channels, msg.Channel)
        fmt.Printf("[INFO] Peer %s saiu do canal %s\n", addr, msg.Channel)
        peerConn.mu.Unlock()
        
    case TypeChatMessage:
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
    }
}

// SendChatMessage envia uma mensagem de chat para um canal
func (d *Discovery) SendChatMessage(channel, content, sender string) {
    // Cria a mensagem
    msg := Message{
        Type:      TypeChatMessage,
        Sender:    sender,
        Channel:   channel,
        Content:   content,
        Timestamp: time.Now(),
    }
    
    fmt.Printf("[INFO] Enviando mensagem para canal %s: %s\n", channel, content)
    
    // Envia para todos os peers que estão no canal
    d.BroadcastMessage(msg)
}

// BroadcastMessage envia uma mensagem para todos os peers que estão em um canal específico
func (d *Discovery) BroadcastMessage(msg Message) {
    d.connMu.RLock()
    defer d.connMu.RUnlock()
    
    fmt.Printf("[INFO] Broadcast de mensagem tipo=%s para canal=%s, %d peers conectados\n", 
        msg.Type, msg.Channel, len(d.connections))
    
    for addr, peerConn := range d.connections {
        // Verifica se o peer está no canal da mensagem (apenas para chat)
        if msg.Type == TypeChatMessage && msg.Channel != "" && !peerConn.IsInChannel(msg.Channel) {
            fmt.Printf("[INFO] Peer %s não está no canal %s, ignorando\n", addr, msg.Channel)
            continue
        }
        
        // Envia a mensagem
        if err := peerConn.SendMessage(msg); err != nil {
            fmt.Printf("[INFO] Erro ao enviar mensagem para peer %s: %v\n", addr, err)
        } else {
            fmt.Printf("[INFO] Mensagem enviada com sucesso para peer %s\n", addr)
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
        
        if !exists && d.canReconnect() {
            // Tenta conectar em uma goroutine para não bloquear
            go func(addr string) {
                if err := d.ConnectToPeer(addr); err != nil {
                    fmt.Printf("[INFO] Erro ao conectar ao peer %s: %v\n", addr, err)
                }
            }(addr)
        }
    }
}

// HandleNewConnection gerencia uma nova conexão recebida
func (d *Discovery) HandleNewConnection(addr string, peerConn *PeerConnection) {
    // Adiciona à lista de conexões
    d.connMu.Lock()
    d.connections[addr] = peerConn
    d.connMu.Unlock()
    
    // Inicia a leitura de mensagens
    go d.readMessages(addr, peerConn)
    
    // Inicia ping periódico
    go d.pingPeer(addr, peerConn)
    
    // Envia informações sobre os canais em que estamos
    d.channelMu.RLock()
    for channel := range d.channels {
        joinMsg := Message{
            Type:      TypeJoinChannel,
            Sender:    d.instanceID,
            Channel:   channel,
            Timestamp: time.Now(),
        }
        peerConn.SendMessage(joinMsg)
    }
    d.channelMu.RUnlock()
    
    fmt.Printf("[INFO] Nova conexão gerenciada: %s\n", addr)
}

// pingPeer envia pings periódicos para um peer
func (d *Discovery) pingPeer(addr string, peerConn *PeerConnection) {
    ticker := time.NewTicker(20 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-d.ctx.Done():
            return
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
                Type:      TypePing,
                Sender:    d.instanceID,
                Timestamp: time.Now(),
            }
            
            if err := peerConn.SendMessage(pingMsg); err != nil {
                fmt.Printf("[INFO] Erro ao enviar ping para peer %s: %v\n", addr, err)
                return
            }
        }
    }
}

// BroadcastJoinChannel notifica todos os peers que entramos em um canal
func (d *Discovery) BroadcastJoinChannel(channel string) {
    // Cria a mensagem
    msg := Message{
        Type:      TypeJoinChannel,
        Sender:    d.instanceID,
        Channel:   channel,
        Timestamp: time.Now(),
    }
    
    // Envia para todos os peers
    d.BroadcastMessage(msg)
}

// BroadcastPartChannel notifica todos os peers que saímos de um canal
func (d *Discovery) BroadcastPartChannel(channel string) {
    // Cria a mensagem
    msg := Message{
        Type:      TypePartChannel,
        Sender:    d.instanceID,
        Channel:   channel,
        Timestamp: time.Now(),
    }
    
    // Envia para todos os peers
    d.BroadcastMessage(msg)
}

// propagateMessage propaga uma mensagem para todos os peers que estão em um canal específico,
// exceto para o peer que enviou a mensagem
func (d *Discovery) propagateMessage(msg Message, excludeAddr string) {
    d.connMu.RLock()
    defer d.connMu.RUnlock()
    
    for addr, peerConn := range d.connections {
        // Não envia para o peer que enviou a mensagem
        if addr == excludeAddr {
            continue
        }
        
        // Verifica se o peer está no canal da mensagem (apenas para chat)
        if msg.Type == TypeChatMessage && msg.Channel != "" && !peerConn.IsInChannel(msg.Channel) {
            continue
        }
        
        // Envia a mensagem
        if err := peerConn.SendMessage(msg); err != nil {
            fmt.Printf("[INFO] Erro ao propagar mensagem para peer %s: %v\n", addr, err)
        }
    }
}

// DisconnectPeer desconecta um peer específico pelo endereço
func (d *Discovery) DisconnectPeer(addr string) error {
    d.connMu.Lock()
    if peerConn, exists := d.connections[addr]; exists {
        peerConn.Close()
        delete(d.connections, addr)
        fmt.Printf("[INFO] Peer desconectado: %s\n", addr)
    }
    d.connMu.Unlock()
    return nil
}

// RemovePeer remove um peer da lista de conexões
func (d *Discovery) RemovePeer(addr string) {
    d.connMu.Lock()
    if _, exists := d.connections[addr]; exists {
        delete(d.connections, addr)
        fmt.Printf("[INFO] Peer removido: %s\n", addr)
    }
    d.connMu.Unlock()
}

// getPeerConn retorna a conexão de um peer pelo endereço
func (d *Discovery) getPeerConn(addr string) (*PeerConnection, bool) {
    d.connMu.RLock()
    peerConn, exists := d.connections[addr]
    d.connMu.RUnlock()
    return peerConn, exists
}

// PeerConnection representa uma conexão com um peer
type PeerConnection struct {
    Conn      net.Conn
    sendQueue chan Message
    closeChan chan struct{}
    channels  map[string]bool
    mu        sync.RWMutex
    LastPing  time.Time
}

// NewPeerConnection cria uma nova conexão de peer
func NewPeerConnection(conn net.Conn) *PeerConnection {
    return &PeerConnection{
        Conn:      conn,
        sendQueue: make(chan Message, 100),
        closeChan: make(chan struct{}),
        channels:  make(map[string]bool),
    }
}

// Close fecha a conexão com o peer
func (pc *PeerConnection) Close() {
    close(pc.closeChan)
    if pc.Conn != nil {
        pc.Conn.Close()
    }
}

// SendMessage envia uma mensagem para o peer
func (pc *PeerConnection) SendMessage(msg Message) error {
    // Serializa a mensagem
    data, err := json.Marshal(msg)
    if err != nil {
        return fmt.Errorf("erro ao serializar mensagem: %w", err)
    }
    
    // Adiciona quebra de linha para delimitar mensagens
    data = append(data, '\n')
    
    // Define timeout de escrita
    pc.Conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
    
    // Envia a mensagem
    _, err = pc.Conn.Write(data)
    if err != nil {
        return fmt.Errorf("erro ao enviar mensagem: %w", err)
    }
    
    return nil
}

// IsInChannel verifica se o peer está em um canal
func (pc *PeerConnection) IsInChannel(channel string) bool {
    pc.mu.RLock()
    defer pc.mu.RUnlock()
    
    return pc.channels[channel]
}

// AddChannel adiciona um canal à lista de canais do peer
func (pc *PeerConnection) AddChannel(channel string) {
    pc.mu.Lock()
    defer pc.mu.Unlock()
    
    pc.channels[channel] = true
}

// RemoveChannel remove um canal da lista de canais do peer
func (pc *PeerConnection) RemoveChannel(channel string) {
    pc.mu.Lock()
    defer pc.mu.Unlock()
    
    delete(pc.channels, channel)
}
