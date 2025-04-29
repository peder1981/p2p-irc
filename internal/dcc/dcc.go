package dcc

import (
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "io"
    "log"
    "net"
    "os"
    "path/filepath"
    "strconv"
    "sync"
    "time"
    "container/list"
    "github.com/google/uuid"
    "github.com/peder1981/p2p-irc/internal/portmanager"
    "github.com/schollz/progressbar/v3"
    "io/ioutil"
)

// TransferStatus representa o estado atual de uma transferência
type TransferStatus int

const (
    StatusPending TransferStatus = iota
    StatusInProgress
    StatusPaused
    StatusCompleted
    StatusFailed
    StatusCanceled
    
    MaxConcurrentTransfers = 5
    MaxRetryTimeout       = 3 * time.Minute
    RetryInterval        = 10 * time.Second
    MaxRetries           = 3
)

// Opções para configuração do gerenciador DCC
type ManagerOptions struct {
    DownloadDir    string      // Diretório para download de arquivos
    BufferSize     int         // Tamanho do buffer para transferência (bytes)
    MaxConcurrent  int         // Máximo de transferências concorrentes
    LogEnabled     bool        // Habilita logging detalhado
    LogFile        string      // Arquivo de log (vazio para stdout)
    VerifyHash     bool        // Verifica hash dos arquivos transferidos
}

// Transfer representa uma transferência DCC
type Transfer struct {
    ID             string
    Filename       string
    Size           int64
    Sender         string
    Receiver       string
    Port           int
    Status         TransferStatus
    Progress       float64
    StartTime      time.Time
    EndTime        time.Time
    Error          error
    Hash           string    // Hash SHA-256 do arquivo
    Offset         int64     // Para retomar transferências
    Speed          float64   // Velocidade em bytes/s
    ETA            time.Duration // Tempo estimado restante
    LastActive     time.Time    // Última vez que houve atividade
    RetryCount     int         // Número de tentativas de retry
    BytesTransferred int64
    bar            *progressbar.ProgressBar
    resumable      bool
    pauseChan      chan struct{} // Canal para controle de pause/resume
    resumeChan     chan struct{} // Canal para controle de resume
    cancelChan     chan struct{} // Canal para controle de cancelamento
    mu             sync.Mutex    // Mutex para proteção de acesso concorrente
    logger         *log.Logger   // Logger para registro de eventos
    bufferSize     int           // Tamanho do buffer para transferência
}

// Manager gerencia transferências DCC
type Manager struct {
    downloadDir  string
    portManager  *portmanager.PortManager
    queue       *TransferQueue
    transfers   map[string]*Transfer
    dirTransfers map[string]*DirectoryTransfer
    mu          sync.RWMutex
    options     ManagerOptions
    logger      *log.Logger
    uiCallback  func(*Transfer)
    persistence *PersistenceManager
}

// NewManager cria um novo gerenciador DCC com opções padrão
func NewManager(downloadDir string) *Manager {
    return NewManagerWithOptions(ManagerOptions{
        DownloadDir:    downloadDir,
        BufferSize:     32 * 1024, // 32KB
        MaxConcurrent:  5,
        LogEnabled:     false,
        VerifyHash:     true,
    })
}

