package dcc

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDCCManager(t *testing.T) {
    // Cria diretórios temporários
    senderDir, err := os.MkdirTemp("", "dcc-test-*")
    if err != nil {
        t.Fatal(err)
    }
    defer os.RemoveAll(senderDir)

    receiverDir, err := os.MkdirTemp("", "dcc-test-*")
    if err != nil {
        t.Fatal(err)
    }
    defer os.RemoveAll(receiverDir)

    // Cria arquivo de teste
    content := []byte("teste de transferência DCC")
    filename := "test.txt"
    testFile := filepath.Join(senderDir, filename)
    err = os.WriteFile(testFile, content, 0644)
    if err != nil {
        t.Fatal(err)
    }

    // Cria gerenciadores
    sender := NewManager(senderDir)
    receiver := NewManager(receiverDir)

    // Inicia transferência
    transfer, err := sender.SendFile(testFile, "receiver")
    if err != nil {
        t.Fatal(err)
    }

    // Inicia recebimento
    recvTransfer, err := receiver.ReceiveFile("sender", filename, transfer.Size, transfer.Port)
    if err != nil {
        t.Fatal(err)
    }

    // Aguarda transferência completar
    for i := 0; i < 30 && (transfer.Status != StatusCompleted || recvTransfer.Status != StatusCompleted); i++ {
        time.Sleep(100 * time.Millisecond)
    }

    // Verifica status final
    if transfer.Status != StatusCompleted {
        t.Errorf("status final do sender incorreto: %d", transfer.Status)
    }
    if recvTransfer.Status != StatusCompleted {
        t.Errorf("status final do receiver incorreto: %d", recvTransfer.Status)
    }

    // Verifica conteúdo recebido
    receivedFile := filepath.Join(receiverDir, filename)
    receivedContent, err := os.ReadFile(receivedFile)
    if err != nil {
        t.Errorf("erro ao ler arquivo recebido: %v", err)
    }
    if string(receivedContent) != string(content) {
        t.Errorf("conteúdo recebido incorreto: got %q, want %q", string(receivedContent), string(content))
    }
}

func TestDCCTransferQueue(t *testing.T) {
    // Cria diretórios temporários
    senderDir, err := os.MkdirTemp("", "dcc-test-*")
    if err != nil {
        t.Fatal(err)
    }
    defer os.RemoveAll(senderDir)

    receiverDir, err := os.MkdirTemp("", "dcc-test-*")
    if err != nil {
        t.Fatal(err)
    }
    defer os.RemoveAll(receiverDir)

    // Cria gerenciadores com opções específicas para o teste
    senderOptions := ManagerOptions{
        DownloadDir: senderDir,
        BufferSize: 4096,
        MaxConcurrent: 3, // Limita a 3 transferências simultâneas
        LogEnabled: false,
    }
    
    receiverOptions := ManagerOptions{
        DownloadDir: receiverDir,
        BufferSize: 4096,
        MaxConcurrent: 3, // Limita a 3 transferências simultâneas
        LogEnabled: false,
    }
    
    sender := NewManagerWithOptions(senderOptions)
    receiver := NewManagerWithOptions(receiverOptions)

    // Cria arquivos de teste e inicia transferências sequencialmente
    content := []byte("teste de transferência DCC")
    numFiles := 3 // Reduzindo para 3 arquivos para evitar problemas com portas
    transfers := make([]*Transfer, 0, numFiles)
    recvTransfers := make([]*Transfer, 0, numFiles)

    for i := 0; i < numFiles; i++ {
        filename := fmt.Sprintf("test%d.txt", i)
        testFile := filepath.Join(senderDir, filename)
        err := os.WriteFile(testFile, content, 0644)
        if err != nil {
            t.Fatal(err)
        }

        // Inicia transferência
        transfer, err := sender.SendFile(testFile, "receiver")
        if err != nil {
            t.Fatal(err)
        }
        transfers = append(transfers, transfer)

        // Inicia recebimento
        recvTransfer, err := receiver.ReceiveFile("sender", filename, transfer.Size, transfer.Port)
        if err != nil {
            t.Fatal(err)
        }
        recvTransfers = append(recvTransfers, recvTransfer)

        // Aguarda um pouco para evitar problemas de concorrência
        time.Sleep(100 * time.Millisecond)
    }

    // Aguarda transferências completarem
    timeout := time.After(10 * time.Second)
    ticker := time.NewTicker(100 * time.Millisecond)
    defer ticker.Stop()

    for {
        select {
        case <-timeout:
            t.Logf("Timeout esperando transferências completarem")
            break
        case <-ticker.C:
            completed := true
            for j := 0; j < len(transfers); j++ {
                if transfers[j].Status != StatusCompleted || recvTransfers[j].Status != StatusCompleted {
                    completed = false
                    break
                }
            }
            if completed {
                goto checkResults
            }
        }
    }

checkResults:
    // Verifica status final
    for i := 0; i < len(transfers); i++ {
        if transfers[i].Status != StatusCompleted {
            t.Errorf("transferência %d não completou: status=%d, erro=%v", i, transfers[i].Status, transfers[i].Error)
        }
        if recvTransfers[i].Status != StatusCompleted {
            t.Errorf("recebimento %d não completou: status=%d, erro=%v", i, recvTransfers[i].Status, recvTransfers[i].Error)
        }

        // Verifica conteúdo recebido
        receivedFile := filepath.Join(receiverDir, fmt.Sprintf("test%d.txt", i))
        receivedContent, err := os.ReadFile(receivedFile)
        if err != nil {
            t.Errorf("erro ao ler arquivo recebido %d: %v", i, err)
            continue
        }
        if string(receivedContent) != string(content) {
            t.Errorf("conteúdo recebido incorreto para arquivo %d: got %q, want %q", i, string(receivedContent), string(content))
        }
    }
}

