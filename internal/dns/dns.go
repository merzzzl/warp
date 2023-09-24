package dns

import (
	"net"
	"strings"

	"github.com/merzzzl/warp/internal/log"
	"github.com/merzzzl/warp/internal/routes"
	"github.com/merzzzl/warp/internal/tun"
	"github.com/miekg/dns"
	"golang.org/x/crypto/ssh"
)

func X() {

}

type DNS struct {
	client      *ssh.Client
	routes      *routes.Routes
	originalDNS string
	remouteDNS  string
	Restore     func() error
	domain      string
}

func NewDNS(client *ssh.Client, route *routes.Routes, domain string) *DNS {
	return &DNS{
		client: client,
		routes: route,
		domain: domain,
	}
}

func (s *DNS) Apply(ip string) error {
	name, err := tun.DefaultRouteInterface()
	if err != nil {
		return err
	}

	rlv, err := tun.NewHandler(name)
	if err != nil {
		return err
	}

	if err := rlv.Set([]string{ip}); err != nil {
		return err
	}

	originalDNSs, err := rlv.OriginalDNS()
	if err != nil {
		return err
	}

	if len(originalDNSs) > 0 {
		s.originalDNS = originalDNSs[0]
	}

	s.Restore = rlv.Restore

	session, err := s.client.NewSession()
	if err != nil {
		return err
	}

	remouteDNS, err := session.CombinedOutput("scutil --dns | grep \"nameserver\\[.\\]\" | awk '{print $3}' | head -n 1")
	if err != nil {
		return err
	}

	if len(remouteDNS) != 0 {
		s.remouteDNS = strings.TrimSpace(string(remouteDNS))
		s.routes.Add(s.remouteDNS)
	}

	return nil
}

func (s *DNS) Handle(conn net.PacketConn) {
	for {
		buffer := make([]byte, 512)
		_, addr, err := conn.ReadFrom(buffer)
		if err != nil {
			return
		}

		req := new(dns.Msg)
		err = req.Unpack(buffer)
		if err != nil {
			log.Error().Err(err).Msg("DNS", "failed to unpack message")
			return
		}

		out, err := s.serveDNS(req)
		if err != nil {
			log.Error().Err(err).Msg("DNS", "failed to resolve dns")
			return
		}

		_, err = conn.WriteTo(out, addr)
		if err != nil {
			return
		}
	}
}

func (s *DNS) serveDNS(r *dns.Msg) (msg []byte, err error) {
	if s.reqContainsDomain(r) {
		m, err := s.fetchRemoteDNSRecords(r)
		if err != nil {
			return nil, err
		}

		return m.Pack()
	}

	m, err := s.fetchLocalDNSRecords(r)
	if err != nil {
		return nil, nil
	}

	return m.Pack()
}

func (s *DNS) fetchRemoteDNSRecords(r *dns.Msg) (*dns.Msg, error) {
	for _, q := range r.Question {
		log.Info().Str("cdomain", q.Name).Msg("DNS", "handle remoute dns")
	}

	c := new(dns.Client)
	c.Net = "tcp"

	response, _, err := c.Exchange(r, s.remouteDNS+":53")
	if err != nil {
		return nil, err
	}

	for _, ans := range response.Answer {
		switch t := ans.(type) {
		case *dns.A:
			s.routes.Add(t.A.String())
		default:
			continue
		}
	}

	return response, nil
}

func (s *DNS) fetchLocalDNSRecords(r *dns.Msg) (*dns.Msg, error) {
	c := new(dns.Client)

	response, _, err := c.Exchange(r, s.originalDNS+":53")
	if err != nil {
		return nil, err
	}

	return response, nil
}

func (s *DNS) reqContainsDomain(r *dns.Msg) bool {
	for _, q := range r.Question {
		if strings.HasSuffix(q.Name, s.domain) {
			return true
		}
	}

	return false
}
