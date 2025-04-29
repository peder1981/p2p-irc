package portmanager

import (
    "fmt"
    "net"
    "sync"
)

const (
    // Faixa de portas que podemos usar
    minPort = 49152 // Início da faixa dinâmica/privada
    maxPort = 65535 // Fim da faixa de portas
)

// PortManager gerencia a alocação de portas para o serviço
type PortManager struct {
    mu           sync.Mutex
    usedPorts    map[int]bool
    currentPort  int
}

// New cria uma nova instância do PortManager
func New() *PortManager {
    return &PortManager{
        usedPorts:   make(map[int]bool),
        currentPort: minPort,
    }
}

// GetAvailablePort encontra e retorna uma porta TCP disponível
func (pm *PortManager) GetAvailablePort() (int, error) {
    pm.mu.Lock()
    defer pm.mu.Unlock()

    // Tenta portas sequencialmente até encontrar uma disponível
    for port := pm.currentPort; port <= maxPort; port++ {
        // Verifica se a porta já está em uso pelo nosso sistema
        if pm.usedPorts[port] {
            continue
        }

        // Tenta criar um listener TCP para verificar se a porta está realmente disponível
        addr := fmt.Sprintf(":%d", port)
        listener, err := net.Listen("tcp", addr)
        if err != nil {
            continue
        }
        listener.Close()

        // Marca a porta como em uso e atualiza a próxima porta a tentar
        pm.usedPorts[port] = true
        pm.currentPort = port + 1
        if pm.currentPort > maxPort {
            pm.currentPort = minPort
        }

        return port, nil
    }

    return 0, fmt.Errorf("não há portas disponíveis na faixa %d-%d", minPort, maxPort)
}

// ReleasePort libera uma porta para ser reutilizada
func (pm *PortManager) ReleasePort(port int) {
    pm.mu.Lock()
    defer pm.mu.Unlock()
    delete(pm.usedPorts, port)
}
