package transport

import (
	"testing"
	"time"
)

// TestListenDial verifies that a Listener can accept connections from Dial.
func TestListenDial(t *testing.T) {
	// Listen on a random available port
	l, err := Listen("127.0.0.1:0", nil)
	if err != nil {
		t.Fatalf("Listen error: %v", err)
	}
	defer l.Close()
	addr := l.Addr().String()

	done := make(chan struct{})
	// Accept connection and read a message
	go func() {
		conn, ok := <-l.AcceptCh
		if !ok {
			t.Error("AcceptCh closed unexpectedly")
			close(done)
			return
		}
		defer conn.Close()
		buf := make([]byte, 4)
		n, err := conn.Read(buf)
		if err != nil {
			t.Errorf("conn.Read error: %v", err)
		}
		if string(buf[:n]) != "ping" {
			t.Errorf("got %q, want \"ping\"", buf[:n])
		}
		close(done)
	}()

	// Give listener a moment to start
	time.Sleep(10 * time.Millisecond)

	conn, err := Dial(addr, nil)
	if err != nil {
		t.Fatalf("Dial error: %v", err)
	}
	defer conn.Close()
	msg := []byte("ping")
	n, err := conn.Write(msg)
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if n != len(msg) {
		t.Errorf("Write wrote %d bytes, want %d", n, len(msg))
	}

	// Wait for acceptance or timeout
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for connection acceptance")
	}
}