// NewManagerWithOptions cria um novo gerenciador DCC com opções personalizadas
func NewManagerWithOptions(options ManagerOptions) *Manager {
    // Usa valores padrão para opções não definidas
    if options.DownloadDir == "" {
        options.DownloadDir = "downloads"
    }
    if options.BufferSize <= 0 {
        options.BufferSize = 32 * 1024 // 32KB
    }
    if options.MaxConcurrent <= 0 {
        options.MaxConcurrent = 5
    }
    
    // Cria diretório de download se não existir
    os.MkdirAll(options.DownloadDir, 0755)
    
    // Configura logger
    var logger *log.Logger
    if options.LogEnabled {
        if options.LogFile != "" {
            logFile, err := os.OpenFile(options.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
            if err == nil {
                logger = log.New(logFile, "DCC: ", log.LstdFlags)
            } else {
                logger = log.New(os.Stdout, "DCC: ", log.LstdFlags)
                logger.Printf("Erro ao abrir arquivo de log %s: %v, usando stdout", options.LogFile, err)
            }
        } else {
            logger = log.New(os.Stdout, "DCC: ", log.LstdFlags)
        }
    } else {
        logger = log.New(ioutil.Discard, "", 0)
    }
    
    m := &Manager{
        downloadDir:  options.DownloadDir,
        portManager:  portmanager.New(),
        queue:       &TransferQueue{
            items:      list.New(),
            processing: make(map[string]*Transfer),
            maxConcurrent: options.MaxConcurrent,
        },
        transfers:   make(map[string]*Transfer),
        dirTransfers: make(map[string]*DirectoryTransfer),
        mu:          sync.RWMutex{},
        options:     options,
        logger:      logger,
    }

    // Inicia processamento da fila
    go func() {
        for {
            m.processQueue()
            time.Sleep(100 * time.Millisecond)
        }
    }()

    // Configura persistência
    m.persistence = NewPersistenceManager(m, filepath.Join(options.DownloadDir, "dcc_state.json"))
    
    // Tenta carregar transferências salvas
    if err := m.persistence.LoadState(); err != nil {
        m.logger.Printf("Erro ao carregar estado de transferências: %v", err)
    }
    
    // Configura salvamento automático a cada 30 segundos
    m.persistence.AutoSave(30 * time.Second)

    m.logger.Printf("Gerenciador DCC iniciado com diretório de download: %s", options.DownloadDir)
    return m
}

// SetUICallback define uma função de callback para atualizar a UI
func (m *Manager) SetUICallback(callback func(*Transfer)) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.uiCallback = callback
}

// updateUI notifica a UI sobre mudanças na transferência
func (m *Manager) updateUI(t *Transfer) {
    m.mu.RLock()
    callback := m.uiCallback
    m.mu.RUnlock()
    
    if callback != nil {
        callback(t)
    }
}

// TransferQueue gerencia a fila de transferências
type TransferQueue struct {
    items       *list.List
    processing  map[string]*Transfer
    mu          sync.RWMutex
    maxConcurrent int
}

// SendFile inicia uma transferência DCC
func (m *Manager) SendFile(filename string, receiver string) (*Transfer, error) {
    // Abre arquivo para verificar se existe
    file, err := os.Open(filename)
    if err != nil {
        return nil, fmt.Errorf("erro ao abrir arquivo: %w", err)
    }
    defer file.Close()

    // Obtém informações do arquivo
    info, err := file.Stat()
    if err != nil {
        return nil, fmt.Errorf("erro ao obter informações do arquivo: %w", err)
    }

    // Obtém porta disponível
    port, err := m.portManager.GetAvailablePort()
    if err != nil {
        return nil, fmt.Errorf("erro ao obter porta: %w", err)
    }

    // Calcula hash do arquivo se verificação estiver habilitada
    var hash string
    if m.options.VerifyHash {
        hash, err = calculateFileHash(filename)
        if err != nil {
            m.logger.Printf("Aviso: não foi possível calcular hash do arquivo %s: %v", filename, err)
        } else {
            m.logger.Printf("Hash calculado para %s: %s", filename, hash)
        }
    }

    // Cria transferência
    transfer := &Transfer{
        ID:        uuid.New().String(),
        Filename:  filepath.Base(filename),
        Size:      info.Size(),
        Sender:    "me",
        Receiver:  receiver,
        Port:      port,
        Status:    StatusPending,
        StartTime: time.Now(),
        LastActive: time.Now(),
        pauseChan: make(chan struct{}),
        resumeChan: make(chan struct{}),
        cancelChan: make(chan struct{}),
        logger:    m.logger,
        bufferSize: m.options.BufferSize,
        Hash:      hash,
    }

    // Registra transferência
    m.mu.Lock()
    m.transfers[transfer.ID] = transfer
    m.mu.Unlock()

    // Adiciona à fila
    m.queue.mu.Lock()
    m.queue.items.PushBack(transfer)
    m.queue.mu.Unlock()

    m.logger.Printf("Nova transferência iniciada: %s para %s (%d bytes)", 
        transfer.Filename, transfer.Receiver, transfer.Size)
    
    // Notifica UI
    m.updateUI(transfer)

    return transfer, nil
}

