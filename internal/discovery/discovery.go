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

    // Gera o ID do nó local
    nodeID := dht.GenerateNodeID(fmt.Sprintf("%s:%d", instanceID, port))

    d := &Discovery{
        serviceName: "p2p-irc",
        port:        port,
        instanceID:  instanceID,
        dht:         dht.NewRoutingTable(nodeID, 20),
        ctx:         ctx,
        cancel:      cancel,
        metrics: Metrics{
            StartTime: time.Now(),
        },
    }

    return d, nil
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
    entries := make(chan *zeroconf.ServiceEntry)
    defer close(entries)

    // Inicia busca mDNS
    resolver, err := zeroconf.NewResolver(nil)
    if err != nil {
        fmt.Printf("[ERRO] Falha ao criar resolver mDNS: %v\n", err)
        d.metrics.FailedAttempts++
        return
    }

    // Canal para controle da goroutine de browse
    done := make(chan struct{})
    defer close(done)

    fmt.Printf("[INFO] Iniciando serviço de descoberta de peers (instanceID: %s)\n", d.instanceID)

    go func() {
        for {
            select {
            case <-d.ctx.Done():
                fmt.Printf("[INFO] Encerrando goroutine de browse (instanceID: %s)\n", d.instanceID)
                return
            case <-done:
                fmt.Printf("[INFO] Recebido sinal de encerramento para browse (instanceID: %s)\n", d.instanceID)
                return
            default:
                d.mu.Lock()
                d.metrics.BrowseAttempts++
                d.mu.Unlock()

                ctx, cancel := context.WithTimeout(d.ctx, time.Second*5)
                err := resolver.Browse(ctx, serviceType, domain, entries)
                cancel()

                if err != nil {
                    d.mu.Lock()
                    d.metrics.BrowseErrors++
                    d.metrics.LastBrowseError = time.Now()
                    d.mu.Unlock()

                    fmt.Printf("[ERRO] Falha na busca mDNS (instanceID: %s): %v\n", d.instanceID, err)

                    // Aplica rate limiting antes de tentar reconectar
                    if d.canReconnect() {
                        d.mu.Lock()
                        d.metrics.ReconnectCount++
                        d.metrics.LastReconnect = time.Now()
                        d.mu.Unlock()
                        fmt.Printf("[DEBUG] Tentando reconexão após rate limiting (instanceID: %s)\n", d.instanceID)
                    } else {
                        fmt.Printf("[DEBUG] Reconexão limitada por rate limiting (instanceID: %s)\n", d.instanceID)
                        time.Sleep(5 * time.Second)
                        continue
                    }
                }

                fmt.Printf("[DEBUG] Browse bem sucedido (instanceID: %s)\n", d.instanceID)
                time.Sleep(30 * time.Second) // Intervalo entre buscas
            }
        }
    }()

    // Processa entradas encontradas
    knownPeers := make(map[string]bool)
    for {
        select {
        case <-d.ctx.Done():
            fmt.Printf("[INFO] Encerrando processamento de entradas (instanceID: %s)\n", d.instanceID)
            return
        case entry, ok := <-entries:
            if !ok {
                fmt.Printf("[INFO] Canal de entradas fechado (instanceID: %s)\n", d.instanceID)
                return
            }

            var peerInstanceID string
            for _, txt := range entry.Text {
                if len(txt) > 9 && txt[:9] == "instance=" {
                    peerInstanceID = txt[9:]
                    break
                }
            }

            // Ignora a própria instância
            if peerInstanceID == d.instanceID {
                d.mu.Lock()
                d.metrics.PeersIgnored++
                d.mu.Unlock()
                continue
            }

            // Atualiza métricas para peers únicos
            if !knownPeers[peerInstanceID] {
                d.mu.Lock()
                d.metrics.TotalPeersFound++
                d.metrics.LastDiscovery = time.Now()
                knownPeers[peerInstanceID] = true
                d.mu.Unlock()
                fmt.Printf("[INFO] Novo peer descoberto (instanceID: %s, peerID: %s, addr: %s)\n",
                    d.instanceID, peerInstanceID, entry.AddrIPv4[0].String())
            } else {
                fmt.Printf("[DEBUG] Peer já conhecido atualizado (instanceID: %s, peerID: %s, addr: %s)\n",
                    d.instanceID, peerInstanceID, entry.AddrIPv4[0].String())
            }

            // Adiciona o peer ao DHT
            nodeID := dht.GenerateNodeID(fmt.Sprintf("%s:%d", peerInstanceID, entry.Port))
            node := dht.Node{
                ID: nodeID,
                Addr: &net.UDPAddr{
                    IP:   entry.AddrIPv4[0],
                    Port: entry.Port,
                },
                LastSeen:   time.Now(),
                InstanceID: peerInstanceID,
            }
            d.dht.AddNode(node)
        }
    }
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
        txt := []string{
            fmt.Sprintf("port=%d", d.port),
            fmt.Sprintf("instance=%s", d.instanceID),
        }

        d.zeroconf, err = zeroconf.Register(
            "p2p-irc-"+d.instanceID,
            serviceType,
            domain,
            d.port,
            txt,
            nil,
        )

        if err == nil {
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

// GetPeers retorna a lista atual de peers conhecidos
func (d *Discovery) GetPeers() []dht.Node {
    d.mu.RLock()
    defer d.mu.RUnlock()
    // Usa o ID da instância local como alvo
    target := dht.GenerateNodeID(fmt.Sprintf("%s:%d", d.instanceID, d.port))
    return d.dht.FindClosestNodes(target, 20)
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
