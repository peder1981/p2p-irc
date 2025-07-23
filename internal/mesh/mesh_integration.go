package mesh

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/peder1981/p2p-irc/internal/discovery"
)

// MessageHandler é uma função de callback para processar mensagens recebidas da mesh.
type MessageHandler func(senderID, channel, sender, content string)

// MeshIntegration gerencia a integração com a rede mesh.
type MeshIntegration struct {
	discoveryService *discovery.Discovery
	config           MeshConfig
	mu               sync.RWMutex
	isRunning        bool
	peers            map[string]*MeshPeer
	listener         net.PacketConn
	quit             chan struct{}
	selfID           []byte
	messageHandler   MessageHandler
	logger           func(string)
}

// MeshConfig contém as configurações para a integração com a rede mesh.
type MeshConfig struct {
	Port              int
	EnableEncryption  bool
	EnableCompression bool
	MeshNetworkID     string
	LogLevel          string
	Enabled           bool
}

// MeshPeer representa um peer na rede mesh.
type MeshPeer struct {
	ID       string
	Address  string
	LastSeen time.Time
}

// NewMeshIntegration cria uma nova instância de integração com a mesh.
func NewMeshIntegration(dsc *discovery.Discovery, cfg MeshConfig, handler MessageHandler) *MeshIntegration {
	selfID := make([]byte, 8)
	_, err := rand.Read(selfID)
	if err != nil {
		log.Fatalf("Falha ao gerar ID do peer da mesh: %v", err)
	}

	return &MeshIntegration{
		discoveryService: dsc,
		config:           cfg,
		peers:            make(map[string]*MeshPeer),
		quit:             make(chan struct{}),
		selfID:           selfID,
		messageHandler:   handler,
	}
}

// Start inicia a integração com a rede mesh.
func (m *MeshIntegration) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.isRunning {
		return nil
	}
	log.Println("Iniciando a integração com a rede mesh interna...")

	addr := fmt.Sprintf(":%d", m.config.Port)
	pc, err := net.ListenPacket("udp", addr)
	if err != nil {
		return fmt.Errorf("falha ao iniciar o listener da mesh em %s: %w", addr, err)
	}
	m.listener = pc
	log.Printf("Listener da rede mesh escutando em %s", m.listener.LocalAddr().String())

	// Atualiza a porta na configuração se uma porta aleatória foi usada (porta 0)
	if udpAddr, ok := m.listener.LocalAddr().(*net.UDPAddr); ok {
		m.config.Port = udpAddr.Port
	}

	go m.handleIncomingPackets()

	m.isRunning = true

	// Anuncia a presença na rede
	if err := m.announce(); err != nil {
		m.listener.Close() // Limpa o listener antes de sair
		return fmt.Errorf("falha ao anunciar presença na rede mesh: %w", err)
	}

	log.Println("Integração com a rede mesh interna iniciada com sucesso.")
	return nil
}

// Stop para a integração com a rede mesh.
func (m *MeshIntegration) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.isRunning {
		return
	}
	log.Println("Parando a integração com a rede mesh interna...")

	close(m.quit)
	if m.listener != nil {
		m.listener.Close()
	}

	m.isRunning = false
	m.peers = make(map[string]*MeshPeer)
	log.Println("Integração com a rede mesh interna parada.")
}

// IsRunning verifica se a integração com a mesh está ativa.
func (m *MeshIntegration) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.isRunning
}