// ReceiveFile inicia o recebimento de uma transferência DCC
func (m *Manager) ReceiveFile(sender string, filename string, size int64, port int) (*Transfer, error) {
    // Cria transferência
    transfer := &Transfer{
        ID:        uuid.New().String(),
        Filename:  filename,
        Size:      size,
        Sender:    sender,
        Receiver:  "me",
        Port:      port,
        Status:    StatusPending,
        StartTime: time.Now(),
        LastActive: time.Now(),
        pauseChan: make(chan struct{}),
        resumeChan: make(chan struct{}),
        cancelChan: make(chan struct{}),
        logger:    m.logger,
        bufferSize: m.options.BufferSize,
    }

    // Registra transferência
    m.mu.Lock()
    m.transfers[transfer.ID] = transfer
    m.mu.Unlock()

    // Adiciona à fila
    m.queue.mu.Lock()
    m.queue.items.PushBack(transfer)
    m.queue.mu.Unlock()

    return transfer, nil
}

// GetTransfer retorna uma transferência pelo ID
func (m *Manager) GetTransfer(id string) (*Transfer, bool) {
    m.mu.RLock()
    defer m.mu.RUnlock()
    t, ok := m.transfers[id]
    return t, ok
}

// ListTransfers retorna todas as transferências
func (m *Manager) ListTransfers() []*Transfer {
    m.mu.RLock()
    defer m.mu.RUnlock()
    transfers := make([]*Transfer, 0, len(m.transfers))
    for _, t := range m.transfers {
        transfers = append(transfers, t)
    }
    return transfers
}

// RemoveCompletedTransfers remove transferências completadas ou canceladas
func (m *Manager) RemoveCompletedTransfers() int {
    m.mu.Lock()
    defer m.mu.Unlock()
    count := 0
    for id, t := range m.transfers {
        if t.Status == StatusCompleted || t.Status == StatusCanceled {
            delete(m.transfers, id)
            count++
        }
    }
    return count
}

// serveFile serve um arquivo via DCC
func (m *Manager) serveFile(t *Transfer, filename string) {
    // Abre o arquivo
    file, err := os.Open(filename)
    if err != nil {
        t.Error = fmt.Errorf("erro ao abrir arquivo: %w", err)
        t.Status = StatusFailed
        return
    }
    defer file.Close()

    // Cria listener
    listener, err := net.Listen("tcp", fmt.Sprintf(":%d", t.Port))
    if err != nil {
        t.Error = fmt.Errorf("erro ao criar listener: %w", err)
        t.Status = StatusFailed
        return
    }
    defer listener.Close()

    // Aceita conexão
    conn, err := listener.Accept()
    if err != nil {
        t.Error = fmt.Errorf("erro ao aceitar conexão: %w", err)
        t.Status = StatusFailed
        return
    }
    defer conn.Close()

    // Atualiza status
    t.mu.Lock()
    t.Status = StatusInProgress
    t.mu.Unlock()
    t.LastActive = time.Now()

    // Cria buffer e barra de progresso
    buf := make([]byte, t.bufferSize)
    bar := progressbar.DefaultBytes(t.Size, "Enviando")

    // Variáveis para controle de pause/resume
    paused := false
    pauseChan := t.pauseChan
    resumeChan := t.resumeChan
    cancelChan := t.cancelChan

    // Loop de transferência
    for {
        if paused {
            // Se estiver pausado, aguarda resume ou cancel
            select {
            case <-resumeChan:
                paused = false
                t.mu.Lock()
                t.Status = StatusInProgress
                t.mu.Unlock()
                resumeChan = t.resumeChan
            case <-cancelChan:
                t.mu.Lock()
                t.Status = StatusCanceled
                t.mu.Unlock()
                return
            default:
                time.Sleep(100 * time.Millisecond)
                continue
            }
        } else {
            // Se não estiver pausado, verifica pause/cancel ou continua transferência
            select {
            case <-pauseChan:
                paused = true
                t.mu.Lock()
                t.Status = StatusPaused
                t.mu.Unlock()
                pauseChan = t.pauseChan
            case <-cancelChan:
                t.mu.Lock()
                t.Status = StatusCanceled
                t.mu.Unlock()
                return
            default:
                // Lê do arquivo
                n, err := file.Read(buf)
                if err != nil {
                    if err == io.EOF {
                        if t.BytesTransferred >= t.Size {
                            t.mu.Lock()
                            t.Status = StatusCompleted
                            t.mu.Unlock()
                        } else {
                            t.Error = fmt.Errorf("arquivo menor que o esperado: %d/%d bytes", t.BytesTransferred, t.Size)
                            t.mu.Lock()
                            t.Status = StatusFailed
                            t.mu.Unlock()
                        }
                        return
                    }
                    t.Error = fmt.Errorf("erro ao ler arquivo: %w", err)
                    t.mu.Lock()
                    t.Status = StatusFailed
                    t.mu.Unlock()
                    return
                }

                // Envia dados
                _, err = conn.Write(buf[:n])
                if err != nil {
                    t.Error = fmt.Errorf("erro ao enviar dados: %w", err)
                    t.mu.Lock()
                    t.Status = StatusFailed
                    t.mu.Unlock()
                    return
                }

                // Atualiza progresso
                t.BytesTransferred += int64(n)
                t.LastActive = time.Now()
                bar.Add(n)

                // Verifica se completou
                if t.BytesTransferred >= t.Size {
                    t.mu.Lock()
                    t.Status = StatusCompleted
                    t.mu.Unlock()
                    return
                }
            }
        }
    }
}

