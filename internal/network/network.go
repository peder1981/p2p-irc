//go:build ignore
// +build ignore

package network

import (
    "context"
    "fmt"
    "time"

    libp2p "github.com/libp2p/go-libp2p"
    dht "github.com/libp2p/go-libp2p-kad-dht"
    pubsub "github.com/libp2p/go-libp2p-pubsub"
    "github.com/libp2p/go-libp2p-core/host"
    "github.com/libp2p/go-libp2p-core/peer"
    "github.com/libp2p/go-libp2p-core/peerstore"
)

// TopicName é o tópico PubSub para chat.
const TopicName = "p2p-irc"

// Network mantém host e inscrição PubSub.
type Network struct {
    Host  host.Host
    Topic *pubsub.Topic
    Sub   *pubsub.Subscription
}

// NewNetwork inicializa libp2p com DHT e PubSub.
func NewNetwork(ctx context.Context, bootstrap []string) (*Network, error) {
    h, err := libp2p.New(ctx)
    if err != nil {
        return nil, fmt.Errorf("host libp2p: %w", err)
    }
    // DHT global
    kadDHT, err := dht.New(ctx, h)
    if err != nil {
        return nil, fmt.Errorf("novo DHT: %w", err)
    }
    if err := kadDHT.Bootstrap(ctx); err != nil {
        return nil, fmt.Errorf("bootstrap DHT: %w", err)
    }
    // Conecta peers bootstrap
    for _, addr := range bootstrap {
        pi, err := peer.AddrInfoFromString(addr)
        if err != nil {
            fmt.Printf("bootstrap inválido %s: %v\n", addr, err)
            continue
        }
        h.Peerstore().AddAddrs(pi.ID, pi.Addrs, peerstore.PermanentAddrTTL)
        go func(pi peer.AddrInfo) {
            cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
            defer cancel()
            if err := h.Connect(cctx, pi); err != nil {
                fmt.Printf("erro conectar bootstrap: %v\n", err)
            }
        }(*pi)
    }
    // PubSub gossip
    ps, err := pubsub.NewGossipSub(ctx, h)
    if err != nil {
        return nil, fmt.Errorf("pubsub: %w", err)
    }
    topic, err := ps.Join(TopicName)
    if err != nil {
        return nil, fmt.Errorf("join tópico: %w", err)
    }
    sub, err := topic.Subscribe()
    if err != nil {
        return nil, fmt.Errorf("sub inscricao: %w", err)
    }
    return &Network{Host: h, Topic: topic, Sub: sub}, nil
}

// Publish envia msg para o tópico.
func (n *Network) Publish(ctx context.Context, msg string) error {
    return n.Topic.Publish(ctx, []byte(msg))
}

// Receive retorna canal de mensagens recebidas.
func (n *Network) Receive(ctx context.Context) (<-chan string, error) {
    ch := make(chan string)
    go func() {
        defer close(ch)
        for {
            m, err := n.Sub.Next(ctx)
            if err != nil {
                return
            }
            ch <- string(m.Data)
        }
    }()
    return ch, nil
}
