package socks5

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/miekg/dns"
	"golang.org/x/net/proxy"

	"github.com/merzzzl/warp/internal/utils/log"
	"github.com/merzzzl/warp/internal/utils/network"
)

type Config struct {
	User     string   `yaml:"user"`
	Password string   `yaml:"password"`
	Host     string   `yaml:"host"`
	Domain   string   `yaml:"domain"`
	IPs      []string `yaml:"ips"`
	DNS      []string `yaml:"dns"`
}

type Protocol struct {
	host   string
	dialer proxy.Dialer
	auth   *proxy.Auth
	domain string
	dns    []string
	ips    []string
	mx     sync.Mutex
}

func New(cfg *Config) (*Protocol, error) {
	var auth *proxy.Auth

	if cfg.User != "" && cfg.Password != "" {
		auth = &proxy.Auth{
			User:     cfg.User,
			Password: cfg.Password,
		}
	}

	dialer, err := proxy.SOCKS5("tcp", cfg.Host, auth, proxy.Direct)
	if err != nil {
		return nil, err
	}

	log.Debug().Str("url", fmt.Sprintf("%s", cfg.Host)).Msg("SOC", "open connection")

	return &Protocol{
		host:   cfg.Host,
		auth:   auth,
		dns:    cfg.DNS,
		dialer: dialer,
		domain: cfg.Domain,
		ips:    cfg.IPs,
	}, nil
}

func (p *Protocol) dial(n, addr string) (net.Conn, error) {
	for i := 0; ; i++ {
		log.Debug().Str("attempt", strconv.Itoa(i)).Str("dest", addr).Str("type", n).Msg("SOC", "open dial")

		conn, err := p.dialer.Dial(n, addr)
		if err == nil || i == 2 {
			return conn, err
		}

		if _, ok := err.(net.Error); ok {
			return nil, err
		}

		log.Warn().Str("dest", addr).Str("type", n).Str("url", fmt.Sprintf("%s", p.host)).Err(err).Msg("SOC", "reopen connection")

		if !p.mx.TryLock() {
			time.Sleep(1 * time.Second)

			continue
		}

		dialer, err := proxy.SOCKS5("tcp", p.host, p.auth, proxy.Direct)
		if err != nil {
			log.Error().Err(err).Msg("SOC", "failed to open socks5 tunnel")

			p.mx.Unlock()

			return nil, err
		}

		p.dialer = dialer

		p.mx.Unlock()

		time.Sleep(1 * time.Second)
	}
}

func (p *Protocol) Domain() string {
	return p.domain
}

func (p *Protocol) FixedIPs() []string {
	return p.ips
}

func (p *Protocol) LookupHost(_ context.Context, req *dns.Msg) *dns.Msg {
	for _, addr := range p.dns {
		dnsConn, err := p.dial("tcp", addr+":53")
		if err != nil {
			log.Error().Str("server", addr).DNS(req).Err(err).Msg("SOC", "handle dns req")

			continue
		}

		co := new(dns.Conn)
		co.Conn = dnsConn

		err = co.WriteMsg(req)
		if err != nil {
			log.Error().Str("server", addr).DNS(req).Err(err).Msg("SOC", "write dns req")

			continue
		}

		rsp, err := co.ReadMsg()
		if err != nil {
			log.Error().Str("server", addr).DNS(req).Err(err).Msg("SOC", "read dns req")

			continue
		}

		log.Debug().Str("server", addr).DNS(req).Msg("SOC", "handle dns req")

		if len(rsp.Answer) == 0 {
			continue
		}

		return rsp
	}

	return req
}

func (p *Protocol) HandleTCP(conn net.Conn) {
	remoteConn, err := p.dial(conn.LocalAddr().Network(), conn.LocalAddr().String())
	if err != nil {
		if !errors.Is(err, io.EOF) {
			log.Warn().Str("dest", conn.LocalAddr().String()).Str("type", conn.LocalAddr().Network()).Err(err).Msg("SSH", "handle conn")
		}

		return
	}

	log.Info().Str("dest", conn.LocalAddr().String()).Str("type", conn.LocalAddr().Network()).Msg("SOC", "handle conn")

	network.Transfer("SOC", conn, remoteConn)
}
