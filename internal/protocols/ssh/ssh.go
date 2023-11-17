package ssh

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"syscall"

	"github.com/merzzzl/warp/internal/routes"
	"github.com/merzzzl/warp/internal/sys/tun"
	"github.com/miekg/dns"
	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

type Config struct {
	SSHUser string
	SSHHost string
	Domain  string
	TunName string
	TunAddr string
}

type SSHRoute struct {
	ctx    context.Context
	cli    *ssh.Client
	domain string
	dns    string
	ips    map[string]net.IP
	mutex  sync.Mutex
	tun    *tun.Tunnel
}

func NewSSHRoute(ctx context.Context, cfg *Config) (*SSHRoute, error) {
	sshConfig := &ssh.ClientConfig{
		User: cfg.SSHUser,
		Auth: []ssh.AuthMethod{
			ssh.PasswordCallback(func() (secret string, err error) {
				fmt.Print("SSH Password:")
				passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
				if err != nil {
					return "", err
				}
				fmt.Print("\r")
				return string(passwordBytes), nil
			}),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	cli, err := ssh.Dial("tcp", cfg.SSHHost+":22", sshConfig)
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

	hand := &handler{client: cli}

	t, err := tun.CreateTUN(cfg.TunName, cfg.TunAddr, hand)
	if err != nil {
		return nil, err
	}

	go func() {
		<-ctx.Done()
		t.Close()
	}()

	return &SSHRoute{
		cli:    cli,
		ctx:    ctx,
		domain: cfg.Domain,
		dns:    dnsIP,
		ips:    make(map[string]net.IP),
		tun:    t,
	}, nil
}

func (s *SSHRoute) GetDNS(host string) (net.IP, bool, error) {
	if !strings.HasSuffix(host, s.domain) || s.dns == "" {
		return nil, false, nil
	}

	s.mutex.Lock()
	ip, ok := s.ips[host]
	s.mutex.Unlock()

	if ok {
		return ip, true, nil
	}

	c := new(dns.Client)
	c.Net = "tcp"

	req := new(dns.Msg)
	req.SetQuestion(host, dns.TypeA)

	dnsConn, err := s.cli.Dial("tcp", s.dns+":53")
	if err != nil {
		return nil, true, err
	}
	defer dnsConn.Close()

	co := new(dns.Conn)
	co.Conn = dnsConn

	err = co.WriteMsg(req)
	if err != nil {
		return nil, true, err
	}

	rsp, err := co.ReadMsg()
	if err != nil {
		return nil, true, err
	}

	remoteIP := rsp.Answer[0].(*dns.A).A

	err = routes.ApplyRoute(remoteIP.String(), s.tun.GetAddr())
	if err != nil {
		return nil, true, err
	}

	s.mutex.Lock()
	s.ips[host] = remoteIP
	s.mutex.Unlock()

	return remoteIP, true, nil
}