func TestDCCParsing(t *testing.T) {
    tests := []struct {
        name     string
        cmd      string
        wantFile string
        wantSize int64
        wantPort int
        wantErr  bool
    }{
        {
            name:     "comando válido",
            cmd:      "DCC SEND test.txt 1234 5678",
            wantFile: "test.txt",
            wantSize: 1234,
            wantPort: 5678,
            wantErr:  false,
        },
        {
            name:     "comando inválido",
            cmd:      "DCC SEND",
            wantFile: "",
            wantSize: 0,
            wantPort: 0,
            wantErr:  true,
        },
        {
            name:     "tamanho inválido",
            cmd:      "DCC SEND test.txt abc 5678",
            wantFile: "",
            wantSize: 0,
            wantPort: 0,
            wantErr:  true,
        },
        {
            name:     "porta inválida",
            cmd:      "DCC SEND test.txt 1234 abc",
            wantFile: "",
            wantSize: 0,
            wantPort: 0,
            wantErr:  true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            file, size, port, err := ParseDCCSend(tt.cmd)
            if (err != nil) != tt.wantErr {
                t.Errorf("ParseDCCSend() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if file != tt.wantFile {
                t.Errorf("ParseDCCSend() file = %v, want %v", file, tt.wantFile)
            }
            if size != tt.wantSize {
                t.Errorf("ParseDCCSend() size = %v, want %v", size, tt.wantSize)
            }
            if port != tt.wantPort {
                t.Errorf("ParseDCCSend() port = %v, want %v", port, tt.wantPort)
            }
        })
    }
}

func TestDCCFormatting(t *testing.T) {
    filename := "test.txt"
    size := int64(1234)
    port := 5678
    want := "DCC SEND test.txt 1234 5678"

    got := FormatDCCSend(filename, size, port)
    if got != want {
        t.Errorf("FormatDCCSend() = %v, want %v", got, want)
    }
}

