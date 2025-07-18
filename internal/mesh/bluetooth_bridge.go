package mesh

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/peder1981/p2p-irc/internal/discovery"
)

// BitchatBridge representa a ponte de integração com o Bitchat
type BitchatBridge struct {
	bitchatPath     string
	meshIntegration *MeshIntegration
	ctx             context.Context
	cancel          context.CancelFunc
	mu              sync.RWMutex

	// Estado da conexão
	isConnected   bool
	lastHeartbeat time.Time

	// Canais de comunicação
	messageQueue chan BitchatMessage
	statusQueue  chan BitchatStatus

	// Configurações
	config BitchatConfig
}

// BitchatMessage representa uma mensagem do protocolo Bitchat
type BitchatMessage struct {
	Type      string `json:"type"`
	Sender    string `json:"sender"`
	Recipient string `json:"recipient,omitempty"`
	Channel   string `json:"channel,omitempty"`
	Content   string `json:"content"`
	Timestamp uint64 `json:"timestamp"`
	Signature string `json:"signature,omitempty"`
}

// BitchatStatus representa o status da rede Bitchat
type BitchatStatus struct {
	ActivePeers    int       `json:"active_peers"`
	NetworkHealth  float64   `json:"network_health"`
	LastActivity   time.Time `json:"last_activity"`
	BluetoothState string    `json:"bluetooth_state"`
}

// BitchatConfig contém configurações para integração com Bitchat
type BitchatConfig struct {
	BitchatBinaryPath string
	EnableEncryption  bool
	EnableCompression bool
	MeshNetworkID     string
	LogLevel          string
}

// NewBitchatBridge cria uma nova ponte de integração com Bitchat
func NewBitchatBridge(config BitchatConfig, meshIntegration *MeshIntegration) *BitchatBridge {
	ctx, cancel := context.WithCancel(context.Background())

	return &BitchatBridge{
		bitchatPath:     config.BitchatBinaryPath,
		meshIntegration: meshIntegration,
		ctx:             ctx,
		cancel:          cancel,
		config:          config,
		messageQueue:    make(chan BitchatMessage, 100),
		statusQueue:     make(chan BitchatStatus, 10),
	}
}

// Start inicia a ponte de integração com Bitchat
func (bb *BitchatBridge) Start() error {
	// Verifica se o binário do Bitchat existe
	if err := bb.checkBitchatBinary(); err != nil {
		return fmt.Errorf("erro ao verificar binário do Bitchat: %w", err)
	}

	// Inicia o processo do Bitchat
	go bb.startBitchatProcess()

	// Inicia processamento de mensagens
	go bb.processMessages()

	// Inicia monitoramento de status
	go bb.monitorStatus()

	return nil
}

// Stop para a ponte de integração
func (bb *BitchatBridge) Stop() error {
	bb.cancel()
	return nil
}

// checkBitchatBinary verifica se o binário do Bitchat está disponível
func (bb *BitchatBridge) checkBitchatBinary() error {
	// Primeiro tenta o caminho configurado
	if bb.bitchatPath != "" {
		if _, err := exec.LookPath(bb.bitchatPath); err == nil {
			return nil
		}
	}

	// Tenta encontrar o binário do Bitchat em locais padrão
	possiblePaths := []string{
		"/home/peder/bitchat-go/dist/bitchat-linux-amd64",
		"./bitchat",
		"../bitchat-go/dist/bitchat-linux-amd64",
		"/usr/local/bin/bitchat",
		"/usr/bin/bitchat",
	}

	for _, path := range possiblePaths {
		if absPath, err := filepath.Abs(path); err == nil {
			if _, err := exec.LookPath(absPath); err == nil {
				bb.bitchatPath = absPath
				fmt.Printf("[MESH] Bitchat encontrado em: %s\n", absPath)
				return nil
			}
		}
	}

	return fmt.Errorf("binário do Bitchat não encontrado")
}

// startBitchatProcess inicia o processo do Bitchat em modo daemon
func (bb *BitchatBridge) startBitchatProcess() {
	for {
		select {
		case <-bb.ctx.Done():
			return
		default:
			if err := bb.runBitchatDaemon(); err != nil {
				fmt.Printf("[MESH] Erro ao executar Bitchat daemon: %v\n", err)
				time.Sleep(5 * time.Second) // Retry após 5 segundos
			}
		}
	}
}

