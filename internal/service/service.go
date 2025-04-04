package service

import (
	"context"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/miekg/dns"
	"github.com/seancfoley/ipaddress-go/ipaddr"
	"github.com/xjasonlyu/tun2socks/v2/core"
	"github.com/xjasonlyu/tun2socks/v2/core/adapter"
	"github.com/xjasonlyu/tun2socks/v2/core/device/tun"
	"github.com/xjasonlyu/tun2socks/v2/core/option"

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
	transferredInSum           atomic.Int64
	transferredOutSum          atomic.Int64
}

type Routes struct {
	list    map[string]Protocol
	gateway string
	mutex   sync.RWMutex
}

type Protocol interface {
	Domain() string
	LookupHost(ctx context.Context, req *dns.Msg) *dns.Msg
}

type protocolFixedIPs interface {
	FixedIPs() []string
}

type protocolHandleUDP interface {
	HandleUDP(conn net.Conn)
}

type protocolHandleTCP interface {
	HandleTCP(conn net.Conn)
}

type tunTransportHandler struct {
	addr     string
	tcpQueue chan adapter.TCPConn
	udpQueue chan adapter.UDPConn
	closeCh  chan struct{}
	adapter.TransportHandler
	routes    *Routes
	traffic   *Traffic
	protocols []Protocol
	ipv6      bool
}

type Service struct {
	routes  *Routes
	traffic *Traffic
	name    string
	addr    string
}

var defaultMTU uint32 = 1280

// New create a tun device and return the Tunnel.
func New(config *Config) (*Service, error) {
	routes := &Routes{
		gateway: config.Name,
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

func newTunTransportHandler(routes *Routes, traffic *Traffic, protocols []Protocol, addr string, ipv6 bool) *tunTransportHandler {
	handler := &tunTransportHandler{
		tcpQueue:  make(chan adapter.TCPConn, 128),
		udpQueue:  make(chan adapter.UDPConn, 128),
		closeCh:   make(chan struct{}, 1),
		protocols: protocols,
		addr:      addr,
		ipv6:      ipv6,
	}

	handler.TransportHandler = handler
	handler.routes = routes
	handler.traffic = traffic

	return handler
}

func (h *tunTransportHandler) run(ctx context.Context) {
	for {
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Error().Msgf("SYS", "transport panic: %v", r)
				}
			}()

			select {
			case conn := <-h.tcpQueue:
				go h.handleTCPConn(ctx, conn)
			case conn := <-h.udpQueue:
				go h.handleUDPConn(ctx, conn)
			case <-h.closeCh:
				return
			}
		}()
	}
}

func (h *tunTransportHandler) finish() {
	h.closeCh <- struct{}{}
}

func (h *tunTransportHandler) HandleTCP(conn adapter.TCPConn) { h.tcpQueue <- conn }

func (h *tunTransportHandler) HandleUDP(conn adapter.UDPConn) { h.udpQueue <- conn }

func (h *tunTransportHandler) handleTCPConn(ctx context.Context, conn adapter.TCPConn) {
	defer conn.Close()

	sip := strings.Split(conn.LocalAddr().String(), ":")
	if sip[0] == h.addr && sip[1] == "53" {
		h.handleDNS(ctx, conn)

		return
	}

	if handler := h.routes.get(strings.Split(conn.LocalAddr().String(), ":")[0]); handler != nil {
		if handler, ok := handler.(protocolHandleTCP); ok {
			handler.HandleTCP(h.traffic.newConn(conn))

			return
		}
	}

	log.Warn().Msgf("TUN", "no handler for tcp connection to: %s", conn.LocalAddr())
}

func (h *tunTransportHandler) handleUDPConn(ctx context.Context, conn adapter.UDPConn) {
	defer conn.Close()

	sip := strings.Split(conn.LocalAddr().String(), ":")
	if sip[0] == h.addr && sip[1] == "53" {
		h.handleDNS(ctx, conn)

		return
	}

	if handler := h.routes.get(strings.Split(conn.LocalAddr().String(), ":")[0]); handler != nil {
		if handler, ok := handler.(protocolHandleUDP); ok {
			handler.HandleUDP(h.traffic.newConn(conn))

			return
		}
	}

	log.Warn().Msgf("TUN", "no handler for udp connection to: %s", conn.LocalAddr())
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

	ips := make([]*ipaddr.IPAddress, 0, len(list))

	for _, ipstr := range list {
		addr := ipaddr.NewIPAddressString(ipstr)
		if ip := addr.GetAddress(); ip != nil {
			ips = append(ips, ip)
		}
	}

	ipsv4, ipsv6 := ipaddr.MergeToPrefixBlocks(ips...)
	ipsv4 = append(ipsv4, ipsv6...)

	list = make([]string, 0, len(ipsv4))

	for _, ip := range ipsv4 {
		list = append(list, ip.String())
	}

	return list
}

func (r *Routes) get(route string) any {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	for k, v := range r.list {
		if ipaddr.NewIPAddressString(k).Contains(ipaddr.NewIPAddressString(route)) {
			return v
		}
	}

	return nil
}

func (r *Routes) add(ip string, hand Protocol) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	for k := range r.list {
		if ipaddr.NewIPAddressString(k).Contains(ipaddr.NewIPAddressString(ip)) {
			log.Debug().Str("ip", ip).Str("exists", k).Msg("TUN", "add route")

			return
		}
	}

	if err := sys.AddRoute(ip, r.gateway); err != nil {
		log.Error().Err(err).Str("ip", ip).Msg("TUN", "add route")

		return
	}

	r.list[ip] = hand

	log.Info().Str("ip", ip).Msg("TUN", "add route")
}

