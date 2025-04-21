package transport

import (
    "net"
)

// Dial connects to a peer at the given TCP address (e.g. "host:port").
func Dial(addr string) (net.Conn, error) {
    return net.Dial("tcp", addr)
}