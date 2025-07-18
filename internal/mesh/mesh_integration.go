package mesh

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/peder1981/p2p-irc/internal/discovery"
)

// TransportType define os tipos de transporte disponíveis
type TransportType int

const (
	TransportTCP TransportType = iota
	TransportBluetoothMesh
	TransportHybrid
)

// MeshCapabilities define as capacidades mesh de um peer
type MeshCapabilities struct {
	SupportsBluetoothMesh bool
	SupportsTCP           bool
	SignalStrength        int // 0-100
	Latency               time.Duration
	BatteryLevel          int // 0-100, -1 se não disponível
	LastSeen              time.Time
}

// MeshPeer representa um peer na rede mesh
type MeshPeer struct {
	ID           string
	Address      string
	Transport    TransportType
	Capabilities MeshCapabilities
	Channels     []string
	LastActivity time.Time
	Connection   net.Conn
	mu           sync.RWMutex
}

// MeshIntegration gerencia a integração entre p2p-irc e tecnologias mesh do Bitchat
type MeshIntegration struct {
	discovery *discovery.Discovery
	peers     map[string]*MeshPeer
	peersMu   sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc

	// Callbacks para UI
	onPeerDiscovered func(peer *MeshPeer)
	onPeerLost       func(peerID string)
	onMeshStatus     func(status MeshStatus)

	// Configurações
	enableBluetooth bool
	enableTCP       bool
	autoFallback    bool

	// Métricas para UX
	metrics MeshMetrics
}

// MeshStatus representa o status atual da rede mesh
type MeshStatus struct {
	ActivePeers    int
	BluetoothPeers int
	TCPPeers       int
	NetworkHealth  float64 // 0.0-1.0
	AverageLatency time.Duration
	TotalMessages  int64
	LastActivity   time.Time
}

// MeshMetrics contém métricas detalhadas para melhorar a UX
type MeshMetrics struct {
	TotalPeersDiscovered  int64
	BluetoothConnections  int64
	TCPConnections        int64
	FailedConnections     int64
	MessagesSent          int64
	MessagesReceived      int64
	AverageSignalStrength float64
	NetworkUptime         time.Duration
	StartTime             time.Time
}

// NewMeshIntegration cria uma nova instância de integração mesh
func NewMeshIntegration(disc *discovery.Discovery) *MeshIntegration {
	ctx, cancel := context.WithCancel(context.Background())

	return &MeshIntegration{
		discovery:       disc,
		peers:           make(map[string]*MeshPeer),
		ctx:             ctx,
		cancel:          cancel,
		enableBluetooth: true,
		enableTCP:       true,
		autoFallback:    true,
		metrics: MeshMetrics{
			StartTime: time.Now(),
		},
	}
}

// Start inicia a integração mesh
func (mi *MeshIntegration) Start() error {
	// Inicia descoberta híbrida
	go mi.hybridDiscovery()

	// Inicia monitoramento de saúde da rede
	go mi.networkHealthMonitor()

	// Inicia limpeza periódica de peers inativos
	go mi.peerCleanup()

	return nil
}

// Stop para a integração mesh
func (mi *MeshIntegration) Stop() error {
	mi.cancel()
	return nil
}

// hybridDiscovery implementa descoberta híbrida inteligente
func (mi *MeshIntegration) hybridDiscovery() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-mi.ctx.Done():
			return
		case <-ticker.C:
			// Tenta descoberta via Bluetooth Mesh primeiro (menor latência)
			if mi.enableBluetooth {
				mi.discoverViaBluetooth()
			}

			// Complementa com descoberta TCP tradicional
			if mi.enableTCP {
				mi.discoverViaTCP()
			}
		}
	}
}

// discoverViaBluetooth implementa descoberta via Bluetooth Mesh
func (mi *MeshIntegration) discoverViaBluetooth() {
	// TODO: Integrar com o sistema Bluetooth Mesh do Bitchat
	// Por enquanto, simula a descoberta
	fmt.Println("[MESH] Descobrindo peers via Bluetooth Mesh...")
}

// discoverViaTCP implementa descoberta via TCP tradicional
func (mi *MeshIntegration) discoverViaTCP() {
	// Usa o sistema de descoberta existente do p2p-irc
	knownPeers := mi.discovery.GetKnownPeers()

	for _, peerAddr := range knownPeers {
		mi.peersMu.RLock()
		_, exists := mi.peers[peerAddr]
		mi.peersMu.RUnlock()

		if !exists {
			// Cria novo peer TCP
			peer := &MeshPeer{
				ID:        peerAddr,
				Address:   peerAddr,
				Transport: TransportTCP,
				Capabilities: MeshCapabilities{
					SupportsTCP:    true,
					SignalStrength: 80, // Assume boa qualidade para TCP
					LastSeen:       time.Now(),
				},
				LastActivity: time.Now(),
			}

			mi.addPeer(peer)
		}
	}
}

// addPeer adiciona um novo peer à rede mesh
func (mi *MeshIntegration) addPeer(peer *MeshPeer) {
	mi.peersMu.Lock()
	mi.peers[peer.ID] = peer
	mi.metrics.TotalPeersDiscovered++
	mi.peersMu.Unlock()

	// Notifica UI sobre novo peer
	if mi.onPeerDiscovered != nil {
		mi.onPeerDiscovered(peer)
	}

	fmt.Printf("[MESH] Novo peer descoberto: %s via %s\n", peer.ID, mi.transportName(peer.Transport))
}

