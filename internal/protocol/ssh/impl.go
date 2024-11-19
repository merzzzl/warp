package ssh

import (
	"context"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
	"golang.org/x/crypto/ssh"

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
	config *ssh.ClientConfig
	cli    *ssh.Client
	domain *regexp.Regexp
	dns    []string
	ips    []string
	mx     sync.Mutex
}

func New(cfg *Config) (*Protocol, error) {
	sshConfig := &ssh.ClientConfig{
		User: cfg.User,
		Auth: []ssh.AuthMethod{
			ssh.Password(cfg.Password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	log.Debug().Str("url", fmt.Sprintf("%s@%s", sshConfig.User, cfg.Host)).Msg("SSH", "open connection")

	cli, err := ssh.Dial("tcp", cfg.Host+":22", sshConfig)
	if err != nil {
		return nil, err
	}

	var dnsList []string

	if len(cfg.DNS) != 0 {
		dnsList = cfg.DNS
	} else {
		log.Debug().Str("url", fmt.Sprintf("%s@%s", sshConfig.User, cfg.Host)).Msg("SSH", "get dns servers")

		session, err := cli.NewSession()
		if err != nil {
			return nil, err
		}

		defer session.Close()

		sshDNS, err := session.CombinedOutput("scutil --dns | grep \"nameserver\\[.\\]\" | awk '{print $3}' | head -n 1")
		if err != nil {
			return nil, err
		}

		if len(sshDNS) != 0 {
			dnsList = append(dnsList, strings.TrimSpace(string(sshDNS)))
		}
	}

	var rx *regexp.Regexp
	if cfg.Domain != "" {
		rx, err = regexp.Compile(cfg.Domain)
		if err != nil {
			return nil, err
		}
	}

	return &Protocol{
		host:   cfg.Host,
		config: sshConfig,
		dns:    dnsList,
		cli:    cli,
		domain: rx,
		ips:    cfg.IPs,
	}, nil
}

func (p *Protocol) dial(n, addr string) (net.Conn, error) {
	for i := 0; ; i++ {
		log.Debug().Str("attempt", strconv.Itoa(i)).Str("dest", addr).Str("type", n).Msg("SSH", "open dial")

		conn, err := p.cli.Dial(n, addr)
		if err == nil || i == 2 {
			return conn, err
		}

		if _, ok := err.(net.Error); ok {
			return nil, err
		}

		log.Warn().Str("url", fmt.Sprintf("%s@%s", p.config.User, p.host)).Msg("SSH", "reopen connection")

		if !p.mx.TryLock() {
			time.Sleep(1 * time.Second)

			continue
		}

		cli, err := ssh.Dial("tcp", p.host+":22", p.config)
		if err != nil {
			log.Error().Err(err).Msg("SSH", "failed to open ssh session")

			p.mx.Unlock()

			return nil, err
		}

		_ = p.cli.Close()
		p.cli = cli

		p.mx.Unlock()

		time.Sleep(1 * time.Second)
	}
}

func (p *Protocol) FixedIPs() []string {
	return p.ips
}

func (p *Protocol) LookupHost(_ context.Context, req *dns.Msg) *dns.Msg {
	if p.domain == nil {
		return req
	}

	for _, que := range req.Question {
		if !p.domain.MatchString(que.Name[:len(que.Name)-1]) {
			log.Debug().Str("question", que.Name).Str("regex", p.domain.String()).Msg("SSH", "does not match")

			return req
		}
	}

	for _, addr := range p.dns {
		dnsConn, err := p.dial("tcp", addr+":53")
		if err != nil {
			log.Error().Str("server", addr).Err(err).Msg("SSH", "handle dns req")

			continue
		}

		co := new(dns.Conn)
		co.Conn = dnsConn

		err = co.WriteMsg(req)
		if err != nil {
			log.Error().Str("server", addr).Err(err).Msg("SSH", "write dns req")

			continue
		}

		rsp, err := co.ReadMsg()
		if err != nil {
			log.Error().Str("server", addr).Err(err).Msg("SSH", "read dns req")

			continue
		}

		return rsp
	}

	return req
}

func (p *Protocol) HandleTCP(conn net.Conn) {
	remoteConn, err := p.dial(conn.LocalAddr().Network(), conn.LocalAddr().String())
	if err != nil {
		log.Warn().Str("dest", conn.LocalAddr().String()).Str("type", conn.LocalAddr().Network()).Err(err).Msg("SSH", "handle conn")

		return
	}

	log.Info().Str("dest", conn.LocalAddr().String()).Str("type", conn.LocalAddr().Network()).Msg("SSH", "handle conn")

	network.Transfer("SSH", conn, remoteConn)
}
