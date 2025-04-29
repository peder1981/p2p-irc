package dcc

import (
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "io"
    "net"
    "os"
    "path/filepath"
    "strconv"
    "sync"
    "time"

    "github.com/peder1981/p2p-irc/internal/portmanager"
    "github.com/schollz/progressbar/v3"
)

// TransferStatus representa o estado atual de uma transferência
type TransferStatus int

const (
    StatusPending TransferStatus = iota
    StatusInProgress
    StatusCompleted
    StatusFailed
    StatusCanceled
)

// Transfer representa uma transferência DCC
type Transfer struct {
    ID          string
    Filename    string
    Size        int64
    Sender      string
    Receiver    string
    Port        int
    Status      TransferStatus
    Progress    float64
    StartTime   time.Time
    EndTime     time.Time
    Error       error
    Hash        string    // Hash SHA-256 do arquivo
    Offset      int64     // Para retomar transferências
    bar         *progressbar.ProgressBar
    resumable   bool
}

// Manager gerencia transferências DCC
type Manager struct {
    transfers     map[string]*Transfer
    portManager   *portmanager.PortManager
    downloadDir   string
    mu           sync.RWMutex
}

// NewManager cria um novo gerenciador DCC
func NewManager(downloadDir string) *Manager {
    if downloadDir == "" {
        downloadDir = "downloads"
    }
    
    // Cria diretório de downloads se não existir
    os.MkdirAll(downloadDir, 0755)
    
    return &Manager{
        transfers:   make(map[string]*Transfer),
        portManager: portmanager.New(),
        downloadDir: downloadDir,
    }
}

// SendFile inicia uma transferência DCC SEND
func (m *Manager) SendFile(filename string, receiver string) (*Transfer, error) {
    // Verifica se arquivo existe
    file, err := os.Open(filename)
    if err != nil {
        return nil, fmt.Errorf("erro ao abrir arquivo: %w", err)
    }
    defer file.Close()

    // Obtém tamanho do arquivo
    info, err := file.Stat()
    if err != nil {
        return nil, fmt.Errorf("erro ao obter info do arquivo: %w", err)
    }

    // Obtém porta disponível
    port, err := m.portManager.GetAvailablePort()
    if err != nil {
        return nil, fmt.Errorf("erro ao obter porta: %w", err)
    }

    // Cria transferência
    transfer := &Transfer{
        ID:       fmt.Sprintf("%d", time.Now().UnixNano()),
        Filename: filepath.Base(filename),
        Size:     info.Size(),
        Sender:   "me",
        Receiver: receiver,
        Port:     port,
        Status:   StatusPending,
    }

    // Inicia servidor
    go m.serveFile(transfer, filename)

    // Registra transferência
    m.mu.Lock()
    m.transfers[transfer.ID] = transfer
    m.mu.Unlock()

    return transfer, nil
}