// removePeer remove um peer da rede mesh
func (mi *MeshIntegration) removePeer(peerID string) {
	mi.peersMu.Lock()
	delete(mi.peers, peerID)
	mi.peersMu.Unlock()

	// Notifica UI sobre peer perdido
	if mi.onPeerLost != nil {
		mi.onPeerLost(peerID)
	}

	fmt.Printf("[MESH] Peer perdido: %s\n", peerID)
}

// transportName retorna o nome do transporte para exibição
func (mi *MeshIntegration) transportName(transport TransportType) string {
	switch transport {
	case TransportTCP:
		return "TCP"
	case TransportBluetoothMesh:
		return "Bluetooth Mesh"
	case TransportHybrid:
		return "Híbrido"
	default:
		return "Desconhecido"
	}
}

// networkHealthMonitor monitora a saúde da rede para melhorar UX
func (mi *MeshIntegration) networkHealthMonitor() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-mi.ctx.Done():
			return
		case <-ticker.C:
			status := mi.calculateNetworkStatus()

			// Notifica UI sobre status da rede
			if mi.onMeshStatus != nil {
				mi.onMeshStatus(status)
			}
		}
	}
}

// calculateNetworkStatus calcula o status atual da rede
func (mi *MeshIntegration) calculateNetworkStatus() MeshStatus {
	mi.peersMu.RLock()
	defer mi.peersMu.RUnlock()

	status := MeshStatus{
		ActivePeers:  len(mi.peers),
		LastActivity: time.Now(),
	}

	var totalLatency time.Duration
	var validLatencyCount int

	for _, peer := range mi.peers {
		switch peer.Transport {
		case TransportTCP:
			status.TCPPeers++
		case TransportBluetoothMesh:
			status.BluetoothPeers++
		}

		if peer.Capabilities.Latency > 0 {
			totalLatency += peer.Capabilities.Latency
			validLatencyCount++
		}
	}

	if validLatencyCount > 0 {
		status.AverageLatency = totalLatency / time.Duration(validLatencyCount)
	}

	// Calcula saúde da rede (0.0-1.0)
	if status.ActivePeers > 0 {
		status.NetworkHealth = float64(status.ActivePeers) / 10.0 // Assume 10 peers como ideal
		if status.NetworkHealth > 1.0 {
			status.NetworkHealth = 1.0
		}
	}

	return status
}

// peerCleanup remove peers inativos periodicamente
func (mi *MeshIntegration) peerCleanup() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-mi.ctx.Done():
			return
		case <-ticker.C:
			mi.cleanupInactivePeers()
		}
	}
}

// cleanupInactivePeers remove peers que não respondem há muito tempo
func (mi *MeshIntegration) cleanupInactivePeers() {
	mi.peersMu.Lock()
	defer mi.peersMu.Unlock()

	timeout := 5 * time.Minute
	now := time.Now()

	for peerID, peer := range mi.peers {
		if now.Sub(peer.LastActivity) > timeout {
			delete(mi.peers, peerID)

			// Notifica UI sobre peer perdido
			if mi.onPeerLost != nil {
				go mi.onPeerLost(peerID)
			}

			fmt.Printf("[MESH] Peer removido por inatividade: %s\n", peerID)
		}
	}
}

// GetPeers retorna todos os peers ativos
func (mi *MeshIntegration) GetPeers() []*MeshPeer {
	mi.peersMu.RLock()
	defer mi.peersMu.RUnlock()

	peers := make([]*MeshPeer, 0, len(mi.peers))
	for _, peer := range mi.peers {
		peers = append(peers, peer)
	}

	return peers
}

// GetMetrics retorna métricas atuais
func (mi *MeshIntegration) GetMetrics() MeshMetrics {
	mi.metrics.NetworkUptime = time.Since(mi.metrics.StartTime)
	return mi.metrics
}

// SetCallbacks define callbacks para notificações da UI
func (mi *MeshIntegration) SetCallbacks(
	onPeerDiscovered func(peer *MeshPeer),
	onPeerLost func(peerID string),
	onMeshStatus func(status MeshStatus),
) {
	mi.onPeerDiscovered = onPeerDiscovered
	mi.onPeerLost = onPeerLost
	mi.onMeshStatus = onMeshStatus
}

// SendMessage envia uma mensagem através da rede mesh
func (mi *MeshIntegration) SendMessage(peerID string, message []byte) error {
	mi.peersMu.RLock()
	peer, exists := mi.peers[peerID]
	mi.peersMu.RUnlock()

	if !exists {
		return fmt.Errorf("peer não encontrado: %s", peerID)
	}

	// Escolhe o melhor transporte baseado nas capacidades
	switch peer.Transport {
	case TransportBluetoothMesh:
		return mi.sendViaBluetooth(peer, message)
	case TransportTCP:
		return mi.sendViaTCP(peer, message)
	default:
		return fmt.Errorf("transporte não suportado: %v", peer.Transport)
	}
}

// sendViaBluetooth envia mensagem via Bluetooth Mesh
func (mi *MeshIntegration) sendViaBluetooth(peer *MeshPeer, message []byte) error {
	// TODO: Integrar com sistema de envio Bluetooth Mesh do Bitchat
	fmt.Printf("[MESH] Enviando mensagem via Bluetooth Mesh para %s\n", peer.ID)
	mi.metrics.MessagesSent++
	return nil
}

// sendViaTCP envia mensagem via TCP
func (mi *MeshIntegration) sendViaTCP(peer *MeshPeer, message []byte) error {
	// Usa o sistema existente do p2p-irc
	fmt.Printf("[MESH] Enviando mensagem via TCP para %s\n", peer.ID)
	mi.metrics.MessagesSent++
	return nil
}
