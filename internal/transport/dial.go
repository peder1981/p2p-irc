package transport

import (
   "bufio"
   "encoding/json"
   "errors"
   "io"
   "net"
   "sync"
   "time"

   "github.com/pion/webrtc/v3"
)

// Dial performs ICE handshake over an initial TCP connection, then returns
// a DataChannel-based net.Conn for peer-to-peer data.
func Dial(addr string, iceServers []webrtc.ICEServer) (net.Conn, error) {
   tcpConn, err := net.Dial("tcp", addr)
   if err != nil {
       return nil, err
   }
   conn, err := handshakeClient(tcpConn, iceServers)
   if err != nil {
       return nil, err
   }
   return conn, nil
}
// handshakeClient performs the WebRTC handshake as the client over signalingConn.
func handshakeClient(signalingConn net.Conn, iceServers []webrtc.ICEServer) (net.Conn, error) {
   settingEngine := webrtc.SettingEngine{}
   // trickle ICE disabled by waiting for ICE gathering to complete
   api := webrtc.NewAPI(webrtc.WithSettingEngine(settingEngine))
   pc, err := api.NewPeerConnection(webrtc.Configuration{ICEServers: iceServers})
   if err != nil {
       signalingConn.Close()
       return nil, err
   }
   dc, err := pc.CreateDataChannel("data", nil)
   if err != nil {
       signalingConn.Close()
       return nil, err
   }
   offer, err := pc.CreateOffer(nil)
   if err != nil {
       signalingConn.Close()
       return nil, err
   }
   // Wait for ICE gathering to complete before sending the offer (disable trickle ICE)
   gatherComplete := webrtc.GatheringCompletePromise(pc)
   if err := pc.SetLocalDescription(offer); err != nil {
       signalingConn.Close()
       return nil, err
   }
   <-gatherComplete
   if err := writeSignal(signalingConn, pc.LocalDescription()); err != nil {
       signalingConn.Close()
       return nil, err
   }
   var answer webrtc.SessionDescription
   if err := readSignal(signalingConn, &answer); err != nil {
       signalingConn.Close()
       return nil, err
   }
   if err := pc.SetRemoteDescription(answer); err != nil {
       signalingConn.Close()
       return nil, err
   }
   openCh := make(chan struct{})
   dc.OnOpen(func() { close(openCh) })
   select {
   case <-openCh:
   case <-time.After(30 * time.Second):
       signalingConn.Close()
       return nil, errors.New("timeout waiting for data channel open")
   }
   remoteAddr := signalingConn.RemoteAddr()
   localAddr := signalingConn.LocalAddr()
   signalingConn.Close()
   return newDataChannelConn(dc, remoteAddr, localAddr), nil
}
// readSignal reads a JSON-encoded SessionDescription from conn.
func readSignal(conn net.Conn, sd *webrtc.SessionDescription) error {
   reader := bufio.NewReader(conn)
   line, err := reader.ReadString('\n')
   if err != nil {
       return err
   }
   return json.Unmarshal([]byte(line), sd)
}
// writeSignal writes a JSON-encoded SessionDescription to conn, suffixed with newline.
func writeSignal(conn net.Conn, sd *webrtc.SessionDescription) error {
   data, err := json.Marshal(sd)
   if err != nil {
       return err
   }
   data = append(data, '\n')
   _, err = conn.Write(data)
   return err
}
// dataChannelConn wraps a WebRTC DataChannel to implement net.Conn.
type dataChannelConn struct {
   dc         *webrtc.DataChannel
   readCh     chan []byte
   readBuf    []byte
   remoteAddr net.Addr
   localAddr  net.Addr
   closeCh    chan struct{}
   mu         sync.Mutex
   closed     bool
}
// newDataChannelConn creates a net.Conn backed by a WebRTC DataChannel.
func newDataChannelConn(dc *webrtc.DataChannel, remote, local net.Addr) net.Conn {
   conn := &dataChannelConn{
       dc:         dc,
       readCh:     make(chan []byte, 16),
       remoteAddr: remote,
       localAddr:  local,
       closeCh:    make(chan struct{}),
   }
   dc.OnMessage(func(msg webrtc.DataChannelMessage) {
       if msg.IsString {
           conn.readCh <- []byte(msg.Data)
       } else {
           conn.readCh <- msg.Data
       }
   })
   dc.OnClose(func() { close(conn.closeCh) })
   return conn
}
func (c *dataChannelConn) Read(b []byte) (n int, err error) {
   if len(c.readBuf) == 0 {
       select {
       case data, ok := <-c.readCh:
           if !ok {
               return 0, io.EOF
           }
           c.readBuf = data
       case <-c.closeCh:
           return 0, io.EOF
       }
   }
   n = copy(b, c.readBuf)
   c.readBuf = c.readBuf[n:]
   return n, nil
}
func (c *dataChannelConn) Write(b []byte) (n int, err error) {
   if err := c.dc.Send(b); err != nil {
       return 0, err
   }
   return len(b), nil
}
func (c *dataChannelConn) Close() error {
   c.mu.Lock()
   defer c.mu.Unlock()
   if c.closed {
       return nil
   }
   c.closed = true
   return c.dc.Close()
}
func (c *dataChannelConn) LocalAddr() net.Addr                { return c.localAddr }
func (c *dataChannelConn) RemoteAddr() net.Addr               { return c.remoteAddr }
func (c *dataChannelConn) SetDeadline(t time.Time) error      { return nil }
func (c *dataChannelConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *dataChannelConn) SetWriteDeadline(t time.Time) error { return nil }