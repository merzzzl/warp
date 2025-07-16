package wg

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"strconv"

	"github.com/MakeNowJust/heredoc"
	"github.com/miekg/dns"
	wgconn "golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun/netstack"

	"github.com/merzzzl/warp/internal/utils/log"
	"github.com/merzzzl/warp/internal/utils/network"
)

var errKeyInvalid = errors.New("invalid wireguard key")

type Config struct {
	PrivateKey    string   `yaml:"private_key"`
	PeerPublicKey string   `yaml:"peer_public_key"`
	Endpoint      string   `yaml:"endpoint"`
	Domains       []string `yaml:"domains"`
	Address       string   `yaml:"address"`
	DNS           []string `yaml:"dns"`
	IPs           []string `yaml:"ips"`
}

type Protocol struct {
	tnet    *netstack.Net
	domains []string
	dns     []string
	ips     []string
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

	log.Debug().Str("ip", localAddress.String()).Str("mtu", strconv.Itoa(defaultMTU)).Msg("WRG", "create tun")

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

	log.Debug().Str("ip", localAddress.String()).Str("mtu", strconv.Itoa(defaultMTU)).Msg("WRG", "create device")

	dev := device.NewDevice(tun, wgconn.NewDefaultBind(), &wglog)

	err = dev.IpcSet(request.String())
	if err != nil {
		return nil, err
	}

	log.Debug().Str("ip", localAddress.String()).Str("mtu", strconv.Itoa(defaultMTU)).Msg("WRG", "up device")

	err = dev.Up()
	if err != nil {
		return nil, err
	}

	go func() {
		<-ctx.Done()

		dev.Close()
	}()

	return &Protocol{
		domains: cfg.Domains,
		tnet:    tnet,
		dns:     cfg.DNS,
		ips:     cfg.IPs,
	}, nil
}

func encodeBase64ToHex(key string) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return "", fmt.Errorf("invalid base64 string (%s): %w", key, err)
	}

	if len(decoded) != 32 {
		return "", fmt.Errorf("key should be 32 bytes (%s): %w", key, errKeyInvalid)
	}

	return hex.EncodeToString(decoded), nil
}

func (p *Protocol) Domains() []string {
	return p.domains
}

func (p *Protocol) FixedIPs() []string {
	return p.ips
}

func (p *Protocol) LookupHost(ctx context.Context, req *dns.Msg) *dns.Msg {
	for _, addr := range p.dns {
		dial, err := p.tnet.DialContext(ctx, "udp", addr+":53")
		if err != nil {
			log.Error().Err(err).DNS(req).Msg("WRG", "failed to handle dns req")

			continue
		}

		co := new(dns.Conn)
		co.Conn = dial

		err = co.WriteMsg(req)
		if err != nil {
			log.Error().Err(err).DNS(req).Msg("WRG", "failed to handle dns req")

			continue
		}

		rsp, err := co.ReadMsg()
		if err != nil {
			log.Error().Err(err).DNS(req).Msg("WRG", "failed to handle dns req")

			continue
		}

		log.Debug().Str("server", addr).DNS(req).Msg("WRG", "handle dns req")

		if len(rsp.Answer) == 0 {
			continue
		}

		return rsp
	}

	return req
}

func (p *Protocol) HandleTCP(conn net.Conn) {
	remoteConn, err := p.tnet.Dial(conn.LocalAddr().Network(), conn.LocalAddr().String())
	if err != nil {
		if !errors.Is(err, io.EOF) {
			log.Warn().Str("dest", conn.LocalAddr().String()).Str("type", conn.LocalAddr().Network()).Err(err).Msg("SSH", "handle conn")
		}

		return
	}

	log.Info().Str("dest", conn.LocalAddr().String()).Str("type", conn.LocalAddr().Network()).Msg("WRG", "handle conn")

	network.Transfer("WRG", conn, remoteConn)
}

func (p *Protocol) HandleUDP(conn net.Conn) {
	remoteConn, err := p.tnet.Dial(conn.LocalAddr().Network(), conn.LocalAddr().String())
	if err != nil {
		if !errors.Is(err, io.EOF) {
			log.Warn().Str("dest", conn.LocalAddr().String()).Str("type", conn.LocalAddr().Network()).Err(err).Msg("SSH", "handle conn")
		}

		return
	}

	log.Info().Str("dest", conn.LocalAddr().String()).Str("type", conn.LocalAddr().Network()).Msg("WRG", "handle conn")

	network.Transfer("WRG", conn, remoteConn)
}
