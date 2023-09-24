package tun

import (
	"github.com/xjasonlyu/tun2socks/v2/core/adapter"
)

type ConnHandler interface {
	HandleTCP(TCPConn)
	HandleUDP(UDPConn)
}

type TCPConn = adapter.TCPConn
type UDPConn = adapter.UDPConn

type tunTransportHandler struct {
	tcpQueue chan adapter.TCPConn
	udpQueue chan adapter.UDPConn
	closeCh  chan struct{}
	adapter.TransportHandler
	connHandler ConnHandler
}

func newTunTransportHandler() *tunTransportHandler {
	handler := &tunTransportHandler{
		tcpQueue: make(chan adapter.TCPConn, 128),
		udpQueue: make(chan adapter.UDPConn, 128),
		closeCh:  make(chan struct{}, 1),
	}
	handler.TransportHandler = handler
	return handler
}

func (h *tunTransportHandler) registerConnHandler(handler ConnHandler) {
	h.connHandler = handler
}

func (h *tunTransportHandler) run() {
	go func() {
		defer func() { recover() }()
		for {
			select {
			case conn := <-h.tcpQueue:
				go h.handleTCPConn(conn)
			case conn := <-h.udpQueue:
				go h.handleUDPConn(conn)
			case <-h.closeCh:
				return
			}
		}
	}()
}

func (h *tunTransportHandler) finish() {
	h.closeCh <- struct{}{}
}

func (h *tunTransportHandler) HandleTCP(conn adapter.TCPConn) { h.tcpQueue <- conn }

func (h *tunTransportHandler) HandleUDP(conn adapter.UDPConn) { h.udpQueue <- conn }

func (h *tunTransportHandler) handleTCPConn(conn adapter.TCPConn) {
	defer conn.Close()
	if h.connHandler != nil {
		h.connHandler.HandleTCP(conn)
	}
}

func (h *tunTransportHandler) handleUDPConn(conn adapter.UDPConn) {
	defer conn.Close()

	if h.connHandler != nil {
		h.connHandler.HandleUDP(conn)
	}
}
