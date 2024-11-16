package local

import (
	"context"
	"net"

	"github.com/miekg/dns"

	"github.com/merzzzl/warp/internal/utils/sys"
)

type Protocol struct {
	servers []string
}

func New() *Protocol {
	return &Protocol{
		servers: sys.GetOriginalDNS(),
	}
}

// LookupHost returns a DNS response for the given request and server list.
func (p *Protocol) LookupHost(ctx context.Context, req *dns.Msg) (*dns.Msg, error) {
	cli := new(dns.Client)

	for _, s := range p.servers {
		res, _, err := cli.ExchangeContext(ctx, req, s+":53")
		if err != nil {
			continue
		}

		return res, nil
	}

	return req, nil
}

func (Protocol) HandleTCP(net.Conn) {}

func (Protocol) HandleUDP(net.Conn) {}

func (Protocol) FixedIPs() []string { return nil }
