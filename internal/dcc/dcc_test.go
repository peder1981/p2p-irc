package dcc

import (
    "os"
    "path/filepath"
    "testing"
    "time"
)

func TestDCCManager(t *testing.T) {
    // Cria diretório temporário para testes
    tmpDir, err := os.MkdirTemp("", "dcc-test-*")
    if err != nil {
        t.Fatalf("erro ao criar diretório temporário: %v", err)
    }
    defer os.RemoveAll(tmpDir)

    // Cria arquivo de teste
    testFile := filepath.Join(tmpDir, "test.txt")
    content := []byte("teste de transferência DCC")
    if err := os.WriteFile(testFile, content, 0644); err != nil {
        t.Fatalf("erro ao criar arquivo de teste: %v", err)
    }

    // Cria gerenciador DCC
    downloadDir := filepath.Join(tmpDir, "downloads")
    manager := NewManager(downloadDir)

    // Testa envio de arquivo
    transfer, err := manager.SendFile(testFile, "receiver")
    if err != nil {
        t.Fatalf("erro ao iniciar envio: %v", err)
    }

    if transfer.Status != StatusPending {
        t.Errorf("status inicial incorreto: %v", transfer.Status)
    }

    if transfer.Size != int64(len(content)) {
        t.Errorf("tamanho incorreto: got %d, want %d", transfer.Size, len(content))
    }

    // Testa recebimento de arquivo
    receiver := NewManager(filepath.Join(tmpDir, "downloads2"))
    recvTransfer, err := receiver.ReceiveFile("sender", "test.txt", transfer.Size, transfer.Port)
    if err != nil {
        t.Fatalf("erro ao iniciar recebimento: %v", err)
    }

    // Espera transferência completar
    time.Sleep(time.Second)

    // Verifica status
    if recvTransfer.Status != StatusCompleted {
        t.Errorf("status final incorreto: %v", recvTransfer.Status)
    }

    // Verifica arquivo recebido
    receivedPath := filepath.Join(receiver.downloadDir, "test.txt")
    received, err := os.ReadFile(receivedPath)
    if err != nil {
        t.Fatalf("erro ao ler arquivo recebido: %v", err)
    }

    if string(received) != string(content) {
        t.Errorf("conteúdo recebido incorreto: got %q, want %q", received, content)
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
