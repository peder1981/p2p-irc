package dcc

import (
	"fmt"
	"time"
)

// FormatSize formata um tamanho em bytes para uma string legível
func FormatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// FormatDuration formata uma duração para uma string legível
func FormatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	
	if h > 0 {
		return fmt.Sprintf("%dh%02dm%02ds", h, m, s)
	} else if m > 0 {
		return fmt.Sprintf("%dm%02ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

// StatusToString converte um status de transferência para texto
func StatusToString(status TransferStatus) string {
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

// DirectionString retorna a direção da transferência (upload/download)
func DirectionString(t *Transfer) string {
	if t.Sender == "me" {
		return "Enviando"
	}
	return "Recebendo"
}

// PeerString retorna o nome do peer (remetente/destinatário)
func PeerString(t *Transfer) string {
	if t.Sender == "me" {
		return t.Receiver
	}
	return t.Sender
}
