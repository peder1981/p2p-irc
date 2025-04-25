package transport

import (
   "errors"
   "net"
   "time"

   "github.com/pion/webrtc/v3"
)

// Listener wraps a TCP listener and provides an Accept channel for connections.
type Listener struct {
    listener net.Listener
    // AcceptCh receives incoming net.Conn objects.
    AcceptCh chan net.Conn
}

// Listen starts listening on the given TCP address and performs ICE handshake with each peer.
func Listen(addr string, iceServers []webrtc.ICEServer) (*Listener, error) {
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
           tcpConn, err := ln.Accept()
           if err != nil {
               return
           }
           go func(conn net.Conn) {
               chConn, err := handshakeServer(conn, iceServers)
               if err != nil {
                   return
               }
               l.AcceptCh <- chConn
           }(tcpConn)
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
// handshakeServer performs the WebRTC handshake as the server over signalingConn.
func handshakeServer(signalingConn net.Conn, iceServers []webrtc.ICEServer) (net.Conn, error) {
   remoteAddr := signalingConn.RemoteAddr()
   localAddr := signalingConn.LocalAddr()
   settingEngine := webrtc.SettingEngine{}
   // trickle ICE disabled by waiting for ICE gathering to complete
   api := webrtc.NewAPI(webrtc.WithSettingEngine(settingEngine))
   pc, err := api.NewPeerConnection(webrtc.Configuration{ICEServers: iceServers})
   if err != nil {
       signalingConn.Close()
       return nil, err
   }
   dcChan := make(chan *webrtc.DataChannel, 1)
   pc.OnDataChannel(func(d *webrtc.DataChannel) { dcChan <- d })
   var offer webrtc.SessionDescription
   if err := readSignal(signalingConn, &offer); err != nil {
       signalingConn.Close()
       return nil, err
   }
   if err := pc.SetRemoteDescription(offer); err != nil {
       signalingConn.Close()
       return nil, err
   }
   answer, err := pc.CreateAnswer(nil)
   if err != nil {
       signalingConn.Close()
       return nil, err
   }
   // Wait for ICE gathering to complete before sending the answer (disable trickle ICE)
   gatherComplete := webrtc.GatheringCompletePromise(pc)
   if err := pc.SetLocalDescription(answer); err != nil {
       signalingConn.Close()
       return nil, err
   }
   <-gatherComplete
   if err := writeSignal(signalingConn, pc.LocalDescription()); err != nil {
       signalingConn.Close()
       return nil, err
   }
   var dc *webrtc.DataChannel
   select {
   case dc = <-dcChan:
   case <-time.After(30 * time.Second):
       signalingConn.Close()
       return nil, errors.New("timeout waiting for data channel")
   }
   openCh := make(chan struct{})
   dc.OnOpen(func() { close(openCh) })
   select {
   case <-openCh:
   case <-time.After(30 * time.Second):
       signalingConn.Close()
       return nil, errors.New("timeout waiting for data channel open")
   }
   signalingConn.Close()
   return newDataChannelConn(dc, remoteAddr, localAddr), nil
}