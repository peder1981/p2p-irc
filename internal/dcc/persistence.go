package dcc

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// TransferState representa o estado persistido de uma transferência
type TransferState struct {
	ID              string    `json:"id"`
	Filename        string    `json:"filename"`
	Size            int64     `json:"size"`
	BytesTransferred int64    `json:"bytes_transferred"`
	Sender          string    `json:"sender"`
	Receiver        string    `json:"receiver"`
	Port            int       `json:"port"`
	Status          int       `json:"status"`
	StartTime       time.Time `json:"start_time"`
	LastActive      time.Time `json:"last_active"`
	RetryCount      int       `json:"retry_count"`
	Hash            string    `json:"hash,omitempty"`
	Path            string    `json:"path,omitempty"` // Caminho completo do arquivo
	Error           string    `json:"error,omitempty"`
}

// DirectoryTransferState representa o estado persistido de uma transferência de diretório
type DirectoryTransferState struct {
	ID               string         `json:"id"`
	DirPath          string         `json:"dir_path"`
	DestPath         string         `json:"dest_path"`
	Sender           string         `json:"sender"`
	Receiver         string         `json:"receiver"`
	Files            []string       `json:"files"`
	TotalSize        int64          `json:"total_size"`
	TotalTransferred int64          `json:"total_transferred"`
	Progress         float64        `json:"progress"`
	Status           int            `json:"status"`
	StartTime        time.Time      `json:"start_time"`
	EndTime          time.Time      `json:"end_time"`
	Transfers        []TransferState `json:"transfers"`
	Error            string         `json:"error,omitempty"`
}

// PersistenceState representa o estado completo persistido
type PersistenceState struct {
	Transfers        []TransferState        `json:"transfers"`
	DirectoryTransfers []DirectoryTransferState `json:"directory_transfers"`
}

// PersistenceManager gerencia a persistência de transferências
type PersistenceManager struct {
	stateFile string
	manager   *Manager
	mu        sync.Mutex
}

// NewPersistenceManager cria um novo gerenciador de persistência
func NewPersistenceManager(manager *Manager, stateFile string) *PersistenceManager {
	if stateFile == "" {
		// Usa o diretório de download como base
		stateFile = filepath.Join(manager.downloadDir, "dcc_transfers.json")
	}

	pm := &PersistenceManager{
		stateFile: stateFile,
		manager:   manager,
	}

	// Configura o callback para salvar estado quando transferências mudarem
	manager.SetUICallback(func(t *Transfer) {
		pm.SaveState()
	})

	return pm
}

// SaveState salva o estado atual de todas as transferências
func (pm *PersistenceManager) SaveState() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	state := PersistenceState{
		Transfers:         make([]TransferState, 0),
		DirectoryTransfers: make([]DirectoryTransferState, 0),
	}

	// Salva transferências individuais
	transfers := pm.manager.ListTransfers()
	for _, t := range transfers {
		// Só salva transferências que podem ser retomadas
		if t.Status == StatusPaused || t.Status == StatusFailed || 
		   (t.Status == StatusInProgress && t.BytesTransferred > 0) {
			transferState := TransferState{
				ID:              t.ID,
				Filename:        t.Filename,
				Size:            t.Size,
				BytesTransferred: t.BytesTransferred,
				Sender:          t.Sender,
				Receiver:        t.Receiver,
				Port:            t.Port,
				Status:          int(t.Status),
				StartTime:       t.StartTime,
				LastActive:      t.LastActive,
				RetryCount:      t.RetryCount,
				Hash:            t.Hash,
			}

			// Adiciona caminho completo para uploads
			if t.Sender == "me" {
				transferState.Path = filepath.Join(pm.manager.downloadDir, t.Filename)
			}

			if t.Error != nil {
				transferState.Error = t.Error.Error()
			}

			state.Transfers = append(state.Transfers, transferState)
		}
	}

	// Salva transferências de diretórios
	dirTransfers := pm.manager.ListDirectoryTransfers()
	for _, dt := range dirTransfers {
		// Só salva transferências que estão em andamento, pausadas ou com erro
		if dt.Status == StatusPaused || dt.Status == StatusFailed || 
		   dt.Status == StatusInProgress {
			
			dtState := DirectoryTransferState{
				ID:               dt.ID,
				DirPath:          dt.DirPath,
				DestPath:         dt.DestPath,
				Sender:           dt.Sender,
				Receiver:         dt.Receiver,
				Files:            dt.Files,
				TotalSize:        dt.TotalSize,
				TotalTransferred: dt.TotalTransferred,
				Progress:         dt.Progress,
				Status:           int(dt.Status),
				StartTime:        dt.StartTime,
				EndTime:          dt.EndTime,
				Transfers:        make([]TransferState, 0),
			}

			if dt.Error != nil {
				dtState.Error = dt.Error.Error()
			}

			// Salva estado de cada transferência de arquivo no diretório
			for _, t := range dt.Transfers {
				tState := TransferState{
					ID:              t.ID,
					Filename:        t.Filename,
					Size:            t.Size,
					BytesTransferred: t.BytesTransferred,
					Sender:          t.Sender,
					Receiver:        t.Receiver,
					Port:            t.Port,
					Status:          int(t.Status),
					StartTime:       t.StartTime,
					LastActive:      t.LastActive,
					RetryCount:      t.RetryCount,
					Hash:            t.Hash,
				}

				if t.Error != nil {
					tState.Error = t.Error.Error()
				}

				dtState.Transfers = append(dtState.Transfers, tState)
			}

			state.DirectoryTransfers = append(state.DirectoryTransfers, dtState)
		}
	}

	// Cria diretório se não existir
	dir := filepath.Dir(pm.stateFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("erro ao criar diretório para estado: %w", err)
	}

	// Serializa para JSON
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("erro ao serializar estado: %w", err)
	}

	// Escreve no arquivo
	if err := os.WriteFile(pm.stateFile, data, 0644); err != nil {
		return fmt.Errorf("erro ao salvar estado: %w", err)
	}

	return nil
}

