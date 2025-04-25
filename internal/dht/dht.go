package dht

import (
    "sync"
)

// DHT é um stub simples para registro de canais e discovery de peers.
type DHT struct {
    mu    sync.RWMutex
    store map[string]map[string]bool // canal -> set de peers
}

// NewDHT cria uma instância de DHT stub.
func NewDHT() *DHT {
    return &DHT{store: make(map[string]map[string]bool)}
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