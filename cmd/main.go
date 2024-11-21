package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/merzzzl/warp/internal/protocol/local"
	"github.com/merzzzl/warp/internal/protocol/ssh"
	"github.com/merzzzl/warp/internal/protocol/wg"
	"github.com/merzzzl/warp/internal/service"
	"github.com/merzzzl/warp/internal/utils/log"
	"github.com/merzzzl/warp/internal/utils/tui"
)

func main() {
	runtime.GOMAXPROCS(1)

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
	localP := local.New(&local.Config{})

	// INFO: Add more protocols here
	// protocol must implement:
	//  required: LookupHost(ctx context.Context, req *dns.Msg) (*dns.Msg, error)
	//  optional: FixedIPs() []string
	//  optional: HandleTCP(conn net.Conn)
	//  optional: HandleUDP(conn net.Conn)
	for _, pConfig := range cfg.Protocols {
		// Register Local
		if pConfig.Local != nil {
			localP = local.New(pConfig.Local)

			continue
		}

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
	}

	group = append(group, localP)

	if err := srv.ListenAndServe(ctx, group); err != nil {
		log.Fatal().Err(err).Msg("APP", "failed to run service")
	}
}
