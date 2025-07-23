package ui

import (
	"fmt"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/widget"
)

// PeerInfo contém os dados de um peer para exibição na UI.
type PeerInfo struct {
	ID      string
	Address string
}

// MeshUI gerencia a interface do usuário para a funcionalidade de mesh.
type MeshUI struct {
	mu       sync.RWMutex
	peers    binding.StringList // Data binding for the peer list
	logs     []string
	logLimit int
	gui      *GUI // Referência à GUI principal para chamadas thread-safe

	// Componentes da UI
	statusLabel *widget.Label
	peerList    *widget.List
	logView     *widget.TextGrid
	view        fyne.CanvasObject
}

// NewMeshUI cria uma nova instância da interface mesh
func NewMeshUI(gui *GUI) *MeshUI {
	mui := &MeshUI{
		peers:    binding.NewStringList(),
		logs:     make([]string, 0),
		logLimit: 100,
		gui:      gui,
	}
	mui.view = mui.CreateMeshPanel()
	return mui
}

// CreateMeshPanel cria e retorna o painel da UI para o status da mesh.
func (mui *MeshUI) CreateMeshPanel() fyne.CanvasObject {
	mui.statusLabel = widget.NewLabel("Status da Mesh: Desconhecido")
	mui.logView = widget.NewTextGrid()

	mui.peerList = widget.NewListWithData(mui.peers,
		func() fyne.CanvasObject {
			return widget.NewLabel("template")
		},
		func(item binding.DataItem, obj fyne.CanvasObject) {
			label := obj.(*widget.Label)
			str, _ := item.(binding.String).Get()
			label.SetText(str)
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
	peerStrings := make([]string, len(peers))
	for i, p := range peers {
		peerStrings[i] = fmt.Sprintf("%s (%s)", p.ID, p.Address)
	}

	mui.gui.RunOnMain(func() {
		mui.mu.Lock()
		defer mui.mu.Unlock()

		if mui.peers != nil {
			mui.peers.Set(peerStrings)
		}
	})
}

// Log adiciona uma entrada de log e atualiza o status se aplicável.
func (mui *MeshUI) Log(message string) {
	mui.mu.Lock()
	mui.logs = append(mui.logs, message)
	if len(mui.logs) > mui.logLimit {
		mui.logs = mui.logs[len(mui.logs)-mui.logLimit:]
	}
	logText := strings.Join(mui.logs, "\n")
	mui.mu.Unlock()

	mui.gui.RunOnMain(func() {
		if mui.logView != nil {
			mui.logView.SetText(logText)
		}

		if mui.statusLabel != nil && strings.HasPrefix(message, "[STATUS]") {
			mui.statusLabel.SetText(fmt.Sprintf("Status da Mesh: %s", strings.TrimSpace(strings.TrimPrefix(message, "[STATUS]"))))
		}
	})
}

// GetView retorna o objeto Canvas para a UI principal.
func (mui *MeshUI) GetView() fyne.CanvasObject {
	return mui.view
}
