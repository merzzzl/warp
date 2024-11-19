package local

import (
	"context"

	"github.com/miekg/dns"

	"github.com/merzzzl/warp/internal/utils/log"
	"github.com/merzzzl/warp/internal/utils/sys"
)

type Config struct {
	DNS []string
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
		log.Debug().Str("server", s).Msg("LOC", "handle dns req")

		res, _, err := cli.ExchangeContext(ctx, req, s+":53")
		if err != nil {
			continue
		}

		return res
	}

	return req
}
