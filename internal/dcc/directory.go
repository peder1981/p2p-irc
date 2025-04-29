package dcc

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// DirectoryTransfer representa uma transferência de diretório
type DirectoryTransfer struct {
	ID               string
	DirPath          string
	DestPath         string
	Sender           string
	Receiver         string
	Files            []string
	TotalSize        int64
	TotalTransferred int64
	Progress         float64
	Status           TransferStatus
	StartTime        time.Time
	EndTime          time.Time
	Error            error
	Transfers        []*Transfer
	mu               sync.Mutex
	logger           *log.Logger
}

// SendDirectory inicia uma transferência de diretório
func (m *Manager) SendDirectory(dirPath string, receiver string) (*DirectoryTransfer, error) {
	// Verifica se o diretório existe
	info, err := os.Stat(dirPath)
	if err != nil {
		return nil, fmt.Errorf("erro ao acessar diretório: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("o caminho especificado não é um diretório")
	}

	// Cria a transferência de diretório
	dt := &DirectoryTransfer{
		ID:        uuid.New().String(),
		DirPath:   dirPath,
		Sender:    "me",
		Receiver:  receiver,
		Status:    StatusPending,
		StartTime: time.Now(),
		Files:     make([]string, 0),
		Transfers: make([]*Transfer, 0),
		logger:    m.logger,
	}

	// Coleta todos os arquivos do diretório
	err = filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			relPath, err := filepath.Rel(dirPath, path)
			if err != nil {
				return err
			}
			dt.Files = append(dt.Files, relPath)
			dt.TotalSize += info.Size()
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("erro ao listar arquivos do diretório: %w", err)
	}

	if len(dt.Files) == 0 {
		return nil, fmt.Errorf("diretório vazio, nada para transferir")
	}

	m.logger.Printf("Iniciando transferência do diretório %s com %d arquivos (%s)",
		dirPath, len(dt.Files), FormatSize(dt.TotalSize))

	// Registra a transferência de diretório
	m.mu.Lock()
	if m.dirTransfers == nil {
		m.dirTransfers = make(map[string]*DirectoryTransfer)
	}
	m.dirTransfers[dt.ID] = dt
	m.mu.Unlock()

	// Inicia a transferência dos arquivos
	go m.processDirectoryTransfer(dt)

	return dt, nil
}

// processDirectoryTransfer processa uma transferência de diretório
func (m *Manager) processDirectoryTransfer(dt *DirectoryTransfer) {
	dt.mu.Lock()
	dt.Status = StatusInProgress
	dt.mu.Unlock()

	// Diretório base para o nome do diretório
	baseDirName := filepath.Base(dt.DirPath)

	// Transfere cada arquivo
	for _, relPath := range dt.Files {
		// Caminho completo do arquivo
		fullPath := filepath.Join(dt.DirPath, relPath)

		// Nome do arquivo para transferência (mantém estrutura de diretórios)
		transferName := filepath.Join(baseDirName, relPath)
		transferName = strings.ReplaceAll(transferName, "\\", "/") // Normaliza separadores

		// Inicia transferência do arquivo
		transfer, err := m.SendFile(fullPath, dt.Receiver)
		if err != nil {
			m.logger.Printf("Erro ao iniciar transferência de %s: %v", fullPath, err)
			continue
		}

		// Adiciona à lista de transferências
		dt.mu.Lock()
		dt.Transfers = append(dt.Transfers, transfer)
		dt.mu.Unlock()

		// Aguarda um pouco para não sobrecarregar
		time.Sleep(100 * time.Millisecond)
	}

	// Monitora o progresso das transferências
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		<-ticker.C

		completed := true
		totalTransferred := int64(0)
		failed := 0

		dt.mu.Lock()
		for _, t := range dt.Transfers {
			if t.Status != StatusCompleted && t.Status != StatusFailed && t.Status != StatusCanceled {
				completed = false
			}
			if t.Status == StatusFailed || t.Status == StatusCanceled {
				failed++
			}
			totalTransferred += t.BytesTransferred
		}

		dt.TotalTransferred = totalTransferred
		dt.Progress = float64(totalTransferred) / float64(dt.TotalSize) * 100

		if completed {
			if failed == len(dt.Transfers) {
				dt.Status = StatusFailed
			} else if failed > 0 {
				dt.Status = StatusCompleted // Parcialmente completo
			} else {
				dt.Status = StatusCompleted
			}
			dt.EndTime = time.Now()
			dt.mu.Unlock()
			break
		}
		dt.mu.Unlock()
	}

	m.logger.Printf("Transferência do diretório %s concluída com status %d", 
		dt.DirPath, dt.Status)
}

