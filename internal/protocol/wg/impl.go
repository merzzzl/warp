package wg

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/netip"
	"regexp"

	"github.com/MakeNowJust/heredoc"
	"github.com/miekg/dns"
	wgconn "golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun/netstack"

	"github.com/merzzzl/warp/internal/utils/log"
	"github.com/merzzzl/warp/internal/utils/network"
)

type Config struct {
	PrivateKey    string   `yaml:"private_key"`
	PeerPublicKey string   `yaml:"peer_public_key"`
	Endpoint      string   `yaml:"endpoint"`
	Domain        string   `yaml:"domain"`
	Address       string   `yaml:"address"`
	DNS           []string `yaml:"dns"`
	IPs           []string `yaml:"ips"`
}

type Protocol struct {
	tnet   *netstack.Net
	domain *regexp.Regexp
	dns    []string
	ips    []string
}

var defaultMTU = 1480

func New(ctx context.Context, cfg *Config) (*Protocol, error) {
	var request bytes.Buffer

	privateKey, err := encodeBase64ToHex(cfg.PrivateKey)
	if err != nil {
		log.Error().Err(err).Msg("WRG", "invalid private key")
	}

	peerPublicKey, err := encodeBase64ToHex(cfg.PeerPublicKey)
	if err != nil {
		log.Error().Err(err).Msg("WRG", "invalid peer public key")
	}

	_, err = request.WriteString(fmt.Sprintf(
		heredoc.Doc(`
			private_key=%s
			public_key=%s
			endpoint=%s
			persistent_keepalive_interval=5
			allowed_ip=0.0.0.0/0
			allowed_ip=::0/0
		`),
		privateKey, peerPublicKey, cfg.Endpoint,
	))
	if err != nil {
		return nil, err
	}

	localAddress, err := netip.ParseAddr(cfg.Address)
	if err != nil {
		return nil, err
	}

	dnss := make([]netip.Addr, 0, len(cfg.DNS))

	for i := range cfg.DNS {
		addr, err := netip.ParseAddr(cfg.DNS[i])
		if err != nil {
			return nil, err
		}

		dnss = append(dnss, addr)
	}

	tun, tnet, err := netstack.CreateNetTUN([]netip.Addr{localAddress}, dnss, defaultMTU)
	if err != nil {
		return nil, err
	}

	wglog := device.Logger{
		Verbosef: func(format string, args ...any) {
			log.Debug().Msgf("WRG", format, args...)
		},
		Errorf: func(format string, args ...any) {
			log.Error().Msgf("WRG", format, args...)
		},
	}

	dev := device.NewDevice(tun, wgconn.NewDefaultBind(), &wglog)

	err = dev.IpcSet(request.String())
	if err != nil {
		return nil, err
	}

	err = dev.Up()
	if err != nil {
		return nil, err
	}

	go func() {
		<-ctx.Done()

		dev.Close()
	}()

	var rx *regexp.Regexp
	if cfg.Domain != "" {
		rx, err = regexp.Compile(cfg.Domain)
		if err != nil {
			return nil, err
		}
	}

	return &Protocol{
		domain: rx,
		tnet:   tnet,
		dns:    cfg.DNS,
		ips:    cfg.IPs,
	}, nil
}

func (p *Protocol) FixedIPs() []string {
	return p.ips
}

func (p *Protocol) LookupHost(ctx context.Context, req *dns.Msg) (*dns.Msg, error) {
	if p.domain == nil {
		return &dns.Msg{}, nil
	}

	for _, que := range req.Question {
		if !p.domain.MatchString(que.Name[:len(que.Name)-1]) {
			return req, nil
		}
	}

	for _, addr := range p.dns {
		dial, err := p.tnet.DialContext(ctx, "udp", addr+":53")
		if err != nil {
			return nil, err
		}

		co := new(dns.Conn)
		co.Conn = dial

		err = co.WriteMsg(req)
		if err != nil {
			return nil, err
		}

		rsp, err := co.ReadMsg()
		if err != nil {
			return nil, err
		}

		if len(rsp.Answer) == 0 {
			continue
		}

		return rsp, nil
	}

	return req, nil
}

func (p *Protocol) HandleTCP(conn net.Conn) {
	log.Info().Str("dest", conn.LocalAddr().String()).Str("type", "TCP").Msg("WRG", "handle conn")

	remoteConn, err := p.tnet.Dial(conn.LocalAddr().Network(), conn.LocalAddr().String())
	if err != nil {
		log.Error().Err(err).Msg("WRG", "failed to connect to remote host")

		return
	}

	network.Transfer("WRG", conn, remoteConn)
}

func (p *Protocol) HandleUDP(conn net.Conn) {
	log.Info().Str("dest", conn.LocalAddr().String()).Str("type", "UDP").Msg("WRG", "handle conn")

	remoteConn, err := p.tnet.Dial(conn.LocalAddr().Network(), conn.LocalAddr().String())
	if err != nil {
		log.Error().Err(err).Msg("WRG", "failed to connect to remote host")

		return
	}

	network.Transfer("WRG", conn, remoteConn)
}
