package main

import (
	"flag"
	"strings"

	"github.com/merzzzl/warp/internal/log"
	"github.com/merzzzl/warp/internal/tui"
)

type Config struct {
	dnsDomain     string
	sshUser       string
	sshHost       string
	tui           *tui.Config
	kubeConfig    string
	kubeNamespace string
	localNet      string
	tunName       string
	tunAddr       string
}

func loadConfig() *Config {
	var sshs string
	var domain string
	var termui bool
	var namespace string
	var config string
	var lo0 string
	var tun string
	var tunAddr string

	flag.StringVar(&sshs, "s", "", "ssh host")
	flag.StringVar(&domain, "d", ".", "cdomain sufix")
	flag.StringVar(&namespace, "n", "default", "kube namespace")
	flag.StringVar(&config, "k", "", "path to kube config")
	flag.StringVar(&lo0, "l", "127.192.168.0", "ip for local network in 24 mask")
	flag.StringVar(&tun, "t", "utun7", "tun interface name")
	flag.StringVar(&tunAddr, "a", "192.168.127.0", "tun interface address")
	flag.BoolVar(&termui, "u", false, "enable tui mode")

	flag.Parse()

	cfg := &Config{}

	if sshs != "" {
		sshVars := strings.Split(sshs, "@")
		if len(sshVars) != 2 {
			log.Fatal().Msg("APP", "incorrect ssh")
		}

		cfg.sshUser = sshVars[0]
		cfg.sshHost = sshVars[1]
	}

	tuiCfg := tui.Config{
		SSH:     sshs,
		Domain:  domain,
		TunIP:   tunAddr,
		TunName: tun,

		K8SEnable: config != "",
		K8S:       namespace,
		LocalIP:   lo0,
	}

	cfg.dnsDomain = domain
	cfg.kubeConfig = config
	cfg.kubeNamespace = namespace
	cfg.localNet = lo0
	cfg.tunName = tun
	cfg.tunAddr = tunAddr

	if termui {
		cfg.tui = &tuiCfg
	}

	return cfg
}
