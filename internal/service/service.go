package service

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/miekg/dns"
	"github.com/xjasonlyu/tun2socks/v2/core"
	"github.com/xjasonlyu/tun2socks/v2/core/adapter"
	"github.com/xjasonlyu/tun2socks/v2/core/device/tun"
	"github.com/xjasonlyu/tun2socks/v2/core/option"

	"github.com/merzzzl/warp/internal/protocol/local"
	"github.com/merzzzl/warp/internal/utils/log"
	"github.com/merzzzl/warp/internal/utils/sys"
)

type Config struct {
	Name string
	IP   string
}

type trafficConn struct {
	net.Conn
	traffic *Traffic
}

type Traffic struct {
	tarificationMutexLastCheck time.Time
	tarificationMutex          sync.Mutex
	transferredIn              atomic.Int64
	transferredOut             atomic.Int64
}

type Routes struct {
	list    map[string]Protocol
	gateway string
	mutex   sync.RWMutex
}

type Protocol interface {
	HandleTCP(conn net.Conn)
	HandleUDP(conn net.Conn)
	LookupHost(ctx context.Context, req *dns.Msg) (*dns.Msg, error)
}

type tunTransportHandler struct {
	tcpQueue chan adapter.TCPConn
	udpQueue chan adapter.UDPConn
	closeCh  chan struct{}
	adapter.TransportHandler
	routes  *Routes
	traffic *Traffic
}

type Service struct {
	routes  *Routes
	traffic *Traffic
	name    string
	addr    string
}

var (
	defaultMTU uint32 = 1480
	lo0               = "127.0.0.1"
)

// New create a tun device and return the Tunnel.
func New(config *Config) (*Service, error) {
	routes := &Routes{
		gateway: config.IP,
		list:    make(map[string]Protocol),
	}

	traffic := &Traffic{
		tarificationMutexLastCheck: time.Now(),
	}

	s := &Service{
		name:    config.Name,
		addr:    config.IP,
		routes:  routes,
		traffic: traffic,
	}

	return s, nil
}

func newTunTransportHandler(routes *Routes, traffic *Traffic) *tunTransportHandler {
	handler := &tunTransportHandler{
		tcpQueue: make(chan adapter.TCPConn, 128),
		udpQueue: make(chan adapter.UDPConn, 128),
		closeCh:  make(chan struct{}, 1),
	}

	handler.TransportHandler = handler
	handler.routes = routes
	handler.traffic = traffic

	return handler
}

func (h *tunTransportHandler) run() {
	defer func() {
		if r := recover(); r != nil {
			log.Error().Msgf("SYS", "transport panic: %v", r)
		}
	}()

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
}

func (h *tunTransportHandler) finish() {
	h.closeCh <- struct{}{}
}

func (h *tunTransportHandler) HandleTCP(conn adapter.TCPConn) { h.tcpQueue <- conn }

func (h *tunTransportHandler) HandleUDP(conn adapter.UDPConn) { h.udpQueue <- conn }

func (h *tunTransportHandler) handleTCPConn(conn adapter.TCPConn) {
	defer conn.Close()

	if handler := h.routes.get(strings.Split(conn.LocalAddr().String(), ":")[0]); handler != nil {
		handler.HandleTCP(h.traffic.newConn(conn))

		return
	}

	log.Error().Msgf("TUN", "no handler for tcp connection to: %s", conn.LocalAddr())
}

func (h *tunTransportHandler) handleUDPConn(conn adapter.UDPConn) {
	defer conn.Close()

	if handler := h.routes.get(strings.Split(conn.LocalAddr().String(), ":")[0]); handler != nil {
		handler.HandleUDP(h.traffic.newConn(conn))

		return
	}

	log.Error().Msgf("TUN", "no handler for tcp connection to: %s", conn.LocalAddr())
}

// GetRoutes returns Routes.
func (t *Service) GetRoutes() *Routes {
	return t.routes
}

// GetAll returns all routes.
func (r *Routes) GetAll() []string {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	list := make([]string, 0, len(r.list))

	for route := range r.list {
		list = append(list, route)
	}

	return list
}

func (r *Routes) get(route string) Protocol {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	return r.list[route]
}

func (r *Routes) add(ip string, hand Protocol) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if _, ok := r.list[ip]; ok {
		return
	}

	if err := sys.AddRoute(ip, r.gateway); err != nil {
		log.Error().Err(err).Str("ip", ip).Msg("TUN", "failed to add route")

		return
	}

	r.list[ip] = hand

	log.Info().Str("ip", ip).Msg("TUN", "route added")
}