// runBitchatDaemon executa o Bitchat em modo daemon
func (bb *BitchatBridge) runBitchatDaemon() error {
	args := []string{
		"--daemon",
		"--mesh-mode",
		"--network-id", bb.config.MeshNetworkID,
		"--log-level", bb.config.LogLevel,
	}

	if bb.config.EnableEncryption {
		args = append(args, "--enable-encryption")
	}

	if bb.config.EnableCompression {
		args = append(args, "--enable-compression")
	}

	cmd := exec.CommandContext(bb.ctx, bb.bitchatPath, args...)

	// Configura pipes para comunicação
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("erro ao criar stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("erro ao criar stderr pipe: %w", err)
	}

	// Inicia o processo
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("erro ao iniciar processo Bitchat: %w", err)
	}

	bb.mu.Lock()
	bb.isConnected = true
	bb.lastHeartbeat = time.Now()
	bb.mu.Unlock()

	// Monitora saídas do processo
	go bb.monitorBitchatOutput(stdout, stderr)

	// Aguarda o processo terminar
	return cmd.Wait()
}

// monitorBitchatOutput monitora as saídas do processo Bitchat
func (bb *BitchatBridge) monitorBitchatOutput(stdout, stderr interface{}) {
	// TODO: Implementar parsing das saídas do Bitchat
	// Por enquanto, apenas simula o monitoramento
	fmt.Println("[MESH] Monitorando saídas do Bitchat...")
}

// processMessages processa mensagens da fila
func (bb *BitchatBridge) processMessages() {
	for {
		select {
		case <-bb.ctx.Done():
			return
		case msg := <-bb.messageQueue:
			bb.handleBitchatMessage(msg)
		}
	}
}

// handleBitchatMessage processa uma mensagem do Bitchat
func (bb *BitchatBridge) handleBitchatMessage(msg BitchatMessage) {
	// Converte mensagem do Bitchat para formato do p2p-irc
	ircMsg := discovery.Message{
		Type:      bb.convertMessageType(msg.Type),
		Sender:    msg.Sender,
		Content:   msg.Content,
		Channel:   msg.Channel,
		Timestamp: time.UnixMilli(int64(msg.Timestamp)),
	}

	// Envia para o sistema de descoberta do p2p-irc
	if bb.meshIntegration != nil && bb.meshIntegration.discovery != nil {
		bb.meshIntegration.discovery.BroadcastMessage(ircMsg)
	}

	fmt.Printf("[MESH] Mensagem processada: %s -> %s\n", msg.Sender, msg.Content)
}

// convertMessageType converte tipos de mensagem do Bitchat para p2p-irc
func (bb *BitchatBridge) convertMessageType(bitchatType string) string {
	switch bitchatType {
	case "chat":
		return discovery.TypeChatMessage
	case "join":
		return discovery.TypeJoinChannel
	case "part":
		return discovery.TypePartChannel
	case "ping":
		return discovery.TypePing
	case "pong":
		return discovery.TypePong
	default:
		return discovery.TypeChatMessage
	}
}

// convertToBitchatType converte tipos de mensagem do p2p-irc para Bitchat
func (bb *BitchatBridge) convertToBitchatType(ircType string) string {
	switch ircType {
	case discovery.TypeChatMessage:
		return "chat"
	case discovery.TypeJoinChannel:
		return "join"
	case discovery.TypePartChannel:
		return "part"
	case discovery.TypePing:
		return "ping"
	case discovery.TypePong:
		return "pong"
	default:
		return "chat"
	}
}

// convertToBitchatMessage converte mensagem P2P-IRC para formato Bitchat
func (bb *BitchatBridge) convertToBitchatMessage(msg discovery.Message) BitchatMessage {
	return BitchatMessage{
		Type:      msg.Type,
		Sender:    msg.Sender,
		Channel:   msg.Channel,
		Content:   msg.Content,
		Timestamp: uint64(msg.Timestamp.UnixMilli()),
	}
}