// downloadFile recebe um arquivo via DCC
func (m *Manager) downloadFile(t *Transfer) {
    // Cria diretório de destino se não existir
    if err := os.MkdirAll(m.downloadDir, 0755); err != nil {
        t.Error = fmt.Errorf("erro ao criar diretório: %w", err)
        t.mu.Lock()
        t.Status = StatusFailed
        t.mu.Unlock()
        m.logger.Printf("Erro ao criar diretório %s: %v", m.downloadDir, err)
        m.updateUI(t)
        return
    }

    // Cria arquivo de destino
    filename := filepath.Join(m.downloadDir, t.Filename)
    file, err := os.Create(filename)
    if err != nil {
        t.Error = fmt.Errorf("erro ao criar arquivo: %w", err)
        t.mu.Lock()
        t.Status = StatusFailed
        t.mu.Unlock()
        m.logger.Printf("Erro ao criar arquivo %s: %v", filename, err)
        m.updateUI(t)
        return
    }
    defer file.Close()

    // Conecta ao servidor
    m.logger.Printf("Conectando a %s:%d para receber %s", "127.0.0.1", t.Port, t.Filename)
    conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", t.Port))
    if err != nil {
        t.Error = fmt.Errorf("erro ao conectar: %w", err)
        t.mu.Lock()
        t.Status = StatusFailed
        t.mu.Unlock()
        m.logger.Printf("Erro ao conectar para receber %s: %v", t.Filename, err)
        m.updateUI(t)
        return
    }
    defer conn.Close()

    // Atualiza status
    t.mu.Lock()
    t.Status = StatusInProgress
    t.mu.Unlock()
    t.LastActive = time.Now()
    m.logger.Printf("Iniciando download de %s (%d bytes)", t.Filename, t.Size)
    m.updateUI(t)

    // Cria buffer e barra de progresso
    buf := make([]byte, t.bufferSize)
    bar := progressbar.DefaultBytes(t.Size, "Recebendo")

    // Variáveis para controle de pause/resume
    paused := false
    pauseChan := t.pauseChan
    resumeChan := t.resumeChan
    cancelChan := t.cancelChan

    // Última atualização da UI
    lastUIUpdate := time.Now()
    
    // Loop de transferência
    for {
        if paused {
            // Se estiver pausado, aguarda resume ou cancel
            select {
            case <-resumeChan:
                paused = false
                t.mu.Lock()
                t.Status = StatusInProgress
                t.mu.Unlock()
                resumeChan = t.resumeChan
                m.logger.Printf("Transferência de %s retomada", t.Filename)
                m.updateUI(t)
            case <-cancelChan:
                t.mu.Lock()
                t.Status = StatusCanceled
                t.mu.Unlock()
                m.logger.Printf("Transferência de %s cancelada", t.Filename)
                m.updateUI(t)
                return
            default:
                time.Sleep(100 * time.Millisecond)
                continue
            }
        } else {
            // Se não estiver pausado, verifica pause/cancel ou continua transferência
            select {
            case <-pauseChan:
                paused = true
                t.mu.Lock()
                t.Status = StatusPaused
                t.mu.Unlock()
                pauseChan = t.pauseChan
                m.logger.Printf("Transferência de %s pausada em %d bytes", t.Filename, t.BytesTransferred)
                m.updateUI(t)
            case <-cancelChan:
                t.mu.Lock()
                t.Status = StatusCanceled
                t.mu.Unlock()
                m.logger.Printf("Transferência de %s cancelada", t.Filename)
                m.updateUI(t)
                return
            default:
                // Recebe dados
                n, err := conn.Read(buf)
                if err != nil {
                    if err == io.EOF {
                        if t.BytesTransferred >= t.Size {
                            t.mu.Lock()
                            t.Status = StatusCompleted
                            t.mu.Unlock()
                            m.logger.Printf("Transferência de %s completada (%d bytes)", t.Filename, t.BytesTransferred)
                            
                            // Verifica hash se disponível
                            if t.Hash != "" && m.options.VerifyHash {
                                m.logger.Printf("Verificando hash de %s", t.Filename)
                                valid, err := verifyFileHash(filename, t.Hash)
                                if err != nil {
                                    m.logger.Printf("Erro ao verificar hash de %s: %v", t.Filename, err)
                                } else if !valid {
                                    t.mu.Lock()
                                    t.Status = StatusFailed
                                    t.Error = fmt.Errorf("hash do arquivo não corresponde ao esperado")
                                    t.mu.Unlock()
                                    m.logger.Printf("Hash inválido para %s", t.Filename)
                                } else {
                                    m.logger.Printf("Hash válido para %s", t.Filename)
                                }
                            }
                            
                            m.updateUI(t)
                        } else {
                            t.Error = fmt.Errorf("conexão fechada antes de completar: %d/%d bytes", t.BytesTransferred, t.Size)
                            t.mu.Lock()
                            t.Status = StatusFailed
                            t.mu.Unlock()
                            m.logger.Printf("Transferência de %s falhou: %v", t.Filename, t.Error)
                            m.updateUI(t)
                        }
                        return
                    }
                    t.Error = fmt.Errorf("erro ao receber dados: %w", err)
                    t.mu.Lock()
                    t.Status = StatusFailed
                    t.mu.Unlock()
                    m.logger.Printf("Erro ao receber dados para %s: %v", t.Filename, err)
                    m.updateUI(t)
                    return
                }

                // Escreve no arquivo
                _, err = file.Write(buf[:n])
                if err != nil {
                    t.Error = fmt.Errorf("erro ao escrever arquivo: %w", err)
                    t.mu.Lock()
                    t.Status = StatusFailed
                    t.mu.Unlock()
                    m.logger.Printf("Erro ao escrever em %s: %v", t.Filename, err)
                    m.updateUI(t)
                    return
                }

                // Atualiza progresso
                t.BytesTransferred += int64(n)
                t.LastActive = time.Now()
                bar.Add(n)
                
                // Atualiza estatísticas
                t.updateTransferStats(t.BytesTransferred)
                
                // Atualiza UI a cada 500ms para não sobrecarregar
                if time.Since(lastUIUpdate) > 500*time.Millisecond {
                    m.updateUI(t)
                    lastUIUpdate = time.Now()
                }

                // Verifica se completou
                if t.BytesTransferred >= t.Size {
                    t.mu.Lock()
                    t.Status = StatusCompleted
                    t.mu.Unlock()
                    m.logger.Printf("Transferência de %s completada (%d bytes)", t.Filename, t.BytesTransferred)
                    
                    // Verifica hash se disponível
                    if t.Hash != "" && m.options.VerifyHash {
                        m.logger.Printf("Verificando hash de %s", t.Filename)
                        valid, err := verifyFileHash(filename, t.Hash)
                        if err != nil {
                            m.logger.Printf("Erro ao verificar hash de %s: %v", t.Filename, err)
                        } else if !valid {
                            t.mu.Lock()
                            t.Status = StatusFailed
                            t.Error = fmt.Errorf("hash do arquivo não corresponde ao esperado")
                            t.mu.Unlock()
                            m.logger.Printf("Hash inválido para %s", t.Filename)
                        } else {
                            m.logger.Printf("Hash válido para %s", t.Filename)
                        }
                    }
                    
                    m.updateUI(t)
                    return
                }
            }
        }
    }
}

