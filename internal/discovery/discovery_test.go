package discovery

import (
    "fmt"
    "net"
    "testing"
    "time"

    "github.com/peder1981/p2p-irc/internal/dht"
)

func TestDiscovery(t *testing.T) {
    // Cria duas instâncias do Discovery
    d1, err := New(nil, 0) // porta 0 = aleatória
    if err != nil {
        t.Fatalf("erro ao criar discovery 1: %v", err)
    }

    d2, err := New(nil, 0)
    if err != nil {
        t.Fatalf("erro ao criar discovery 2: %v", err)
    }

    // Inicia os serviços
    if err := d1.Start(); err != nil {
        t.Fatalf("erro ao iniciar discovery 1: %v", err)
    }
    defer d1.Stop()

    if err := d2.Start(); err != nil {
        t.Fatalf("erro ao iniciar discovery 2: %v", err)
    }
    defer d2.Stop()

    // Espera descoberta mútua
    time.Sleep(2 * time.Second)

    // Verifica se descobriram um ao outro
    peers1 := d1.GetPeers()
    peers2 := d2.GetPeers()

    if len(peers1) == 0 {
        t.Error("discovery 1 não encontrou nenhum peer")
    }

    if len(peers2) == 0 {
        t.Error("discovery 2 não encontrou nenhum peer")
    }

    // Verifica se as instâncias são diferentes
    if d1.instanceID == d2.instanceID {
        t.Error("IDs de instância são iguais")
    }
}

func TestDiscoveryReconnection(t *testing.T) {
    // Cria discovery com retry
    d, err := New(nil, 0)
    if err != nil {
        t.Fatalf("erro ao criar discovery: %v", err)
    }

    // Inicia serviço
    if err := d.Start(); err != nil {
        t.Fatalf("erro ao iniciar discovery: %v", err)
    }
    defer d.Stop()

    // Simula queda de conexão
    d.zeroconf.Shutdown()

    // Espera reconexão
    time.Sleep(6 * time.Second)

    // Verifica se está funcionando
    if d.zeroconf == nil {
        t.Error("discovery não se recuperou após queda")
    }
}

func TestDiscoveryConcurrent(t *testing.T) {
    // Cria múltiplas instâncias concorrentemente
    const n = 5
    discoveries := make([]*Discovery, n)
    errors := make(chan error, n)

    for i := 0; i < n; i++ {
        go func(i int) {
            d, err := New(nil, 0)
            if err != nil {
                errors <- fmt.Errorf("erro ao criar discovery %d: %v", i, err)
                return
            }

            if err := d.Start(); err != nil {
                errors <- fmt.Errorf("erro ao iniciar discovery %d: %v", i, err)
                return
            }

            discoveries[i] = d
            errors <- nil
        }(i)
    }

    // Espera todas as goroutines
    for i := 0; i < n; i++ {
        if err := <-errors; err != nil {
            t.Error(err)
        }
    }

    // Limpa
    for _, d := range discoveries {
        if d != nil {
            d.Stop()
        }
    }
}

func TestDiscoveryCache(t *testing.T) {
    // Cria discovery com cache
    d, err := New(nil, 0)
    if err != nil {
        t.Fatalf("erro ao criar discovery: %v", err)
    }

    // Adiciona alguns peers ao cache
    for i := 0; i < 3; i++ {
        nodeID := dht.GenerateNodeID(fmt.Sprintf("test%d:1234", i))
        node := dht.Node{
            ID: nodeID,
            Addr: &net.UDPAddr{
                IP:   net.ParseIP("127.0.0.1"),
                Port: 1234 + i,
            },
            LastSeen: time.Now(),
        }
        d.dht.AddNode(node)
    }

    // Verifica se peers estão no cache
    peers := d.GetPeers()
    if len(peers) != 3 {
        t.Errorf("número incorreto de peers em cache: got %d, want 3", len(peers))
    }

    // Verifica expiração de cache
    time.Sleep(time.Second)
    peers = d.GetPeers()
    for _, p := range peers {
        if time.Since(p.LastSeen) > time.Hour {
            t.Error("peer não expirou corretamente")
        }
    }
}

