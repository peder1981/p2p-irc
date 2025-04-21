package transport

import (
    "net"
)

// Listener wraps a TCP listener and provides an Accept channel for connections.
type Listener struct {
    listener net.Listener
    // AcceptCh receives incoming net.Conn objects.
    AcceptCh chan net.Conn
}

// Listen starts listening on the given TCP address (e.g. ":8080").
// It returns a Listener whose Addr can be retrieved via Addr().
func Listen(addr string) (*Listener, error) {
    ln, err := net.Listen("tcp", addr)
    if err != nil {
        return nil, err
    }
    l := &Listener{
        listener: ln,
        AcceptCh: make(chan net.Conn),
    }
    go func() {
        defer close(l.AcceptCh)
        for {
            conn, err := ln.Accept()
            if err != nil {
                return
            }
            l.AcceptCh <- conn
        }
    }()
    return l, nil
}

// Addr returns the listener's network address.
func (l *Listener) Addr() net.Addr {
    return l.listener.Addr()
}

// Close shuts down the listener.
func (l *Listener) Close() error {
    return l.listener.Close()
}