// ListenAndServe listens on the given address and serves DNS requests using the provided resolvers.
func (t *Service) ListenAndServe(ctx context.Context, protocols []Protocol) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	dev, err := tun.Open(t.name, defaultMTU)
	if err != nil {
		return err
	}

	handler := newTunTransportHandler(t.routes, t.traffic)

	coreStack, err := core.CreateStack(&core.Config{
		LinkEndpoint:     dev,
		TransportHandler: handler,
		Options:          []option.Option{},
	})
	if err != nil {
		return err
	}

	err = sys.CreateTun(t.name, t.addr, defaultMTU)
	if err != nil {
		return err
	}

	pc, err := newPacketConn(lo0 + ":53")
	if err != nil {
		return err
	}

	if err := sys.SetDNS([]string{lo0}); err != nil {
		return err
	}

	dns.HandleFunc(".", func(w dns.ResponseWriter, r *dns.Msg) {
		go func() {
			rsp, err := t.serveDNS(ctx, protocols, r)
			if err != nil {
				log.Error().Err(err).Msg("DNS", "failed to handle dns request")

				return
			}

			if err := w.WriteMsg(rsp); err != nil {
				log.Error().Err(err).Msg("DNS", "failed to write dns response")

				return
			}
		}()
	})

	log.Info().Str("host", lo0+":53").Msg("TUN", "start tun interface")
	defer log.Info().Str("host", lo0+":53").Msg("TUN", "stop tun interface")

	go handler.run()

	log.Info().Str("host", lo0+":53").Msg("DNS", "start dns server")
	defer log.Info().Str("host", lo0+":53").Msg("DNS", "stop dns server")

	go func() {
		if err := dns.ActivateAndServe(nil, pc, nil); !errors.Is(err, net.ErrClosed) {
			log.Error().Err(err).Msgf("DNS", "activate failed")

			cancel()
		}
	}()

	<-ctx.Done()

	if err := sys.DeleteTun(t.name); err != nil {
		log.Error().Err(err).Msg("TUN", "failed to delete tun")
	}

	coreStack.Close()

	if err := dev.Close(); err != nil {
		log.Error().Err(err).Msg("TUN", "failed to close device")
	}

	handler.finish()

	if err := pc.Close(); err != nil {
		log.Error().Err(err).Msg("DNS", "failed to close conn")
	}

	if err := sys.RestoreDNS(); err != nil {
		log.Error().Err(err).Msg("DNS", "failed to restore dns")
	}

	return nil
}

func newPacketConn(host string) (net.PacketConn, error) {
	for {
		pc, err := net.ListenPacket("udp", host)
		if err != nil {
			if opErr, ok := err.(*net.OpError); !ok || opErr.Op != "listen" {
				return nil, err
			}

			log.Error().Err(err).Msg("DNS", "attempt to release the port")

			if _, err = sys.Command("kill -9 $(sudo lsof -i udp:53 -t)"); err != nil {
				log.Error().Err(err).Msg("DNS", "failed on stop system DNS server")
			}

			continue
		}

		return pc, nil
	}
}

func (t *Service) serveDNS(ctx context.Context, protocols []Protocol, req *dns.Msg) (*dns.Msg, error) {
	for _, protocol := range protocols {
		rsp, err := lookupHost(ctx, protocol, req)
		if err != nil {
			return nil, err
		}

		if len(rsp.Answer) == 0 {
			continue
		}

		if _, ok := protocol.(*local.Protocol); !ok {
			log.Info().DNS(rsp).Msg("DNS", "dns resolved")

			for _, ans := range rsp.Answer {
				if a, ok := ans.(*dns.A); ok {
					t.routes.add(a.A.String(), protocol)
				}
			}
		}

		return rsp, nil
	}

	return req, nil
}

func lookupHost(ctx context.Context, protocol Protocol, req *dns.Msg) (*dns.Msg, error) {
	ctx, cancel := context.WithDeadline(ctx, time.Now().Add(time.Second*10))
	defer cancel()

	rsp, err := protocol.LookupHost(ctx, req)
	if err != nil {
		return nil, err
	}

	return rsp, nil
}

// GetRates returns the rates for in and out traffic.
func (t *Traffic) GetRates() (float64, float64) {
	in := t.transferredIn.Swap(0)
	out := t.transferredOut.Swap(0)

	t.tarificationMutex.Lock()

	sec := time.Since(t.tarificationMutexLastCheck).Seconds()
	t.tarificationMutexLastCheck = time.Now()

	t.tarificationMutex.Unlock()

	inRate := float64(in*8) / sec
	outRate := float64(out*8) / sec

	return inRate, outRate
}

func (t *Traffic) newConn(conn net.Conn) *trafficConn {
	return &trafficConn{
		Conn:    conn,
		traffic: t,
	}
}

// Read implements the io.Reader interface for a Conn.
func (t *trafficConn) Read(p []byte) (n int, err error) {
	s, err := t.Conn.Read(p)

	t.traffic.transferredIn.Add(int64(s))

	return s, err
}

// Write implements the io.Writer interface for a Conn.
func (t *trafficConn) Write(p []byte) (n int, err error) {
	s, err := t.Conn.Write(p)

	t.traffic.transferredOut.Add(int64(s))

	return s, err
}

// GetTraffic returns the Traffic for this Service.
func (t *Service) GetTraffic() *Traffic {
	return t.traffic
}
