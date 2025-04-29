package discovery

import (
    "context"
    "crypto/rand"
    "encoding/hex"
    "fmt"
    "net"
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

    fmt.Printf("[INFO] Iniciando serviço de descoberta de peers (instanceID: %s, porta: %d)\n", d.instanceID, d.port)

    // Goroutine para procurar peers
    go func() {
        defer close(done)
        
        // Primeira busca imediata
        d.performBrowse()
        
        // Depois usa ticker para buscas periódicas
        ticker := time.NewTicker(time.Second * 30)
        defer ticker.Stop()
        
        for {
            select {
            case <-d.ctx.Done():
                fmt.Printf("[INFO] Encerrando goroutine de browse (instanceID: %s)\n", d.instanceID)
                return
            case <-ticker.C:
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

    // Cria um novo contexto para esta busca
    browseCtx, browseCancel := context.WithTimeout(d.ctx, time.Second*5)
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