// SetLogger define a função de callback para registrar mensagens na UI.
func (m *MeshIntegration) SetLogger(logger func(string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logger = logger
}

// GetPeers retorna a lista de peers conhecidos na rede mesh.
// TODO: Implementar a lógica para obter peers da rede mesh.
func (m *MeshIntegration) GetPeers() []*MeshPeer {

	m.mu.Lock()

	defer func() {
		m.mu.Unlock()

	}()

	peers := make([]*MeshPeer, 0, len(m.peers))
	for _, peer := range m.peers {
		peers = append(peers, peer)
	}
	return peers
}

// handleIncomingPackets é executado em uma goroutine para ler do listener da mesh.
func (m *MeshIntegration) handleIncomingPackets() {
	buffer := make([]byte, 4096) // Buffer de 4KB
	for {
		select {
		case <-m.quit:
			return
		default:
			n, addr, err := m.listener.ReadFrom(buffer)
			if err != nil {
				if m.isRunning {
					log.Printf("Erro ao ler pacote da mesh: %v", err)
					if m.logger != nil {
						m.logger(fmt.Sprintf("[ERROR] Erro ao ler pacote da mesh: %v", err))
					}
				}
				continue
			}

			data := buffer[:n]
			packet, err := DecodePacket(data)
			if err != nil {
				log.Printf("Erro ao decodificar pacote de %s: %v", addr.String(), err)
				continue
			}

			peerID := hex.EncodeToString(packet.SenderID)


			m.mu.Lock()


			switch packet.Type {
			case Announce:
				if _, exists := m.peers[peerID]; !exists {
					log.Printf("Novo peer da mesh descoberto por Announce: %s (%s)", peerID, addr.String())
					if m.logger != nil {
						m.logger(fmt.Sprintf("Novo peer descoberto: %s", peerID))
					}
				} else {
					log.Printf("Peer existente atualizado por Announce: %s", peerID)
				}
				m.peers[peerID] = &MeshPeer{
					ID:       peerID,
					Address:  addr.String(),
					LastSeen: time.Now(),
				}

			case Leave:
				if _, exists := m.peers[peerID]; exists {
					log.Printf("Peer %s anunciou saída. Removendo.", peerID)
					if m.logger != nil {
						m.logger(fmt.Sprintf("Peer %s saiu.", peerID))
					}
					delete(m.peers, peerID)
				}

			case Message:
				msg, err := DecodeMessage(packet.Payload)
				if err != nil {
					log.Printf("Erro ao decodificar mensagem do peer %s: %v", peerID, err)
				} else if m.messageHandler != nil {
					m.messageHandler(peerID, msg.Channel, msg.Sender, msg.Content)
				} else {
					log.Printf("Mensagem recebida do peer %s, mas nenhum handler está configurado: [%s] %s: %s", peerID, msg.Channel, msg.Sender, msg.Content)
				}

			default:
				log.Printf("Recebido pacote mesh de tipo não tratado (%d) do peer %s", packet.Type, peerID)
			}

			m.mu.Unlock()

		}
	}
}

// SendMessage envia uma mensagem para a rede mesh.
func (m *MeshIntegration) SendMessage(peerID string, message string) error {
	m.mu.Lock()
	peer, exists := m.peers[peerID]
	m.mu.Unlock()

	if !exists {
		return fmt.Errorf("peer desconhecido: %s", peerID)
	}

	// Criar a mensagem interna
	msg := &BitchatMessage{
		Sender:    hex.EncodeToString(m.selfID), // Ou o nickname do usuário
		Content:   message,
		Timestamp: time.Now(),
	}
	payload, err := msg.Encode()
	if err != nil {
		return fmt.Errorf("falha ao codificar a mensagem: %w", err)
	}

	// Criar o pacote externo
	packet := &BitchatPacket{
		Version:   1,
		Type:      Message,
		TTL:       64,
		Timestamp: uint64(time.Now().UnixMilli()),
		SenderID:  m.selfID,
		Payload:   payload,
	}

	encodedPacket, err := packet.Encode()
	if err != nil {
		return fmt.Errorf("falha ao codificar o pacote: %w", err)
	}

	// Enviar para o endereço do peer
	addr, err := net.ResolveUDPAddr("udp", peer.Address)
	if err != nil {
		return fmt.Errorf("falha ao resolver o endereço do peer %s: %w", peerID, err)
	}

	_, err = m.listener.WriteTo(encodedPacket, addr)
	if err != nil {
		return fmt.Errorf("falha ao enviar pacote para o peer %s: %w", peerID, err)
	}

	log.Printf("Mensagem enviada para o peer %s", peerID)
	return nil
}

func (m *MeshIntegration) announce() error {
	log.Println("Anunciando presença na rede mesh...")
	packet := &BitchatPacket{
		Version:   1,
		Type:      Announce,
		TTL:       64,
		Timestamp: uint64(time.Now().UnixMilli()),
		SenderID:  m.selfID,
	}

	encodedPacket, err := packet.Encode()
	if err != nil {
		return fmt.Errorf("falha ao codificar pacote de announce: %w", err)
	}

	return m.broadcast(encodedPacket)
}

func (m *MeshIntegration) broadcast(data []byte) error {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("255.255.255.255:%d", m.config.Port))
	if err != nil {
		return fmt.Errorf("falha ao resolver endereço de broadcast: %w", err)
	}

	_, err = m.listener.WriteTo(data, addr)
	return err
}
