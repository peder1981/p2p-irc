package dht

import (
    "crypto/sha256"
    "encoding/hex"
    "net"
    "sort"
    "sync"
    "time"
)

const (
    // K é o número de nós mais próximos a manter em cada bucket
    K = 20

    // Alpha é o número de consultas paralelas em buscas de nós
    Alpha = 3

    // IDLength é o comprimento do ID do nó em bytes
    IDLength = 32
)

// Node representa um nó na rede DHT
type Node struct {
    ID         NodeID
    Addr       *net.UDPAddr
    LastSeen   time.Time
    InstanceID string // ID único da instância
}

// NodeID é o identificador único de um nó
type NodeID [IDLength]byte

// String retorna a representação em string do NodeID
func (n NodeID) String() string {
    return hex.EncodeToString(n[:])
}

// GenerateNodeID gera um NodeID a partir de uma string
func GenerateNodeID(input string) NodeID {
    hash := sha256.Sum256([]byte(input))
    return NodeID(hash)
}

// Bucket representa um k-bucket na tabela de roteamento Kademlia
type Bucket struct {
    nodes []Node
    mu    sync.RWMutex
}

// RoutingTable implementa a tabela de roteamento Kademlia
type RoutingTable struct {
    buckets    [IDLength * 8]*Bucket
    localID    NodeID
    mu         sync.RWMutex
    bucketSize int
}

// NewRoutingTable cria uma nova tabela de roteamento
func NewRoutingTable(localID NodeID, bucketSize int) *RoutingTable {
    rt := &RoutingTable{
        localID:    localID,
        bucketSize: bucketSize,
    }
    for i := 0; i < IDLength*8; i++ {
        rt.buckets[i] = &Bucket{}
    }
    return rt
}

// LocalID retorna o ID local da tabela
func (rt *RoutingTable) LocalID() NodeID {
    return rt.localID
}

// AddNode adiciona um nó à tabela de roteamento
func (rt *RoutingTable) AddNode(node Node) {
    // Não adiciona o nó local
    if node.ID == rt.localID {
        return
    }

    bucket := rt.getBucketForNode(node.ID)
    bucket.mu.Lock()
    defer bucket.mu.Unlock()

    // Verifica se o nó já existe
    for i, n := range bucket.nodes {
        if n.ID == node.ID {
            // Atualiza o nó existente
            bucket.nodes[i] = node
            return
        }
    }

    // Adiciona o nó se houver espaço
    if len(bucket.nodes) < rt.bucketSize {
        bucket.nodes = append(bucket.nodes, node)
        return
    }

    // Se o bucket estiver cheio, substitui o nó mais antigo se este não responder
    oldestNode := bucket.nodes[0]
    if time.Since(oldestNode.LastSeen) > 1*time.Hour {
        bucket.nodes[0] = node
    }
}

// RemoveNode remove um nó da tabela de roteamento
func (rt *RoutingTable) RemoveNode(nodeID NodeID) {
    bucket := rt.getBucketForNode(nodeID)
    bucket.mu.Lock()
    defer bucket.mu.Unlock()

    for i, node := range bucket.nodes {
        if node.ID == nodeID {
            bucket.nodes = append(bucket.nodes[:i], bucket.nodes[i+1:]...)
            return
        }
    }
}

// FindClosestNodes encontra os k nós mais próximos a um ID alvo
func (rt *RoutingTable) FindClosestNodes(target NodeID, count int) []Node {
    var closest []Node
    rt.mu.RLock()
    defer rt.mu.RUnlock()

    // Coleta todos os nós
    for _, bucket := range rt.buckets {
        bucket.mu.RLock()
        closest = append(closest, bucket.nodes...)
        bucket.mu.RUnlock()
    }

    // Ordena por distância ao alvo
    sort.Slice(closest, func(i, j int) bool {
        distI := xor(closest[i].ID, target)
        distJ := xor(closest[j].ID, target)
        return less(distI, distJ)
    })

    // Retorna os k mais próximos
    if len(closest) > count {
        closest = closest[:count]
    }
    return closest
}

// getBucketForNode retorna o bucket apropriado para um dado ID
func (rt *RoutingTable) getBucketForNode(id NodeID) *Bucket {
    distance := xor(rt.localID, id)
    for i := 0; i < IDLength*8; i++ {
        if getBit(distance, i) != 0 {
            return rt.buckets[i]
        }
    }
    return rt.buckets[IDLength*8-1]
}

// xor calcula o XOR entre dois NodeIDs
func xor(a, b NodeID) NodeID {
    var result NodeID
    for i := 0; i < IDLength; i++ {
        result[i] = a[i] ^ b[i]
    }
    return result
}

// less compara dois NodeIDs bit a bit
func less(a, b NodeID) bool {
    for i := 0; i < IDLength; i++ {
        if a[i] != b[i] {
            return a[i] < b[i]
        }
    }
    return false
}

// getBit retorna o bit na posição pos do NodeID
func getBit(id NodeID, pos int) byte {
    if pos >= IDLength*8 {
        return 0
    }
    return (id[pos/8] >> uint(7-pos%8)) & 0x1
}

// DHT representa a tabela hash distribuída
type DHT struct {
    store map[string]map[string]bool // chave -> conjunto de peers
    mu    sync.RWMutex
    rt    *RoutingTable
}

// NewDHT cria uma nova instância de DHT
func NewDHT(localID NodeID, bucketSize int) *DHT {
    return &DHT{
        store: make(map[string]map[string]bool),
        rt:    NewRoutingTable(localID, bucketSize),
    }
}

// Register adiciona o peer ao canal no DHT.
func (d *DHT) Register(channel, peer string) {
    d.mu.Lock()
    defer d.mu.Unlock()
    if d.store[channel] == nil {
        d.store[channel] = make(map[string]bool)
    }
    d.store[channel][peer] = true
}

// Peers retorna a lista de peers inscritos no canal.
func (d *DHT) Peers(channel string) []string {
    d.mu.RLock()
    defer d.mu.RUnlock()
    set := d.store[channel]
    peers := make([]string, 0, len(set))
    for p := range set {
        peers = append(peers, p)
    }
    return peers
}

// Channels retorna todos os canais registrados.
func (d *DHT) Channels() []string {
    d.mu.RLock()
    defer d.mu.RUnlock()
    chans := make([]string, 0, len(d.store))
    for ch := range d.store {
        chans = append(chans, ch)
    }
    return chans
}

// RegisterPeer registra um peer para uma chave
func (d *DHT) RegisterPeer(key string, peerID string) {
    d.mu.Lock()
    defer d.mu.Unlock()
    if d.store[key] == nil {
        d.store[key] = make(map[string]bool)
    }
    d.store[key][peerID] = true
}

// GetPeers retorna todos os peers para uma chave
func (d *DHT) GetPeers(key string) []string {
    d.mu.RLock()
    defer d.mu.RUnlock()
    if peers, ok := d.store[key]; ok {
        result := make([]string, 0, len(peers))
        for peer := range peers {
            result = append(result, peer)
        }
        return result
    }
    return nil
}

// RemovePeer remove um peer de todas as chaves
func (d *DHT) RemovePeer(peerID string) {
    d.mu.Lock()
    defer d.mu.Unlock()
    for _, peers := range d.store {
        delete(peers, peerID)
    }
}