// ReceiveFile aceita uma transferência DCC SEND
func (m *Manager) ReceiveFile(sender, filename string, size int64, port int) (*Transfer, error) {
    // Cria transferência
    transfer := &Transfer{
        ID:       fmt.Sprintf("%d", time.Now().UnixNano()),
        Filename: filename,
        Size:     size,
        Sender:   sender,
        Receiver: "me",
        Port:     port,
        Status:   StatusPending,
    }

    // Inicia download
    go m.downloadFile(transfer)

    // Registra transferência
    m.mu.Lock()
    m.transfers[transfer.ID] = transfer
    m.mu.Unlock()

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

// serveFile serve um arquivo para download
func (m *Manager) serveFile(t *Transfer, filename string) {
    defer m.portManager.ReleasePort(t.Port)

    // Calcula hash do arquivo
    hash, err := calculateFileHash(filename)
    if err != nil {
        t.Status = StatusFailed
        t.Error = fmt.Errorf("erro ao calcular hash: %w", err)
        return
    }
    t.Hash = hash

    // Inicia servidor TCP
    listener, err := net.Listen("tcp", fmt.Sprintf(":%d", t.Port))
    if err != nil {
        t.Status = StatusFailed
        t.Error = fmt.Errorf("erro ao iniciar servidor: %w", err)
        return
    }
    defer listener.Close()

    // Aceita uma conexão
    conn, err := listener.Accept()
    if err != nil {
        t.Status = StatusFailed
        t.Error = fmt.Errorf("erro ao aceitar conexão: %w", err)
        return
    }
    defer conn.Close()

    // Abre arquivo
    file, err := os.Open(filename)
    if err != nil {
        t.Status = StatusFailed
        t.Error = fmt.Errorf("erro ao abrir arquivo: %w", err)
        return
    }
    defer file.Close()

    // Verifica se cliente quer retomar
    resumeHeader := make([]byte, 8)
    _, err = conn.Read(resumeHeader)
    if err == nil {
        offset, err := strconv.ParseInt(string(resumeHeader), 10, 64)
        if err == nil && offset > 0 && offset < t.Size {
            file.Seek(offset, 0)
            t.Offset = offset
            t.resumable = true
        }
    }

    // Inicia transferência
    t.Status = StatusInProgress
    t.StartTime = time.Now()

    // Cria barra de progresso
    t.bar = progressbar.NewOptions64(
        t.Size,
        progressbar.OptionSetDescription(fmt.Sprintf("Enviando %s", t.Filename)),
        progressbar.OptionSetWriter(os.Stderr),
        progressbar.OptionShowBytes(true),
        progressbar.OptionSetWidth(15),
        progressbar.OptionThrottle(65*time.Millisecond),
        progressbar.OptionShowCount(),
        progressbar.OptionOnCompletion(func() {
            fmt.Printf("\n")
        }),
    )
    t.bar.Set64(t.Offset)

    // Envia arquivo
    written, err := io.Copy(io.MultiWriter(conn, t.bar), file)
    if err != nil {
        t.Status = StatusFailed
        t.Error = fmt.Errorf("erro ao enviar arquivo: %w", err)
        return
    }

    // Finaliza transferência
    t.EndTime = time.Now()
    t.Progress = 100
    if written+t.Offset == t.Size {
        t.Status = StatusCompleted
    } else {
        t.Status = StatusFailed
        t.Error = fmt.Errorf("tamanho enviado (%d) diferente do esperado (%d)", written+t.Offset, t.Size)
    }
}

// downloadFile baixa um arquivo de uma transferência DCC
func (m *Manager) downloadFile(t *Transfer) {
    // Conecta ao servidor
    conn, err := net.Dial("tcp", fmt.Sprintf(":%d", t.Port))
    if err != nil {
        t.Status = StatusFailed
        t.Error = fmt.Errorf("erro ao conectar: %w", err)
        return
    }
    defer conn.Close()

    // Verifica se arquivo existe para retomar
    filepath := filepath.Join(m.downloadDir, t.Filename)
    var file *os.File
    if info, err := os.Stat(filepath); err == nil && info.Size() < t.Size {
        // Arquivo existe e está incompleto
        file, err = os.OpenFile(filepath, os.O_APPEND|os.O_WRONLY, 0644)
        if err == nil {
            t.Offset = info.Size()
            t.resumable = true
            // Envia offset para servidor
            fmt.Fprintf(conn, "%08d", t.Offset)
        }
    }

    if file == nil {
        // Cria novo arquivo
        file, err = os.Create(filepath)
        if err != nil {
            t.Status = StatusFailed
            t.Error = fmt.Errorf("erro ao criar arquivo: %w", err)
            return
        }
    }
    defer file.Close()

    // Inicia transferência
    t.Status = StatusInProgress
    t.StartTime = time.Now()

    // Cria barra de progresso
    t.bar = progressbar.NewOptions64(
        t.Size,
        progressbar.OptionSetDescription(fmt.Sprintf("Baixando %s", t.Filename)),
        progressbar.OptionSetWriter(os.Stderr),
        progressbar.OptionShowBytes(true),
        progressbar.OptionSetWidth(15),
        progressbar.OptionThrottle(65*time.Millisecond),
        progressbar.OptionShowCount(),
        progressbar.OptionOnCompletion(func() {
            fmt.Printf("\n")
        }),
    )
    t.bar.Set64(t.Offset)

    // Recebe arquivo
    written, err := io.Copy(io.MultiWriter(file, t.bar), conn)
    if err != nil {
        t.Status = StatusFailed
        t.Error = fmt.Errorf("erro ao receber arquivo: %w", err)
        return
    }

    // Verifica hash
    if t.Hash != "" {
        hash, err := calculateFileHash(filepath)
        if err != nil {
            t.Status = StatusFailed
            t.Error = fmt.Errorf("erro ao calcular hash: %w", err)
            return
        }
        if hash != t.Hash {
            t.Status = StatusFailed
            t.Error = fmt.Errorf("hash não confere")
            return
        }
    }

    // Finaliza transferência
    t.EndTime = time.Now()
    t.Progress = 100
    if written+t.Offset == t.Size {
        t.Status = StatusCompleted
    } else {
        t.Status = StatusFailed
        t.Error = fmt.Errorf("tamanho recebido (%d) diferente do esperado (%d)", written+t.Offset, t.Size)
    }
}

// calculateFileHash calcula o hash SHA-256 de um arquivo
func calculateFileHash(filename string) (string, error) {
    file, err := os.Open(filename)
    if err != nil {
        return "", err
    }
    defer file.Close()

    hash := sha256.New()
    if _, err := io.Copy(hash, file); err != nil {
        return "", err
    }

    return hex.EncodeToString(hash.Sum(nil)), nil
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
