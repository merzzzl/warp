package local

import (
	"context"
	"net"

	"github.com/miekg/dns"

	"github.com/merzzzl/warp/internal/utils/log"
	"github.com/merzzzl/warp/internal/utils/sys"
)

type Config struct {
	DNS []string `yaml:"dns"`
}

type Protocol struct {
	servers []string
}

func New(cfg *Config) *Protocol {
	var servers []string

	if len(cfg.DNS) != 0 {
		servers = cfg.DNS
	} else {
		servers = sys.GetOriginalDNS()
	}

	return &Protocol{
		servers: servers,
	}
}

// LookupHost returns a DNS response for the given request and server list.
func (p *Protocol) LookupHost(ctx context.Context, req *dns.Msg) *dns.Msg {
	cli := new(dns.Client)

	for _, s := range p.servers {
		res, _, err := cli.ExchangeContext(ctx, req, s+":53")
		if err != nil {
			if err, ok := err.(net.Error); ok && err.Timeout() {
				log.Debug().Str("server", s).DNS(req).Msg("LOC", "dns req timeout")

				continue
			}

			log.Error().Str("server", s).DNS(req).Err(err).Msg("LOC", "handle dns req")

			continue
		}

		log.Debug().Str("server", s).DNS(req).Msg("LOC", "handle dns req")

		return res
	}

	return req
}
