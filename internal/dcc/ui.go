package dcc

import (
	"fmt"
	"strings"
	"time"
)

// TransferView representa uma visualização de transferência para a UI
type TransferView struct {
	ID             string
	Filename       string
	Size           int64
	BytesTransferred int64
	Progress       float64  // 0-100
	Speed          float64  // bytes/segundo
	ETA            time.Duration
	Status         TransferStatus
	StatusText     string
	Direction      string  // "upload" ou "download"
	Peer           string
	StartTime      time.Time
	LastActive     time.Time
	Error          string
	CanPause       bool
	CanResume      bool
	CanCancel      bool
	CanRetry       bool
	Transfer       *Transfer
}

// UIManager gerencia a interface do usuário para transferências DCC
type UIManager struct {
	manager        *Manager
	updateCallback func([]TransferView)
	refreshRate    time.Duration
	stopChan       chan struct{}
}

// NewUIManager cria um novo gerenciador de UI para DCC
func NewUIManager(manager *Manager, callback func([]TransferView)) *UIManager {
	ui := &UIManager{
		manager:        manager,
		updateCallback: callback,
		refreshRate:    500 * time.Millisecond,
		stopChan:       make(chan struct{}),
	}

	// Define o callback no gerenciador DCC
	manager.SetUICallback(func(t *Transfer) {
		ui.updateTransferView()
	})

	// Inicia a atualização periódica
	go ui.periodicUpdate()

	return ui
}

// Stop interrompe as atualizações periódicas
func (ui *UIManager) Stop() {
	close(ui.stopChan)
}

// periodicUpdate atualiza a UI periodicamente
func (ui *UIManager) periodicUpdate() {
	ticker := time.NewTicker(ui.refreshRate)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ui.updateTransferView()
		case <-ui.stopChan:
			return
		}
	}
}

// updateTransferView atualiza a visualização de todas as transferências
func (ui *UIManager) updateTransferView() {
	if ui.updateCallback == nil {
		return
	}

	transfers := ui.manager.ListTransfers()
	views := make([]TransferView, 0, len(transfers))

	for _, t := range transfers {
		view := TransferView{
			ID:              t.ID,
			Filename:        t.Filename,
			Size:            t.Size,
			BytesTransferred: t.BytesTransferred,
			Progress:        float64(t.BytesTransferred) / float64(t.Size) * 100,
			Speed:           t.Speed,
			ETA:             t.ETA,
			Status:          t.Status,
			StatusText:      statusToString(t.Status),
			Direction:       directionString(t),
			Peer:            peerString(t),
			StartTime:       t.StartTime,
			LastActive:      t.LastActive,
			CanPause:        t.Status == StatusInProgress,
			CanResume:       t.Status == StatusPaused,
			CanCancel:       t.Status != StatusCompleted && t.Status != StatusCanceled,
			CanRetry:        t.Status == StatusFailed,
			Transfer:        t,
		}

		if t.Error != nil {
			view.Error = t.Error.Error()
		}

		views = append(views, view)
	}

	ui.updateCallback(views)
}

// FormatTransferInfo formata informações de transferência para exibição
func FormatTransferInfo(view TransferView) string {
	var info strings.Builder
	
	// Status e tipo
	info.WriteString(fmt.Sprintf("Status: %s (%s para %s)\n", 
		StatusToString(view.Status), 
		directionString(view.Transfer), 
		peerString(view.Transfer)))
	
	// Nome do arquivo
	info.WriteString(fmt.Sprintf("Arquivo: %s\n", view.Filename))
	
	// Barra de progresso
	progressBar := createProgressBar(view.Progress, 20)
	info.WriteString(fmt.Sprintf("%s %.1f%%\n", progressBar, view.Progress))
	
	// Tamanho
	info.WriteString(fmt.Sprintf("%s / %s", FormatSize(view.BytesTransferred), FormatSize(view.Size)))
	
	// Velocidade e ETA
	if view.Status == StatusInProgress {
		info.WriteString(fmt.Sprintf(" @ %s/s", FormatSize(int64(view.Speed))))
		if view.ETA > 0 {
			info.WriteString(fmt.Sprintf(", ETA: %s", FormatDuration(view.ETA)))
		}
	}
	info.WriteString("\n")
	
	// Tempo
	info.WriteString(fmt.Sprintf("Iniciado: %s", view.StartTime.Format("15:04:05")))
	if view.Status == StatusCompleted {
		elapsed := view.LastActive.Sub(view.StartTime)
		info.WriteString(fmt.Sprintf(", Duração: %s", FormatDuration(elapsed)))
	}
	info.WriteString("\n")
	
	// Erro
	if view.Error != "" {
		info.WriteString(fmt.Sprintf("Erro: %s\n", view.Error))
	}
	
	// Ações disponíveis
	actions := []string{}
	if view.CanPause {
		actions = append(actions, "Pausar")
	}
	if view.CanResume {
		actions = append(actions, "Retomar")
	}
	if view.CanCancel {
		actions = append(actions, "Cancelar")
	}
	if view.CanRetry {
		actions = append(actions, "Tentar novamente")
	}
	
	if len(actions) > 0 {
		info.WriteString("Ações: ")
		info.WriteString(strings.Join(actions, ", "))
		info.WriteString("\n")
	}
	
	return info.String()
}

// createProgressBar cria uma barra de progresso textual
func createProgressBar(progress float64, width int) string {
	completed := int(progress / 100 * float64(width))
	if completed > width {
		completed = width
	}
	
	var bar strings.Builder
	bar.WriteString("[")
	
	for i := 0; i < width; i++ {
		if i < completed {
			bar.WriteString("=")
		} else {
			bar.WriteString(" ")
		}
	}
	
	bar.WriteString("]")
	return bar.String()
}

func statusToString(status TransferStatus) string {
	switch status {
	case StatusPending:
		return "Pendente"
	case StatusInProgress:
		return "Em progresso"
	case StatusPaused:
		return "Pausado"
	case StatusCompleted:
		return "Concluído"
	case StatusFailed:
		return "Falhou"
	case StatusCanceled:
		return "Cancelado"
	default:
		return "Desconhecido"
	}
}

func directionString(t *Transfer) string {
	if t.Sender == "me" {
		return "Enviando"
	}
	return "Recebendo"
}

func peerString(t *Transfer) string {
	if t.Sender == "me" {
		return t.Receiver
	}
	return t.Sender
}
