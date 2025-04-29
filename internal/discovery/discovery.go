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
)

// Metrics contém métricas do sistema de descoberta
type Metrics struct {
    ActivePeers     int
    TotalDiscovered int64
    LastDiscovery   time.Time
    FailedAttempts  int64
    Uptime         time.Duration
    StartTime      time.Time
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

// Stop para todos os serviços de descoberta
func (d *Discovery) Stop() {
    d.cancel()
    if d.zeroconf != nil {
        d.zeroconf.Shutdown()
    }
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

// discoverPeers procura continuamente por novos peers
func (d *Discovery) discoverPeers() {
    entries := make(chan *zeroconf.ServiceEntry)

    // Inicia busca mDNS
    resolver, err := zeroconf.NewResolver(nil)
    if err != nil {
        fmt.Printf("Erro ao criar resolver mDNS: %v\n", err)
        return
    }

    go func() {
        for {
            ctx, cancel := context.WithTimeout(d.ctx, time.Second*5)
            err := resolver.Browse(ctx, serviceType, domain, entries)
            cancel()
            if err != nil {
                fmt.Printf("Erro na busca mDNS: %v\n", err)
                time.Sleep(5 * time.Second)
                continue
            }
            time.Sleep(30 * time.Second) // Intervalo entre buscas
        }
    }()

    // Processa entradas encontradas
    for entry := range entries {
        var peerInstanceID string
        for _, txt := range entry.Text {
            if len(txt) > 9 && txt[:9] == "instance=" {
                peerInstanceID = txt[9:]
                break
            }
        }

        // Ignora a própria instância
        if peerInstanceID == d.instanceID {
            continue
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
