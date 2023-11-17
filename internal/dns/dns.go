package dns

import (
	"context"
	"net"

	"github.com/merzzzl/warp/internal/log"
	"github.com/merzzzl/warp/internal/routes"
	"github.com/merzzzl/warp/internal/sys"
	"github.com/merzzzl/warp/internal/sys/resolv"
	"github.com/miekg/dns"
)

type DNS struct {
	getters []DNSGetter
	server  *dns.Server
	host    net.IP
}

type DNSGetter func(host string) (net.IP, bool, error)

func NewDNS(getter ...DNSGetter) (*DNS, error) {
	ns := &DNS{}

	dns.HandleFunc(".", func(w dns.ResponseWriter, r *dns.Msg) {
		rsp, err := ns.serveDNS(r)
		if err != nil {
			log.Error().Err(err).Msg("DNS", "failed to handle dns request")
			w.WriteMsg(r)
			return
		}

		if err := w.WriteMsg(rsp); err != nil {
			log.Error().Err(err).Msg("DNS", "failed to write dns response")
			w.WriteMsg(r)
			return
		}
	})

	ip, err := routes.GetFreeHost()
	if err != nil {
		return nil, err
	}

	ns.getters = getter
	ns.server = &dns.Server{Addr: ip.String() + ":53", Net: "udp"}
	ns.host = ip

	return ns, nil
}

func (s *DNS) Start(ctx context.Context) error {
	log.Info().Str("host", s.server.Addr).Msg("DNS", "start dns server")
	defer log.Info().Str("host", s.server.Addr).Msg("DNS", "stop dns server")

	if err := resolv.SetDNS([]string{s.host.String()}); err != nil {
		log.Error().Err(err).Msg("DNS", "failed to set dns")
	}

	go func() {
		<-ctx.Done()
		s.server.Shutdown()
	}()

	for err := s.server.ListenAndServe(); err != nil; err = s.server.ListenAndServe() {
		if opErr, ok := err.(*net.OpError); ok && opErr.Op == "listen" {
			log.Error().Err(err).Msg("DNS", "attempt to release the port")

			sys.Command("kill -9 $(sudo lsof -i udp:53 -t)")
		} else {
			return err
		}
	}

	return nil
}

func (s *DNS) serveDNS(r *dns.Msg) (*dns.Msg, error) {
	m, ok, err := s.fetchOverGetter(r)
	if err != nil {
		return nil, err
	}

	if !ok {
		m, err = s.fetchLocalDNSRecords(r)
		if err != nil {
			return nil, err
		}
	}

	return m, nil
}

func (s *DNS) fetchOverGetter(r *dns.Msg) (*dns.Msg, bool, error) {
	rsp := new(dns.Msg)
	rsp.SetReply(r)

	for _, q := range r.Question {
		for _, getter := range s.getters {
			ip, ok, err := getter(q.Name)
			if !ok {
				continue
			}

			log.Info().Str("cdomain", q.Name).Msg("DNS", "handle remote")

			if err != nil {
				return nil, true, err
			}

			if len(ip) == 0 {
				continue
			}

			rsp.Answer = append(rsp.Answer, &dns.A{
				Hdr: dns.RR_Header{
					Name:   q.Name,
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
				},
				A: ip,
			})

			log.Info().Str("cdomain", q.Name).Str("ip", ip.String()).Msg("DNS", "dns resolved")
		}
	}

	return rsp, len(rsp.Answer) > 0, nil
}

func (s *DNS) fetchLocalDNSRecords(r *dns.Msg) (*dns.Msg, error) {
	c := new(dns.Client)

	for _, ns := range resolv.GetOriginalDNS() {
		response, _, err := c.Exchange(r, ns+":53")
		if err != nil {
			continue
		}

		return response, nil
	}

	return new(dns.Msg), nil
}