// updateTransferStats atualiza as estatísticas da transferência
func (t *Transfer) updateTransferStats(bytesTransferred int64) {
    t.mu.Lock()
    defer t.mu.Unlock()
    
    now := time.Now()
    elapsed := now.Sub(t.StartTime)
    
    // Atualiza velocidade (bytes por segundo)
    if elapsed.Seconds() > 0 {
        t.Speed = float64(bytesTransferred) / elapsed.Seconds()
    }
    
    // Atualiza ETA
    if t.Speed > 0 && bytesTransferred < t.Size {
        remainingBytes := t.Size - bytesTransferred
        remainingTime := time.Duration(float64(remainingBytes) / t.Speed * float64(time.Second))
        t.ETA = remainingTime
    }
    
    // Atualiza progresso
    if t.Size > 0 {
        t.Progress = float64(bytesTransferred) / float64(t.Size) * 100
    }
    
    t.LastActive = now
}

// PauseTransfer pausa uma transferência
func (m *Manager) PauseTransfer(id string) error {
    m.mu.Lock()
    t, ok := m.transfers[id]
    if !ok {
        m.mu.Unlock()
        return fmt.Errorf("transferência não encontrada")
    }

    t.mu.Lock()
    defer t.mu.Unlock()
    m.mu.Unlock()

    if t.Status != StatusInProgress {
        return fmt.Errorf("transferência não está em progresso")
    }

    // Fecha o canal atual e cria um novo para o próximo pause
    close(t.pauseChan)
    t.pauseChan = make(chan struct{})
    
    return nil
}

