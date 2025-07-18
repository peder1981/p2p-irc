package ui

import (
	"fmt"
	"time"

	"github.com/peder1981/p2p-irc/internal/mesh"
)

// MeshUI gerencia a interface básica para informações mesh
type MeshUI struct {
	// Dados
	peersData []mesh.MeshPeer
	logsData  []string

	// Callbacks
	onPeerDiscovered func(*mesh.MeshPeer)
	onPeerLost       func(string)
	onStatusChanged  func(mesh.MeshStatus)

	// Estado interno
	isVisible  bool
	lastUpdate time.Time
}

// NewMeshUI cria uma nova instância da interface mesh
func NewMeshUI() *MeshUI {
	return &MeshUI{
		peersData: make([]mesh.MeshPeer, 0),
		logsData:  make([]string, 0, 100),
		isVisible: false,
	}
}

// UpdateStatus atualiza o status da conexão mesh
func (mui *MeshUI) UpdateStatus(status mesh.MeshStatus) {
	mui.lastUpdate = time.Now()

	// Chama callback se definido
	if mui.onStatusChanged != nil {
		mui.onStatusChanged(status)
	}
}

// UpdatePeers atualiza a lista de peers
func (mui *MeshUI) UpdatePeers(peers []mesh.MeshPeer) {
	mui.peersData = make([]mesh.MeshPeer, len(peers))
	copy(mui.peersData, peers)
}

// AddPeer adiciona um novo peer à lista
func (mui *MeshUI) AddPeer(peer *mesh.MeshPeer) {
	// Verifica se peer já existe
	for i, existingPeer := range mui.peersData {
		if existingPeer.ID == peer.ID {
			// Atualiza peer existente
			mui.peersData[i] = *peer
			return
		}
	}

	// Adiciona novo peer
	mui.peersData = append(mui.peersData, *peer)

	// Adiciona log
	mui.AddLog(fmt.Sprintf("Peer descoberto: %s", peer.ID))

	// Chama callback se definido
	if mui.onPeerDiscovered != nil {
		mui.onPeerDiscovered(peer)
	}
}

// RemovePeer remove um peer da lista
func (mui *MeshUI) RemovePeer(peerID string) {
	for i, peer := range mui.peersData {
		if peer.ID == peerID {
			// Remove peer da lista
			mui.peersData = append(mui.peersData[:i], mui.peersData[i+1:]...)

			// Adiciona log
			mui.AddLog(fmt.Sprintf("Peer desconectado: %s", peerID))

			// Chama callback se definido
			if mui.onPeerLost != nil {
				mui.onPeerLost(peerID)
			}

			return
		}
	}
}

// AddLog adiciona uma entrada de log
func (mui *MeshUI) AddLog(message string) {
	timestamp := time.Now().Format("15:04:05")
	logEntry := fmt.Sprintf("[%s] %s", timestamp, message)

	// Adiciona no início da lista
	mui.logsData = append([]string{logEntry}, mui.logsData...)

	// Limita número de logs
	if len(mui.logsData) > 100 {
		mui.logsData = mui.logsData[:100]
	}
}

// SetCallbacks define callbacks para eventos mesh
func (mui *MeshUI) SetCallbacks(
	onPeerDiscovered func(*mesh.MeshPeer),
	onPeerLost func(string),
	onStatusChanged func(mesh.MeshStatus),
) {
	mui.onPeerDiscovered = onPeerDiscovered
	mui.onPeerLost = onPeerLost
	mui.onStatusChanged = onStatusChanged
}

// Show exibe a interface mesh
func (mui *MeshUI) Show() {
	mui.isVisible = true
	mui.AddLog("Interface mesh ativada")
}

// Hide oculta a interface mesh
func (mui *MeshUI) Hide() {
	mui.isVisible = false
	mui.AddLog("Interface mesh desativada")
}

// IsVisible retorna se a interface está visível
func (mui *MeshUI) IsVisible() bool {
	return mui.isVisible
}

// ShowNotification exibe uma notificação
func (mui *MeshUI) ShowNotification(title, message string) {
	mui.AddLog(fmt.Sprintf("NOTIFICAÇÃO: %s - %s", title, message))
}

// GetPeersCount retorna o número de peers conectados
func (mui *MeshUI) GetPeersCount() int {
	count := 0
	for _, peer := range mui.peersData {
		// Considera conectado se atividade recente
		if time.Since(peer.LastActivity) < 2*time.Minute {
			count++
		}
	}
	return count
}

// GetPeers retorna a lista de peers
func (mui *MeshUI) GetPeers() []mesh.MeshPeer {
	return mui.peersData
}

// GetLogs retorna os logs
func (mui *MeshUI) GetLogs() []string {
	return mui.logsData
}
