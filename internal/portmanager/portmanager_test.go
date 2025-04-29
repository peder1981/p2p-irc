package portmanager

import (
    "net"
    "testing"
)

func TestPortManager(t *testing.T) {
    pm := New()

    // Testa obtenção de porta disponível
    port1, err := pm.GetAvailablePort()
    if err != nil {
        t.Fatalf("erro ao obter porta: %v", err)
    }
    if port1 < 49152 || port1 > 65535 {
        t.Errorf("porta %d fora do range esperado [49152-65535]", port1)
    }

    // Testa que a porta obtida está realmente disponível
    addr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: port1}
    listener, err := net.ListenTCP("tcp", addr)
    if err != nil {
        t.Fatalf("porta %d não está disponível: %v", port1, err)
    }
    defer listener.Close()

    // Testa liberação de porta
    pm.ReleasePort(port1)

    // Testa que podemos obter uma porta (não necessariamente a mesma)
    port2, err := pm.GetAvailablePort()
    if err != nil {
        t.Fatalf("erro ao obter segunda porta: %v", err)
    }
    if port2 < 49152 || port2 > 65535 {
        t.Errorf("porta %d fora do range esperado [49152-65535]", port2)
    }

    // Testa obtenção de múltiplas portas
    ports := make(map[int]bool)
    for i := 0; i < 10; i++ {
        p, err := pm.GetAvailablePort()
        if err != nil {
            t.Fatalf("erro ao obter porta %d: %v", i, err)
        }
        if ports[p] {
            t.Errorf("porta %d obtida duas vezes", p)
        }
        ports[p] = true
    }

    // Libera todas as portas
    for p := range ports {
        pm.ReleasePort(p)
    }
}

func TestPortManagerConcurrent(t *testing.T) {
    pm := New()
    done := make(chan bool)
    ports := make(chan int, 100)

    // Inicia 10 goroutines para obter portas concorrentemente
    for i := 0; i < 10; i++ {
        go func() {
            for j := 0; j < 10; j++ {
                port, err := pm.GetAvailablePort()
                if err != nil {
                    t.Errorf("erro ao obter porta: %v", err)
                    return
                }
                ports <- port
            }
            done <- true
        }()
    }

    // Espera todas as goroutines terminarem
    for i := 0; i < 10; i++ {
        <-done
    }
    close(ports)

    // Verifica se todas as portas são únicas
    seen := make(map[int]bool)
    for port := range ports {
        if seen[port] {
            t.Errorf("porta %d obtida mais de uma vez", port)
        }
        seen[port] = true
    }
}