// ReceiveDirectory prepara o recebimento de um diretório
func (m *Manager) ReceiveDirectory(sender string, dirName string) (*DirectoryTransfer, error) {
	// Cria a transferência de diretório
	dt := &DirectoryTransfer{
		ID:        uuid.New().String(),
		DestPath:  filepath.Join(m.downloadDir, dirName),
		Sender:    sender,
		Receiver:  "me",
		Status:    StatusPending,
		StartTime: time.Now(),
		Files:     make([]string, 0),
		Transfers: make([]*Transfer, 0),
		logger:    m.logger,
	}

	// Cria o diretório de destino
	if err := os.MkdirAll(dt.DestPath, 0755); err != nil {
		return nil, fmt.Errorf("erro ao criar diretório de destino: %w", err)
	}

	// Registra a transferência de diretório
	m.mu.Lock()
	if m.dirTransfers == nil {
		m.dirTransfers = make(map[string]*DirectoryTransfer)
	}
	m.dirTransfers[dt.ID] = dt
	m.mu.Unlock()

	return dt, nil
}

// PauseDirectoryTransfer pausa todas as transferências de um diretório
func (m *Manager) PauseDirectoryTransfer(dt *DirectoryTransfer) error {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	if dt.Status != StatusInProgress {
		return fmt.Errorf("transferência de diretório não está em progresso")
	}

	for _, t := range dt.Transfers {
		if t.Status == StatusInProgress {
			m.PauseTransfer(t.ID)
		}
	}

	dt.Status = StatusPaused
	return nil
}

// ResumeDirectoryTransfer retoma todas as transferências de um diretório
func (m *Manager) ResumeDirectoryTransfer(dt *DirectoryTransfer) error {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	if dt.Status != StatusPaused {
		return fmt.Errorf("transferência de diretório não está pausada")
	}

	for _, t := range dt.Transfers {
		if t.Status == StatusPaused {
			m.ResumeTransfer(t.ID)
		}
	}

	dt.Status = StatusInProgress
	return nil
}

// CancelDirectoryTransfer cancela todas as transferências de um diretório
func (m *Manager) CancelDirectoryTransfer(dt *DirectoryTransfer) error {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	if dt.Status == StatusCompleted || dt.Status == StatusCanceled {
		return fmt.Errorf("transferência de diretório já finalizada")
	}

	for _, t := range dt.Transfers {
		if t.Status != StatusCompleted && t.Status != StatusCanceled {
			m.CancelTransfer(t.ID)
		}
	}

	dt.Status = StatusCanceled
	return nil
}

// ListDirectoryTransfers retorna todas as transferências de diretórios
func (m *Manager) ListDirectoryTransfers() []*DirectoryTransfer {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if m.dirTransfers == nil {
		return []*DirectoryTransfer{}
	}
	
	dirTransfers := make([]*DirectoryTransfer, 0, len(m.dirTransfers))
	for _, dt := range m.dirTransfers {
		dirTransfers = append(dirTransfers, dt)
	}
	return dirTransfers
}

// GetDirectoryTransfer retorna uma transferência de diretório pelo ID
func (m *Manager) GetDirectoryTransfer(id string) (*DirectoryTransfer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if m.dirTransfers == nil {
		return nil, fmt.Errorf("transferência de diretório não encontrada")
	}
	
	dt, ok := m.dirTransfers[id]
	if !ok {
		return nil, fmt.Errorf("transferência de diretório não encontrada")
	}
	return dt, nil
}