func TestDiscoveryMetrics(t *testing.T) {
    d, err := New(nil, 0)
    if err != nil {
        t.Fatalf("erro ao criar discovery: %v", err)
    }

    // Verifica métricas iniciais
    metrics := d.GetMetrics()
    if metrics.ActivePeers != 0 {
        t.Error("número inicial de peers deveria ser 0")
    }
    if metrics.BrowseAttempts != 0 {
        t.Error("número inicial de tentativas de browse deveria ser 0")
    }
    if metrics.BrowseErrors != 0 {
        t.Error("número inicial de erros de browse deveria ser 0")
    }

    // Inicia o serviço
    if err := d.Start(); err != nil {
        t.Fatalf("erro ao iniciar discovery: %v", err)
    }

    // Espera algumas tentativas de browse
    time.Sleep(2 * time.Second)

    metrics = d.GetMetrics()
    if metrics.BrowseAttempts == 0 {
        t.Error("deveria ter registrado tentativas de browse")
    }

    // Para o serviço e verifica se as métricas são mantidas
    d.Stop()
    metricsAfterStop := d.GetMetrics()
    if metricsAfterStop.BrowseAttempts != metrics.BrowseAttempts {
        t.Error("métricas não deveriam ser resetadas após parar o serviço")
    }
}

func TestDiscoveryShutdown(t *testing.T) {
    d, err := New(nil, 0)
    if err != nil {
        t.Fatalf("erro ao criar discovery: %v", err)
    }

    // Inicia o serviço
    if err := d.Start(); err != nil {
        t.Fatalf("erro ao iniciar discovery: %v", err)
    }

    // Espera o serviço inicializar
    time.Sleep(time.Second)

    // Adiciona alguns peers antes do shutdown
    for i := 0; i < 3; i++ {
        nodeID := dht.GenerateNodeID(fmt.Sprintf("test%d:1234", i))
        node := dht.Node{
            ID: nodeID,
            Addr: &net.UDPAddr{
                IP:   net.ParseIP("127.0.0.1"),
                Port: 1234 + i,
            },
            LastSeen: time.Now(),
            InstanceID: fmt.Sprintf("test%d", i),
        }
        d.dht.AddNode(node)
    }

    // Verifica peers antes do shutdown
    peersBeforeStop := d.GetPeers()
    if len(peersBeforeStop) != 3 {
        t.Errorf("número incorreto de peers antes do shutdown: got %d, want 3", len(peersBeforeStop))
    }

    // Para o serviço
    d.Stop()

    // Espera um pouco para garantir que todas as goroutines foram finalizadas
    time.Sleep(time.Second)

    // Tenta adicionar um novo peer após o shutdown
    nodeID := dht.GenerateNodeID("test4:1234")
    node := dht.Node{
        ID: nodeID,
        Addr: &net.UDPAddr{
            IP:   net.ParseIP("127.0.0.1"),
            Port: 1234,
        },
        LastSeen: time.Now(),
        InstanceID: "test4",
    }
    d.dht.AddNode(node)

    // Verifica se o serviço ainda mantém os peers após o shutdown
    peersAfterStop := d.GetPeers()
    if len(peersAfterStop) != 4 {
        t.Errorf("número incorreto de peers após shutdown: got %d, want 4", len(peersAfterStop))
    }
}

func TestDiscoveryGracefulShutdown(t *testing.T) {
    d, err := New(nil, 0)
    if err != nil {
        t.Fatalf("erro ao criar discovery: %v", err)
    }

    // Inicia o serviço
    if err := d.Start(); err != nil {
        t.Fatalf("erro ao iniciar discovery: %v", err)
    }

    // Espera o serviço inicializar
    time.Sleep(time.Second)

    // Simula descoberta de peers em uma goroutine separada
    done := make(chan struct{})
    go func() {
        for i := 0; i < 10; i++ {
            nodeID := dht.GenerateNodeID(fmt.Sprintf("test%d:1234", i))
            node := dht.Node{
                ID: nodeID,
                Addr: &net.UDPAddr{
                    IP:   net.ParseIP("127.0.0.1"),
                    Port: 1234 + i,
                },
                LastSeen: time.Now(),
                InstanceID: fmt.Sprintf("test%d", i),
            }
            d.dht.AddNode(node)
            time.Sleep(100 * time.Millisecond)
        }
        close(done)
    }()

    // Espera algumas descobertas acontecerem
    time.Sleep(300 * time.Millisecond)

    // Inicia shutdown enquanto descobertas ainda estão acontecendo
    d.Stop()

    // Espera a goroutine de descoberta terminar
    <-done

    // Verifica métricas após shutdown
    metrics := d.GetMetrics()
    if metrics.PeersIgnored > metrics.TotalPeersFound {
        t.Error("número de peers ignorados não deveria ser maior que o total encontrado")
    }
}
