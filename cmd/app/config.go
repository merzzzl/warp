package main

import (
	"flag"
	"fmt"
	"strings"
	"syscall"

	"github.com/merzzzl/warp/internal/log"
	"github.com/merzzzl/warp/internal/tui"
	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

type Config struct {
	tunelIP   string
	tunelName string
	dnsDomain string
	ssh       *ssh.ClientConfig
	sshHost   string
	sshString string
	tui       *tui.Config
}

func loadConfig() *Config {
	var sshs string
	var tun string
	var ip string
	var domain string
	var termui bool

	flag.StringVar(&sshs, "ssh", "root@127.0.0.1", "ssh host")
	flag.StringVar(&tun, "tun", "utun5", "utun name")
	flag.StringVar(&ip, "ip", "192.168.48.1", "utun name")
	flag.StringVar(&domain, "domain", ".", "cdomain sufix")
	flag.BoolVar(&termui, "tui", false, "enable tui mode")

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
	}

	cfg := &Config{}

	cfg.ssh = &sshConfig
	cfg.sshHost = host
	cfg.sshString = sshs
	cfg.dnsDomain = domain
	cfg.tunelIP = ip
	cfg.tunelName = tun

	if termui {
		cfg.tui = &tuiCfg
	}

	return cfg
}