// ResumeTransfer retoma uma transferência
func (m *Manager) ResumeTransfer(id string) error {
    m.mu.Lock()
    t, ok := m.transfers[id]
    if !ok {
        m.mu.Unlock()
        return fmt.Errorf("transferência não encontrada")
    }

    t.mu.Lock()
    defer t.mu.Unlock()
    m.mu.Unlock()

    if t.Status != StatusPaused {
        return fmt.Errorf("transferência não está pausada")
    }

    // Fecha o canal atual e cria um novo para o próximo resume
    close(t.resumeChan)
    t.resumeChan = make(chan struct{})
    
    return nil
}

// CancelTransfer cancela uma transferência
func (m *Manager) CancelTransfer(id string) error {
    m.mu.Lock()
    t, ok := m.transfers[id]
    if !ok {
        m.mu.Unlock()
        return fmt.Errorf("transferência não encontrada")
    }

    t.mu.Lock()
    defer t.mu.Unlock()
    m.mu.Unlock()

    if t.Status == StatusCompleted || t.Status == StatusCanceled {
        return fmt.Errorf("transferência já finalizada")
    }

    // Fecha o canal de cancelamento
    close(t.cancelChan)
    t.cancelChan = make(chan struct{})
    
    return nil
}

// processQueue processa a fila de transferências
func (m *Manager) processQueue() {
    m.queue.mu.Lock()
    defer m.queue.mu.Unlock()

    // Remove transferências completadas/canceladas
    for id, t := range m.queue.processing {
        if t.Status == StatusCompleted || t.Status == StatusCanceled {
            delete(m.queue.processing, id)
        }
    }

    // Verifica limite de transferências ativas
    if len(m.queue.processing) >= m.queue.maxConcurrent {
        return
    }

    // Processa itens da fila
    for e := m.queue.items.Front(); e != nil; {
        t := e.Value.(*Transfer)
        next := e.Next() // Guarda próximo antes de remover

        // Pula transferências já em processamento
        if _, ok := m.queue.processing[t.ID]; ok {
            e = next
            continue
        }

        // Verifica se transferência está pronta para processamento
        if t.Status == StatusPending || t.Status == StatusFailed {
            // Verifica tempo de retry
            if t.Status == StatusFailed {
                if time.Since(t.LastActive) < RetryInterval {
                    e = next
                    continue
                }
                t.RetryCount++
                if t.RetryCount > MaxRetries {
                    m.queue.items.Remove(e)
                    e = next
                    continue
                }
            }

            // Adiciona à lista de processamento
            m.queue.processing[t.ID] = t

            // Remove da fila
            m.queue.items.Remove(e)

            // Inicia transferência em goroutine
            go func(t *Transfer) {
                if t.Sender == "me" {
                    filename := filepath.Join(m.downloadDir, t.Filename)
                    m.serveFile(t, filename)
                } else {
                    m.downloadFile(t)
                }

                // Atualiza status após completar
                m.queue.mu.Lock()
                if t.Status == StatusCompleted || (t.Status == StatusFailed && t.RetryCount >= MaxRetries) {
                    delete(m.queue.processing, t.ID)
                } else if t.Status == StatusFailed {
                    // Adiciona de volta à fila para retry
                    m.queue.items.PushBack(t)
                    delete(m.queue.processing, t.ID)
                }
                m.queue.mu.Unlock()
            }(t)

            // Verifica limite de transferências ativas
            if len(m.queue.processing) >= m.queue.maxConcurrent {
                break
            }
        }

        e = next
    }
}

