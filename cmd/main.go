package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/merzzzl/warp/internal/protocol/socks5"
	"github.com/merzzzl/warp/internal/protocol/ssh"
	"github.com/merzzzl/warp/internal/protocol/wg"
	"github.com/merzzzl/warp/internal/service"
	"github.com/merzzzl/warp/internal/utils/log"
	"github.com/merzzzl/warp/internal/utils/tui"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	cfg, err := loadConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("APP", "failed on load config")
	}

	if cfg.debug {
		log.EnableDebug()
	}

	srv, err := service.New(cfg.Tunnel)
	if err != nil {
		log.Fatal().Err(err).Msg("APP", "failed create tunnel")
	}

	if !cfg.verbose {
		go func() {
			defer cancel()

			if err := tui.CreateTUI(srv.GetRoutes(), srv.GetTraffic(), cfg.fun); err != nil {
				log.Error().Err(err).Msg("APP", "failed on create tui")
			}
		}()
	} else {
		go func() {
			defer cancel()

			c := make(chan os.Signal, 1)
			signal.Notify(c, os.Interrupt, syscall.SIGTERM)
			<-c

			if _, err := fmt.Print("\n"); err != nil {
				return
			}
		}()
	}

	group := []service.Protocol{}

	// INFO: Add more protocols here
	// protocol must implement:
	//
	//  DNS resolver (optional):
	//  required: LookupHost(ctx context.Context, req *dns.Msg) (*dns.Msg, error)
	//  required: Domain() string
	//
	//  Additional methods:
	//  optional: FixedIPs() []string
	//  optional: HandleTCP(conn net.Conn)
	//  optional: HandleUDP(conn net.Conn)

	for _, pConfig := range cfg.Protocols {
		// Register SSH
		if pConfig.SSH != nil {
			sshR, err := ssh.New(pConfig.SSH)
			if err != nil {
				log.Fatal().Err(err).Msg("APP", "failed to create SSH route")
			}

			group = append(group, sshR)

			continue
		}

		// Register WireGuard
		if pConfig.WireGuard != nil {
			cbR, err := wg.New(ctx, pConfig.WireGuard)
			if err != nil {
				log.Fatal().Err(err).Msg("APP", "failed to create WireGuard route")
			}

			group = append(group, cbR)

			continue
		}

		// Register SOCKS5
		if pConfig.SOCKS5 != nil {
			socks5R, err := socks5.New(pConfig.SOCKS5)
			if err != nil {
				log.Fatal().Err(err).Msg("APP", "failed to create SOCKS5 route")
			}
			group = append(group, socks5R)

			continue
		}
	}

	if err := srv.ListenAndServe(ctx, group, cfg.ipv6); err != nil {
		log.Fatal().Err(err).Msg("APP", "failed to run service")
	}
}