// LoadState carrega o estado salvo de transferências
func (pm *PersistenceManager) LoadState() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Verifica se o arquivo existe
	if _, err := os.Stat(pm.stateFile); os.IsNotExist(err) {
		return nil // Arquivo não existe, não é um erro
	}

	// Lê o arquivo
	data, err := os.ReadFile(pm.stateFile)
	if err != nil {
		return fmt.Errorf("erro ao ler arquivo de estado: %w", err)
	}

	// Tenta primeiro deserializar o novo formato
	var state PersistenceState
	err = json.Unmarshal(data, &state)
	
	// Se falhar, tenta o formato antigo (apenas transferências individuais)
	if err != nil {
		var oldStates []TransferState
		if err := json.Unmarshal(data, &oldStates); err != nil {
			return fmt.Errorf("erro ao deserializar estado: %w", err)
		}
		state.Transfers = oldStates
	}

	// Restaura transferências individuais
	for _, transferState := range state.Transfers {
		var t *Transfer
		var err error

		// Recria a transferência com base na direção
		if transferState.Sender == "me" {
			// É um upload (envio)
			if transferState.Path == "" {
				// Se não temos o caminho, não podemos retomar
				continue
			}
			t, err = pm.manager.SendFile(transferState.Path, transferState.Receiver)
		} else {
			// É um download (recebimento)
			t, err = pm.manager.ReceiveFile(transferState.Sender, transferState.Filename, transferState.Size, transferState.Port)
		}

		if err != nil {
			continue // Pula esta transferência se houver erro
		}

		// Restaura o estado
		t.mu.Lock()
		t.ID = transferState.ID
		t.BytesTransferred = transferState.BytesTransferred
		t.Status = TransferStatus(transferState.Status)
		t.StartTime = transferState.StartTime
		t.LastActive = transferState.LastActive
		t.RetryCount = transferState.RetryCount
		t.Hash = transferState.Hash
		if transferState.Error != "" {
			t.Error = fmt.Errorf(transferState.Error)
		}
		t.mu.Unlock()

		// Adiciona à fila se necessário
		if t.Status == StatusPaused || t.Status == StatusFailed {
			pm.manager.queue.mu.Lock()
			pm.manager.queue.items.PushBack(t)
			pm.manager.queue.mu.Unlock()
		}
	}

	// Restaura transferências de diretórios
	for _, dtState := range state.DirectoryTransfers {
		var dt *DirectoryTransfer
		var err error

		// Recria a transferência de diretório com base na direção
		if dtState.Sender == "me" {
			// É um upload (envio)
			if dtState.DirPath == "" {
				// Se não temos o caminho, não podemos retomar
				continue
			}
			dt, err = pm.manager.SendDirectory(dtState.DirPath, dtState.Receiver)
		} else {
			// É um download (recebimento)
			dt, err = pm.manager.ReceiveDirectory(dtState.Sender, filepath.Base(dtState.DestPath))
		}

		if err != nil {
			continue // Pula esta transferência se houver erro
		}

		// Restaura o estado
		dt.mu.Lock()
		dt.ID = dtState.ID
		dt.Files = dtState.Files
		dt.TotalSize = dtState.TotalSize
		dt.TotalTransferred = dtState.TotalTransferred
		dt.Progress = dtState.Progress
		dt.Status = TransferStatus(dtState.Status)
		dt.StartTime = dtState.StartTime
		dt.EndTime = dtState.EndTime
		if dtState.Error != "" {
			dt.Error = fmt.Errorf(dtState.Error)
		}
		dt.mu.Unlock()

		// Não precisamos restaurar as transferências individuais, pois elas
		// já foram restauradas no loop anterior
	}

	return nil
}

// AutoSave inicia salvamento automático periódico
func (pm *PersistenceManager) AutoSave(interval time.Duration) chan struct{} {
	stopChan := make(chan struct{})
	ticker := time.NewTicker(interval)

	go func() {
		for {
			select {
			case <-ticker.C:
				pm.SaveState()
			case <-stopChan:
				ticker.Stop()
				return
			}
		}
	}()

	return stopChan
}
