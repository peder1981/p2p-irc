package mesh

import (
	"fmt"
	"sync"

	"github.com/peder1981/p2p-irc/internal/discovery"
)

// MeshManager gerencia o ciclo de vida da integração mesh.
type MeshManager struct {
	mu               sync.Mutex
	config           MeshConfig
	discoveryService *discovery.Discovery
	integration      *MeshIntegration
	isRunning        bool
	messageHandler   MessageHandler
}

// NewMeshManager cria um novo gerenciador de mesh.
func NewMeshManager(cfg MeshConfig, dsc *discovery.Discovery, handler MessageHandler) *MeshManager {
	return &MeshManager{
		config:           cfg,
		discoveryService: dsc,
		messageHandler:   handler,
	}
}

// Start tenta iniciar a integração com a rede mesh.
func (m *MeshManager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.isRunning {
		return fmt.Errorf("a rede mesh já está ativa")
	}

	m.integration = NewMeshIntegration(m.discoveryService, m.config, m.messageHandler)
	if err := m.integration.Start(); err != nil {
		return fmt.Errorf("falha ao iniciar a integração mesh: %w", err)
	}

	m.isRunning = true

	return nil
}

// Stop para a integração com a rede mesh.
func (m *MeshManager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.isRunning {
		return fmt.Errorf("a rede mesh não está ativa")
	}

	m.integration.Stop()
	m.integration = nil
	m.isRunning = false
	return nil
}

// IsRunning retorna true se a integração mesh estiver ativa.
func (m *MeshManager) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.isRunning
}

// GetIntegration retorna a instância de integração atual (pode ser nil).
func (m *MeshManager) GetIntegration() *MeshIntegration {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.integration
}

// GetPeers retorna a lista de peers atualmente conhecidos pela mesh.
func (m *MeshManager) GetPeers() []*MeshPeer {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.integration == nil || !m.isRunning {
		return nil
	}

	return m.integration.GetPeers()
}

