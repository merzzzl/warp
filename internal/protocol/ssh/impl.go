package ssh

import (
	"context"
	"io"
	"net"
	"regexp"
	"strings"
	"sync"

	"github.com/miekg/dns"
	"golang.org/x/crypto/ssh"

	"github.com/merzzzl/warp/internal/utils/log"
)

type Config struct {
	User     string
	Password string
	Host     string
	Domain   string
}

type Protocol struct {
	ips    map[string]any
	cli    *ssh.Client
	domain *regexp.Regexp
	dns    string
	mutex  sync.Mutex
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
		ips:    make(map[string]any, 0),
	}, nil
}

func (p *Protocol) LookupHost(_ context.Context, req *dns.Msg) (*dns.Msg, error) {
	for _, que := range req.Question {
		if !p.domain.MatchString(que.Name) || p.dns == "" {
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

	p.mutex.Lock()

	for _, ans := range rsp.Answer {
		if a, ok := ans.(*dns.A); ok {
			p.ips[a.A.String()] = struct{}{}
		}
	}

	p.mutex.Unlock()

	return rsp, nil
}

func (p *Protocol) HandleTCP(conn net.Conn) {
	log.Info().Str("dest", conn.LocalAddr().String()).Str("type", "TCP").Msg("SSH", "handle conn")

	remoteConn, err := p.cli.Dial(conn.LocalAddr().Network(), conn.LocalAddr().String())
	if err != nil {
		log.Error().Err(err).Msg("SSH", "failed to connect to remote host")

		return
	}

	var wg sync.WaitGroup

	wg.Add(2)

	go func() {
		_, err := io.Copy(remoteConn, conn)
		if err != nil {
			log.Error().Err(err).Msg("SSH", "failed to transfer data")
		}

		if err := remoteConn.Close(); err != nil {
			log.Error().Err(err).Msg("SSH", "failed to close conn")
		}

		wg.Done()
	}()

	go func() {
		_, err := io.Copy(conn, remoteConn)
		if err != nil {
			log.Error().Err(err).Msg("SSH", "failed to transfer data")
		}

		if err := conn.Close(); err != nil {
			log.Error().Err(err).Msg("SSH", "failed to close conn")
		}

		wg.Done()
	}()

	wg.Wait()
}

func (*Protocol) HandleUDP(conn net.Conn) {
	log.Info().Str("dest", conn.LocalAddr().String()).Str("type", "UDP").Msg("SSH", "handle conn")
}
