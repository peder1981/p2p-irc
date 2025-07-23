package ui

import (
	"fmt"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// PeerInfo contém os dados de um peer para exibição na UI.
type PeerInfo struct {
	ID      string
	Address string
}

// MeshUI gerencia a interface do usuário para a funcionalidade de mesh.
type MeshUI struct {
	mu        sync.RWMutex
	peersData []PeerInfo
	logs      []string
	logLimit  int

	// Componentes da UI
	statusLabel *widget.Label
	peerList    *widget.List
	logView     *widget.TextGrid
	view        fyne.CanvasObject
}

// NewMeshUI cria uma nova instância da interface mesh
func NewMeshUI() *MeshUI {
	mui := &MeshUI{
		peersData: make([]PeerInfo, 0),
		logs:      make([]string, 0),
		logLimit:  100,
	}
	mui.view = mui.CreateMeshPanel()
	return mui
}

// CreateMeshPanel cria e retorna o painel da UI para o status da mesh.
func (mui *MeshUI) CreateMeshPanel() fyne.CanvasObject {
	mui.statusLabel = widget.NewLabel("Status da Mesh: Desconhecido")
	mui.logView = widget.NewTextGrid()

	mui.peerList = widget.NewList(
		func() int {
			mui.mu.RLock()
			defer mui.mu.RUnlock()
			return len(mui.peersData)
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("template")
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			mui.mu.RLock()
			defer mui.mu.RUnlock()
			if i < len(mui.peersData) {
				peer := mui.peersData[i]
				o.(*widget.Label).SetText(fmt.Sprintf("%s (%s)", peer.ID, peer.Address))
			}
		},
	)

	peerContainer := container.NewBorder(widget.NewLabel("Peers da Mesh"), nil, nil, nil, mui.peerList)
	logContainer := container.NewBorder(widget.NewLabel("Logs da Mesh"), nil, nil, nil, container.NewScroll(mui.logView))

	return container.NewVSplit(
		container.NewVBox(mui.statusLabel, peerContainer),
		logContainer,
	)
}

// UpdatePeerList atualiza a lista de peers
func (mui *MeshUI) UpdatePeerList(peers []PeerInfo) {
	mui.mu.Lock()
	defer mui.mu.Unlock()

	mui.peersData = peers

	if mui.peerList != nil {
		mui.peerList.Refresh()
	}
}

// Log adiciona uma entrada de log e atualiza o status se aplicável.
func (mui *MeshUI) Log(message string) {
	mui.mu.Lock()
	defer mui.mu.Unlock()

	mui.logs = append(mui.logs, message)
	if len(mui.logs) > mui.logLimit {
		mui.logs = mui.logs[len(mui.logs)-mui.logLimit:]
	}

	if mui.logView != nil {
		mui.logView.SetText(strings.Join(mui.logs, "\n"))
	}

	if mui.statusLabel != nil && strings.HasPrefix(message, "[STATUS]") {
		mui.statusLabel.SetText(fmt.Sprintf("Status da Mesh: %s", strings.TrimSpace(strings.TrimPrefix(message, "[STATUS]"))))
	}
}

// GetView retorna o objeto Canvas para a UI principal.
func (mui *MeshUI) GetView() fyne.CanvasObject {
	return mui.view
}
