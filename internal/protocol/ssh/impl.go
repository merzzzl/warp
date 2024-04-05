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
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Host     string `yaml:"host"`
	Domain   string `yaml:"domain"`
}

type Protocol struct {
	cli    *ssh.Client
	domain *regexp.Regexp
	dns    string
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

	session, err := cli.NewSession()
	if err != nil {
		return nil, err
	}

	sshDNS, err := session.CombinedOutput("scutil --dns | grep \"nameserver\\[.\\]\" | awk '{print $3}' | head -n 1")
	if err != nil {
		return nil, err
	}

	var dnsIP string
	if len(sshDNS) != 0 {
		dnsIP = strings.TrimSpace(string(sshDNS))
	}

	rx, err := regexp.Compile(cfg.Domain)
	if err != nil {
		return nil, err
	}

	return &Protocol{
		cli:    cli,
		domain: rx,
		dns:    dnsIP,
	}, nil
}

func (p *Protocol) LookupHost(_ context.Context, req *dns.Msg) (*dns.Msg, error) {
	for _, que := range req.Question {
		if !p.domain.MatchString(que.Name[:len(que.Name)-1]) || p.dns == "" {
			return req, nil
		}
	}

	dnsConn, err := p.cli.Dial("tcp", p.dns+":53")
	if err != nil {
		return nil, err
	}

	defer dnsConn.Close()

	co := new(dns.Conn)
	co.Conn = dnsConn

	err = co.WriteMsg(req)
	if err != nil {
		return nil, err
	}

	rsp, err := co.ReadMsg()
	if err != nil {
		return nil, err
	}

	return rsp, nil
}

func (p *Protocol) HandleTCP(conn net.Conn) {
	log.Info().Str("dest", conn.LocalAddr().String()).Str("type", "TCP").Msg("SSH", "handle conn")

	remoteConn, err := p.cli.Dial(conn.LocalAddr().Network(), conn.LocalAddr().String())
	if err != nil {
		log.Error().Err(err).Msg("SSH", "failed to connect to remote host")

		return
	}

	network.Transfer("SSH", conn, remoteConn)
}

func (*Protocol) HandleUDP(conn net.Conn) {
	log.Info().Str("dest", conn.LocalAddr().String()).Str("type", "UDP").Msg("SSH", "handle conn")

	log.Error().Msg("SSH", "udp unsupported")
}
