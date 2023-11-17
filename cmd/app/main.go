package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/merzzzl/warp/internal/dns"
	"github.com/merzzzl/warp/internal/log"
	"github.com/merzzzl/warp/internal/protocols/kube"
	"github.com/merzzzl/warp/internal/protocols/ssh"
	"github.com/merzzzl/warp/internal/routes"
	"github.com/merzzzl/warp/internal/sys/resolv"
	"github.com/merzzzl/warp/internal/tarification"
	"github.com/merzzzl/warp/internal/tui"
)

func main() {
	runtime.GOMAXPROCS(2)

	ctx, cancel := context.WithCancel(context.Background())
	cfg := loadConfig()

	routes.SetSubnet(net.ParseIP(cfg.localNet))
	defer func() {
		if err := routes.Free(); err != nil {
			log.Error().Err(err).Msg("APP", "failed to free host")
		}
		log.Info().Msg("APP", "hosts cleaned")
	}()

	dnsG := []dns.DNSGetter{}

	if cfg.sshHost != "" {
		sshR, err := ssh.NewSSHRoute(ctx, &ssh.Config{
			SSHUser: cfg.sshUser,
			SSHHost: cfg.sshHost,
			Domain:  cfg.dnsDomain,
			TunName: cfg.tunName,
			TunAddr: cfg.tunAddr,
		})
		if err != nil {
			log.Fatal().Err(err).Msg("APP", "failed to create SSH route")
		}

		dnsG = append(dnsG, sshR.GetDNS)
	}

	if cfg.kubeConfig != "" {
		k8sR, err := kube.NewKubeRoute(ctx, &kube.Config{
			KubeConfigPath: cfg.kubeConfig,
			KubeNamespace:  cfg.kubeNamespace,
		})
		if err != nil {
			log.Fatal().Err(err).Msg("APP", "failed to create Kube route")
		}

		dnsG = append(dnsG, k8sR.GetDNS)
	}

	defer func() {
		if err := resolv.Restore(); err != nil {
			log.Error().Err(err).Msg("APP", "failed to restore resolv")
		}
		log.Info().Msg("APP", "dns restored")
	}()

	ns, err := dns.NewDNS(dnsG...)
	if err != nil {
		log.Fatal().Err(err).Msg("APP", "failed to create DNS")
	}

	go tarification.Run(ctx)

	go func() {
		defer cancel()
		if cfg.tui != nil {
			ui := tui.NewTUI(cfg.tui)
			if err := ui.CreateTUI(); err != nil {
				log.Error().Err(err).Msg("APP", "failed on create tui")
			}
		} else {
			c := make(chan os.Signal, 1)
			signal.Notify(c, os.Interrupt, syscall.SIGTERM)
			<-c
			fmt.Print("\r")
		}
	}()

	if err := ns.Start(ctx); err != nil {
		log.Error().Err(err).Msg("APP", "failed to start DNS")
	}
}