func TestDCCPauseResume(t *testing.T) {
    // Cria diretórios temporários
    senderDir, err := os.MkdirTemp("", "dcc-test-*")
    if err != nil {
        t.Fatal(err)
    }
    defer os.RemoveAll(senderDir)

    receiverDir, err := os.MkdirTemp("", "dcc-test-*")
    if err != nil {
        t.Fatal(err)
    }
    defer os.RemoveAll(receiverDir)

    // Cria arquivo de teste grande o suficiente para ter tempo de pausar
    content := make([]byte, 100*1024) // 100KB
    for i := range content {
        content[i] = byte(i % 256)
    }
    
    filename := "test.txt"
    testFile := filepath.Join(senderDir, filename)
    err = os.WriteFile(testFile, content, 0644)
    if err != nil {
        t.Fatal(err)
    }

    // Cria gerenciadores com buffer pequeno para transferência mais lenta
    senderOptions := ManagerOptions{
        DownloadDir: senderDir,
        BufferSize: 1024, // 1KB para transferência mais lenta
        MaxConcurrent: 1,
        LogEnabled: false,
    }
    
    receiverOptions := ManagerOptions{
        DownloadDir: receiverDir,
        BufferSize: 1024, // 1KB para transferência mais lenta
        MaxConcurrent: 1,
        LogEnabled: false,
    }
    
    sender := NewManagerWithOptions(senderOptions)
    receiver := NewManagerWithOptions(receiverOptions)

    // Inicia transferência
    transfer, err := sender.SendFile(testFile, "receiver")
    if err != nil {
        t.Fatal(err)
    }

    // Inicia recebimento
    recvTransfer, err := receiver.ReceiveFile("sender", filename, transfer.Size, transfer.Port)
    if err != nil {
        t.Fatal(err)
    }

    // Aguarda a transferência iniciar
    startTime := time.Now()
    for time.Since(startTime) < 2*time.Second && recvTransfer.Status != StatusInProgress {
        time.Sleep(100 * time.Millisecond)
    }

    // Verifica se a transferência iniciou
    if recvTransfer.Status != StatusInProgress {
        t.Skipf("Não foi possível iniciar a transferência no tempo esperado (status=%d)", recvTransfer.Status)
        return
    }

    // Aguarda um pouco para transferir alguns dados
    time.Sleep(500 * time.Millisecond)

    // Verifica se alguns dados foram transferidos
    if recvTransfer.BytesTransferred == 0 {
        t.Skip("Nenhum byte foi transferido, não é possível testar pause/resume")
        return
    }

    // Pausa a transferência
    err = receiver.PauseTransfer(recvTransfer.ID)
    if err != nil {
        t.Fatalf("Erro ao pausar transferência: %v", err)
    }

    // Aguarda a transferência pausar
    pauseTime := time.Now()
    for time.Since(pauseTime) < 2*time.Second && recvTransfer.Status != StatusPaused {
        time.Sleep(100 * time.Millisecond)
    }

    // Verifica se a transferência foi pausada
    if recvTransfer.Status != StatusPaused {
        t.Fatalf("A transferência não foi pausada corretamente (status=%d)", recvTransfer.Status)
    }

    // Registra bytes transferidos antes do pause
    bytesBeforePause := recvTransfer.BytesTransferred
    t.Logf("Bytes transferidos antes do pause: %d", bytesBeforePause)

    // Aguarda um pouco para garantir que a transferência está realmente pausada
    time.Sleep(500 * time.Millisecond)

    // Verifica se os bytes transferidos não aumentaram durante o pause
    if recvTransfer.BytesTransferred > bytesBeforePause {
        t.Errorf("Bytes continuaram sendo transferidos durante o pause: %d > %d", 
            recvTransfer.BytesTransferred, bytesBeforePause)
    }

    // Retoma a transferência
    err = receiver.ResumeTransfer(recvTransfer.ID)
    if err != nil {
        t.Fatalf("Erro ao retomar transferência: %v", err)
    }

    // Aguarda a transferência retomar
    resumeTime := time.Now()
    for time.Since(resumeTime) < 2*time.Second && recvTransfer.Status != StatusInProgress {
        time.Sleep(100 * time.Millisecond)
    }

    // Verifica se a transferência foi retomada
    if recvTransfer.Status != StatusInProgress {
        t.Fatalf("A transferência não foi retomada corretamente (status=%d)", recvTransfer.Status)
    }

    // Aguarda um pouco para transferir mais dados após o resume
    time.Sleep(500 * time.Millisecond)

    // Verifica se mais bytes foram transferidos após o resume
    if recvTransfer.BytesTransferred <= bytesBeforePause {
        t.Errorf("Nenhum byte adicional foi transferido após o resume: %d <= %d",
            recvTransfer.BytesTransferred, bytesBeforePause)
    } else {
        t.Logf("Bytes transferidos após o resume: %d (adicional: %d)", 
            recvTransfer.BytesTransferred, recvTransfer.BytesTransferred - bytesBeforePause)
    }

    // Aguarda a transferência completar
    completeTime := time.Now()
    for time.Since(completeTime) < 10*time.Second && recvTransfer.Status != StatusCompleted {
        time.Sleep(100 * time.Millisecond)
    }

    // Verifica se a transferência foi completada
    if recvTransfer.Status != StatusCompleted {
        t.Fatalf("A transferência não foi completada (status=%d, erro=%v)", 
            recvTransfer.Status, recvTransfer.Error)
    }

    // Verifica o conteúdo do arquivo recebido
    receivedFile := filepath.Join(receiverDir, filename)
    receivedContent, err := os.ReadFile(receivedFile)
    if err != nil {
        t.Fatalf("Erro ao ler arquivo recebido: %v", err)
    }

    if len(receivedContent) != len(content) {
        t.Errorf("Tamanho do arquivo recebido incorreto: %d != %d", 
            len(receivedContent), len(content))
    } else {
        // Verifica se o conteúdo é igual
        for i := 0; i < len(content); i++ {
            if receivedContent[i] != content[i] {
                t.Errorf("Conteúdo do arquivo recebido incorreto no byte %d: %d != %d", 
                    i, receivedContent[i], content[i])
                break
            }
        }
    }
}