// convertFromBitchatMessage converte mensagem Bitchat para formato P2P-IRC
func (bb *BitchatBridge) convertFromBitchatMessage(msg BitchatMessage) discovery.Message {
	return discovery.Message{
		Type:      msg.Type,
		Sender:    msg.Sender,
		Channel:   msg.Channel,
		Content:   msg.Content,
		Timestamp: time.UnixMilli(int64(msg.Timestamp)),
	}
}

// monitorStatus monitora o status da rede Bitchat
func (bb *BitchatBridge) monitorStatus() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-bb.ctx.Done():
			return
		case <-ticker.C:
			bb.updateNetworkStatus()
		case status := <-bb.statusQueue:
			bb.processStatusUpdate(status)
		}
	}
}

// updateNetworkStatus atualiza o status da rede
func (bb *BitchatBridge) updateNetworkStatus() {
	bb.mu.Lock()
	defer bb.mu.Unlock()

	// Verifica se a conexão ainda está ativa
	if time.Since(bb.lastHeartbeat) > 30*time.Second {
		bb.isConnected = false
		fmt.Println("[MESH] Conexão com Bitchat perdida")
	}

	// Simula status da rede (TODO: obter dados reais do Bitchat)
	if bb.isConnected {
		status := BitchatStatus{
			ActivePeers:    bb.getActivePeersCount(),
			NetworkHealth:  bb.calculateNetworkHealth(),
			LastActivity:   time.Now(),
			BluetoothState: "connected",
		}

		bb.statusQueue <- status
	}
}

// processStatusUpdate processa atualizações de status
func (bb *BitchatBridge) processStatusUpdate(status BitchatStatus) {
	// Atualiza métricas da integração mesh
	if bb.meshIntegration != nil {
		meshStatus := MeshStatus{
			ActivePeers:    status.ActivePeers,
			BluetoothPeers: status.ActivePeers, // Assume que todos são Bluetooth
			NetworkHealth:  status.NetworkHealth,
			LastActivity:   status.LastActivity,
		}

		// Notifica sobre mudanças de status
		if bb.meshIntegration.onMeshStatus != nil {
			bb.meshIntegration.onMeshStatus(meshStatus)
		}
	}

	fmt.Printf("[MESH] Status atualizado: %d peers ativos, saúde: %.2f\n",
		status.ActivePeers, status.NetworkHealth)
}

// getActivePeersCount retorna o número de peers ativos
func (bb *BitchatBridge) getActivePeersCount() int {
	if bb.meshIntegration != nil {
		return len(bb.meshIntegration.GetPeers())
	}
	return 0
}

// calculateNetworkHealth calcula a saúde da rede
func (bb *BitchatBridge) calculateNetworkHealth() float64 {
	if !bb.isConnected {
		return 0.0
	}

	// Calcula baseado no tempo desde o último heartbeat
	timeSinceHeartbeat := time.Since(bb.lastHeartbeat)
	if timeSinceHeartbeat > 10*time.Second {
		return 0.5
	}

	return 1.0
}

// SendToBitchat envia uma mensagem para a rede Bitchat
func (bb *BitchatBridge) SendToBitchat(msg discovery.Message) error {
	if !bb.isConnected {
		return fmt.Errorf("não conectado ao Bitchat")
	}

	// Converte mensagem do p2p-irc para formato Bitchat
	bitchatMsg := bb.convertToBitchatMessage(msg)

	// Adiciona à fila de mensagens
	select {
	case bb.messageQueue <- bitchatMsg:
		return nil
	default:
		return fmt.Errorf("fila de mensagens cheia")
	}
}

// IsConnected retorna se está conectado ao Bitchat
func (bb *BitchatBridge) IsConnected() bool {
	bb.mu.RLock()
	defer bb.mu.RUnlock()
	return bb.isConnected
}

// GetStatus retorna o status atual da ponte
func (bb *BitchatBridge) GetStatus() map[string]interface{} {
	bb.mu.RLock()
	defer bb.mu.RUnlock()

	return map[string]interface{}{
		"connected":      bb.isConnected,
		"last_heartbeat": bb.lastHeartbeat,
		"bitchat_path":   bb.bitchatPath,
		"network_id":     bb.config.MeshNetworkID,
		"encryption":     bb.config.EnableEncryption,
		"compression":    bb.config.EnableCompression,
	}
}
