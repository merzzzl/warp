package dns

import (
	"context"
	"net"
	"strings"

	"github.com/merzzzl/warp/internal/kube"
	"github.com/merzzzl/warp/internal/log"
	"github.com/merzzzl/warp/internal/routes"
	"github.com/merzzzl/warp/internal/tun"
	"github.com/miekg/dns"
	"golang.org/x/crypto/ssh"
)

type DNS struct {
	clientSSH   *ssh.Client
	clientKube  *kube.KubeRoute
	routes      *routes.Routes
	originalDNS string
	sshDNS      string
	Restore     func() error
	domain      string
}

func NewDNS(clientSSH *ssh.Client, clientKube *kube.KubeRoute, route *routes.Routes, domain string) *DNS {
	return &DNS{
		clientSSH:  clientSSH,
		clientKube: clientKube,
		routes:     route,
		domain:     domain,
	}
}

func (s *DNS) ApplyK8S(ctx context.Context) error {
	s.clientKube.SetContext(ctx)

	if err := s.clientKube.LoadDomains(ctx); err != nil {
		return err
	}

	return nil
}

func (s *DNS) ApplySSH(ip string) error {
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

	session, err := s.clientSSH.NewSession()
	if err != nil {
		return err
	}

	sshDNS, err := session.CombinedOutput("scutil --dns | grep \"nameserver\\[.\\]\" | awk '{print $3}' | head -n 1")
	if err != nil {
		return err
	}

	if len(sshDNS) != 0 {
		s.sshDNS = strings.TrimSpace(string(sshDNS))
		s.routes.Add(s.sshDNS)
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
	if s.reqContainsSSHDomain(r) && s.sshDNS != "" {
		m, err := s.fetchSSHDNSRecords(r)
		if err != nil {
			return nil, err
		}

		return m.Pack()
	}

	if s.clientKube != nil && s.reqContainsK8SDomain(r) && s.clientKube != nil {
		m, err := s.fetchK8SDNSRecords(r)
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

func (s *DNS) fetchSSHDNSRecords(r *dns.Msg) (*dns.Msg, error) {
	for _, q := range r.Question {
		log.Info().Str("cdomain", q.Name).Msg("DNS", "handle ssh dns")
	}

	c := new(dns.Client)
	c.Net = "tcp"

	response, _, err := c.Exchange(r, s.sshDNS+":53")
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

func (s *DNS) fetchK8SDNSRecords(r *dns.Msg) (*dns.Msg, error) {
	response := new(dns.Msg)
	response.SetReply(r)

	for _, q := range r.Question {
		log.Info().Str("cdomain", q.Name).Msg("DNS", "handle k8s dns")

		ipAddress, err := s.clientKube.GetDNS(q.Name)
		if err != nil {
			return nil, err
		}

		switch q.Qtype {
		case dns.TypeA:
			rr := &dns.A{
				Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 3600},
				A:   ipAddress,
			}
			response.Answer = append(response.Answer, rr)
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

func (s *DNS) reqContainsK8SDomain(r *dns.Msg) bool {
	for _, q := range r.Question {
		if s.clientKube.IsKubeDomain(q.Name) {
			return true
		}
	}

	return false
}

func (s *DNS) reqContainsSSHDomain(r *dns.Msg) bool {
	for _, q := range r.Question {
		if strings.HasSuffix(q.Name, s.domain) {
			return true
		}
	}

	return false
}
