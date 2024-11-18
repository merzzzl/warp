package ssh

import (
	"context"
	"net"
	"regexp"
	"strings"

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
}

func New(cfg *Config) (*Protocol, error) {
	sshConfig := &ssh.ClientConfig{
		User: cfg.User,
		Auth: []ssh.AuthMethod{
			ssh.Password(cfg.Password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	cli, err := ssh.Dial("tcp", cfg.Host+":22", sshConfig)
	if err != nil {
		return nil, err
	}

	var dnsList []string

	if len(cfg.DNS) != 0 {
		dnsList = cfg.DNS
	} else {
		session, err := cli.NewSession()
		if err != nil {
			return nil, err
		}

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

func (p *Protocol) open() {
	if session, err := p.cli.NewSession(); err == nil {
		_ = session.Close()

		return
	}

	cli, err := ssh.Dial("tcp", p.host+":22", p.config)
	if err != nil {
		log.Error().Err(err).Msg("SSH", "failed to open ssh session")

		return
	}

	_ = p.cli.Close()
	p.cli = cli
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
			return req
		}
	}

	for _, addr := range p.dns {
		dnsConn, err := p.cli.Dial("tcp", addr+":53")
		if err != nil {
			go p.open()

			log.Error().Err(err).Msg("SSH", "failed to handle dns req")

			continue
		}

		co := new(dns.Conn)
		co.Conn = dnsConn

		err = co.WriteMsg(req)
		if err != nil {
			log.Error().Err(err).Msg("SSH", "failed to handle dns req")

			continue
		}

		rsp, err := co.ReadMsg()
		if err != nil {
			log.Error().Err(err).Msg("SSH", "failed to handle dns req")

			continue
		}

		return rsp
	}

	return req
}

func (p *Protocol) HandleTCP(conn net.Conn) {
	log.Info().Str("dest", conn.LocalAddr().String()).Str("type", "TCP").Msg("SSH", "handle conn")

	remoteConn, err := p.cli.Dial(conn.LocalAddr().Network(), conn.LocalAddr().String())
	if err != nil {
		go p.open()
		log.Error().Err(err).Msg("SSH", "failed to connect to remote host")

		return
	}

	network.Transfer("SSH", conn, remoteConn)
}