// checkActiveTransfers verifica transferências ativas e tenta reconectar as que falharam
func (m *Manager) checkActiveTransfers() {
    m.queue.mu.Lock()
    defer m.queue.mu.Unlock()
    
    now := time.Now()
    for id, t := range m.queue.processing {
        // Remove transferências completadas ou canceladas
        if t.Status == StatusCompleted || t.Status == StatusCanceled {
            delete(m.queue.processing, id)
            continue
        }
        
        // Verifica timeout e falhas
        if t.Status == StatusFailed && now.Sub(t.LastActive) > RetryInterval {
            if now.Sub(t.StartTime) > MaxRetryTimeout || t.RetryCount >= MaxRetries {
                delete(m.queue.processing, id)
                continue
            }
            
            // Obtém nova porta
            port, err := m.portManager.GetAvailablePort()
            if err != nil {
                t.Error = fmt.Errorf("erro ao obter nova porta: %w", err)
                delete(m.queue.processing, id)
                continue
            }

            // Libera porta antiga
            m.portManager.ReleasePort(t.Port)
            t.Port = port
            
            // Tenta reconectar
            t.RetryCount++
            t.Status = StatusPending
            t.pauseChan = make(chan struct{})
            t.resumeChan = make(chan struct{})
            t.cancelChan = make(chan struct{})
            t.LastActive = now
            
            // Adiciona transferência de volta à fila
            m.queue.items.PushBack(t)
            delete(m.queue.processing, id)
        }
    }
}

// calculateFileHash calcula o hash SHA-256 de um arquivo
func calculateFileHash(filepath string) (string, error) {
    file, err := os.Open(filepath)
    if err != nil {
        return "", fmt.Errorf("erro ao abrir arquivo para hash: %w", err)
    }
    defer file.Close()

    hash := sha256.New()
    if _, err := io.Copy(hash, file); err != nil {
        return "", fmt.Errorf("erro ao calcular hash: %w", err)
    }

    return hex.EncodeToString(hash.Sum(nil)), nil
}

// verifyFileHash verifica se o hash de um arquivo corresponde ao esperado
func verifyFileHash(filepath string, expectedHash string) (bool, error) {
    if expectedHash == "" {
        return true, nil // Se não houver hash esperado, considera válido
    }

    actualHash, err := calculateFileHash(filepath)
    if err != nil {
        return false, err
    }

    return actualHash == expectedHash, nil
}

// SendFiles envia múltiplos arquivos
func (m *Manager) SendFiles(filenames []string, receiver string) ([]*Transfer, error) {
    transfers := make([]*Transfer, 0, len(filenames))
    
    for _, filename := range filenames {
        transfer, err := m.SendFile(filename, receiver)
        if err != nil {
            // Cancela transferências anteriores
            for _, t := range transfers {
                if t.Status == StatusPending || t.Status == StatusInProgress {
                    t.Status = StatusCanceled
                }
            }
            return transfers, fmt.Errorf("erro ao enviar %s: %w", filename, err)
        }
        transfers = append(transfers, transfer)
    }

    return transfers, nil
}

// ParseDCCSend analisa um comando DCC SEND
func ParseDCCSend(cmd string) (filename string, size int64, port int, err error) {
    // Formato: DCC SEND filename size port
    var sizeStr, portStr string
    _, err = fmt.Sscanf(cmd, "DCC SEND %s %s %s", &filename, &sizeStr, &portStr)
    if err != nil {
        return "", 0, 0, fmt.Errorf("formato inválido: %w", err)
    }

    size, err = strconv.ParseInt(sizeStr, 10, 64)
    if err != nil {
        return "", 0, 0, fmt.Errorf("tamanho inválido: %w", err)
    }

    port, err = strconv.Atoi(portStr)
    if err != nil {
        return "", 0, 0, fmt.Errorf("porta inválida: %w", err)
    }

    return filename, size, port, nil
}

// FormatDCCSend formata um comando DCC SEND
func FormatDCCSend(filename string, size int64, port int) string {
    return fmt.Sprintf("DCC SEND %s %d %d", filename, size, port)
}
