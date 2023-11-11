package main

import (
	"flag"
	"fmt"
	"net"
	"strings"
	"syscall"

	"github.com/merzzzl/warp/internal/log"
	"github.com/merzzzl/warp/internal/tui"
	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

type Config struct {
	tunelIP       string
	tunelName     string
	dnsDomain     string
	ssh           *ssh.ClientConfig
	sshHost       string
	tui           *tui.Config
	kubeConfig    string
	kubeNamespace string
	localNet      net.IP
}

func loadConfig() *Config {
	var sshs string
	var tun string
	var ip string
	var domain string
	var termui bool
	var namespace string
	var config string
	var lo0 string

	flag.StringVar(&sshs, "s", "root@127.0.0.1", "ssh host")
	flag.StringVar(&tun, "t", "utun5", "utun name")
	flag.StringVar(&ip, "i", "192.168.48.1", "utun name")
	flag.StringVar(&domain, "d", ".", "cdomain sufix")
	flag.StringVar(&namespace, "n", "default", "kube namespace")
	flag.StringVar(&config, "k", "", "path to kube config")
	flag.StringVar(&lo0, "l", "127.0.40.0", "ip for local network in 24 mask")
	flag.BoolVar(&termui, "u", false, "enable tui mode")

	flag.Parse()

	sshVars := strings.Split(sshs, "@")
	if len(sshVars) != 2 {
		log.Fatal().Msg("APP", "incorrect ssh")
	}

	user := sshVars[0]
	host := sshVars[1]

	sshConfig := ssh.ClientConfig{
		User: user,
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

	tuiCfg := tui.Config{
		SSH:    sshs,
		Tunnel: tun,
		IP:     ip,
		Domain: domain,
		K8S:    namespace,
	}

	cfg := &Config{}

	lo0ip := net.ParseIP(lo0)

	cfg.ssh = &sshConfig
	cfg.sshHost = host
	cfg.dnsDomain = domain
	cfg.tunelName = tun
	cfg.tunelIP = ip
	cfg.kubeConfig = config
	cfg.kubeNamespace = namespace
	cfg.localNet = lo0ip

	if termui {
		cfg.tui = &tuiCfg
	}

	return cfg
}