// ListenAndServe listens on the given address and serves DNS requests using the provided resolvers.
func (t *Service) ListenAndServe(ctx context.Context, protocols []Protocol, ipv6 bool) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	dev, err := tun.Open(t.name, defaultMTU)
	if err != nil {
		return err
	}

	handler := newTunTransportHandler(t.routes, t.traffic, protocols, t.addr, ipv6)

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

	for _, p := range protocols {
		domain := p.Domain()
		if domain == "" {
			continue
		}

		if err := sys.SetDNS(t.addr, domain); err != nil {
			return err
		}
	}

	log.Info().Str("host", t.addr+":53").Msg("TUN", "start tun interface")
	defer log.Info().Str("host", t.addr+":53").Msg("TUN", "stop tun interface")

	go handler.run(ctx)

	log.Info().Str("host", t.addr+":53").Msg("DNS", "start dns server")
	defer log.Info().Str("host", t.addr+":53").Msg("DNS", "stop dns server")

	for _, protocol := range protocols {
		if p, ok := protocol.(protocolFixedIPs); ok {
			for _, ip := range p.FixedIPs() {
				handler.routes.add(ip, protocol)
			}
		}
	}

	<-ctx.Done()

	if err := sys.DeleteTun(t.name); err != nil {
		log.Error().Err(err).Msg("TUN", "delete tun")
	}

	coreStack.Close()

	if err := dev.Close(); err != nil {
		log.Error().Err(err).Msg("TUN", "close device")
	}

	handler.finish()

	for _, p := range protocols {
		domain := p.Domain()
		if domain == "" {
			continue
		}

		if err := sys.RestoreDNS(domain); err != nil {
			log.Error().Err(err).Msg("DNS", "restore dns")
		}
	}

	return nil
}

func (h *tunTransportHandler) handleDNS(ctx context.Context, conn net.Conn) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	defer conn.Close()

	b := make([]byte, 1024)

	n, err := conn.Read(b)
	if err != nil {
		log.Warn().Err(err).Msg("DNS", "read msg")

		return
	}

	b = b[:n]

	msg := new(dns.Msg)

	err = msg.Unpack(b)
	if err != nil {
		log.Warn().Err(err).Msg("DNS", "unpack msg")

		return
	}

	b, err = h.serveDNS(ctx, msg).Pack()
	if err != nil {
		log.Debug().Err(err).Msg("DNS", "serve dns")

		return
	}

	_, err = conn.Write(b)
	if err != nil {
		log.Warn().Err(err).Msg("DNS", "write dns")

		return
	}
}

func isIPV6Request(req *dns.Msg) bool {
	for _, q := range req.Question {
		if q.Qtype == dns.TypeAAAA {
			return true
		}
	}
	return false
}

func emptyResponse(req *dns.Msg) *dns.Msg {
	rsp := new(dns.Msg)
	rsp.SetReply(req)
	rsp.Authoritative = true
	rsp.Rcode = dns.RcodeSuccess
	return rsp
}

func (h *tunTransportHandler) serveDNS(ctx context.Context, req *dns.Msg) *dns.Msg {
	if !h.ipv6 && isIPV6Request(req) {
		log.Debug().Msg("DNS", "drop ipv6 request")

		return emptyResponse(req)
	}

	for _, protocol := range h.protocols {
		if !strings.HasSuffix(req.Question[0].Name, protocol.Domain()+".") {
			continue
		}

		rsp := protocol.LookupHost(ctx, req.Copy())

		if len(rsp.Answer) == 0 {
			continue
		}

		log.Info().DNS(rsp).Msg("DNS", "resolve host")

		// if protocol, ok := protocol.(protocolFixedIPs); ok {
		// 	if len(protocol.FixedIPs()) > 0 {
		// 		log.Debug().DNS(rsp).Msg("DNS", "use fixed ips")

		// 		return rsp
		// 	}
		// }

		for _, ans := range rsp.Answer {
			if a, ok := ans.(*dns.A); ok {
				h.routes.add(a.A.String(), protocol)
			}
		}

		return rsp
	}

	return req
}

// GetRates returns the rates for in and out traffic.
func (t *Traffic) GetRates() (float64, float64) {
	in := t.transferredIn.Swap(0)
	out := t.transferredOut.Swap(0)

	t.tarificationMutex.Lock()

	sec := time.Since(t.tarificationMutexLastCheck).Seconds()
	t.tarificationMutexLastCheck = time.Now()

	t.tarificationMutex.Unlock()

	inRate := float64(in) / sec
	outRate := float64(out) / sec

	return inRate, outRate
}

// GetTransferred returns the transferred datat for in and out traffic.
func (t *Traffic) GetTransferred() (float64, float64) {
	return float64(t.transferredInSum.Load()), float64(t.transferredOutSum.Load())
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
	t.traffic.transferredInSum.Add(int64(s))

	return s, err
}

// Write implements the io.Writer interface for a Conn.
func (t *trafficConn) Write(p []byte) (n int, err error) {
	s, err := t.Conn.Write(p)

	t.traffic.transferredOut.Add(int64(s))
	t.traffic.transferredOutSum.Add(int64(s))

	return s, err
}

// GetTraffic returns the Traffic for this Service.
func (t *Service) GetTraffic() *Traffic {
	return t.traffic
}