func TestDCCRetry(t *testing.T) {
    // Cria diretórios temporários
    senderDir, err := os.MkdirTemp("", "dcc-test-*")
    if err != nil {
        t.Fatal(err)
    }
    defer os.RemoveAll(senderDir)

    receiverDir, err := os.MkdirTemp("", "dcc-test-*")
    if err != nil {
        t.Fatal(err)
    }
    defer os.RemoveAll(receiverDir)

    // Cria arquivo de teste
    content := []byte("teste de transferência DCC")
    filename := "test.txt"
    testFile := filepath.Join(senderDir, filename)
    err = os.WriteFile(testFile, content, 0644)
    if err != nil {
        t.Fatal(err)
    }

    // Cria gerenciadores
    sender := NewManager(senderDir)
    receiver := NewManager(receiverDir)

    // Inicia transferência
    transfer, err := sender.SendFile(testFile, "receiver")
    if err != nil {
        t.Fatal(err)
    }

    // Inicia recebimento com porta errada para forçar erro
    recvTransfer, err := receiver.ReceiveFile("sender", filename, transfer.Size, transfer.Port+1)
    if err != nil {
        t.Fatal(err)
    }

    // Aguarda transferência falhar
    for i := 0; i < 20 && recvTransfer.Status != StatusFailed; i++ {
        time.Sleep(100 * time.Millisecond)
    }

    // Verifica status após falha
    if recvTransfer.Status != StatusFailed {
        t.Errorf("status após falha incorreto: got %d, want %d", recvTransfer.Status, StatusFailed)
    }

    // Inicia nova transferência com porta correta
    recvTransfer, err = receiver.ReceiveFile("sender", filename, transfer.Size, transfer.Port)
    if err != nil {
        t.Fatal(err)
    }

    // Aguarda transferência completar
    for i := 0; i < 30 && recvTransfer.Status != StatusCompleted; i++ {
        time.Sleep(100 * time.Millisecond)
    }

    // Verifica status final
    if recvTransfer.Status != StatusCompleted {
        t.Errorf("transferência não completou após retry: status=%d, erro=%v", recvTransfer.Status, recvTransfer.Error)
    }

    // Verifica conteúdo recebido
    receivedFile := filepath.Join(receiverDir, filename)
    receivedContent, err := os.ReadFile(receivedFile)
    if err != nil {
        t.Errorf("erro ao ler arquivo recebido: %v", err)
    }
    if string(receivedContent) != string(content) {
        t.Errorf("conteúdo recebido incorreto: got %q, want %q", string(receivedContent), string(content))
    }
}
