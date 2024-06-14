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
	runtime.GOMAXPROCS(2)

	ctx, cancel := context.WithCancel(context.Background())

	cfg, err := loadConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("APP", "failed on load config")
	}

	srv, err := service.New(cfg.Tunnel)
	if err != nil {
		log.Fatal().Err(err).Msg("APP", "failed create tunnel")
	}

	if !cfg.verbose {
		go func() {
			defer cancel()

			if err := tui.CreateTUI(srv.GetRoutes(), srv.GetTraffic()); err != nil {
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

	// Register SSH
	if cfg.SSH != nil {
		sshR, err := ssh.New(cfg.SSH)
		if err != nil {
			log.Fatal().Err(err).Msg("APP", "failed to create SSH route")
		}

		group = append(group, sshR)
	}

	// Register WireGuard
	if cfg.WireGuard != nil {
		cbR, err := wg.New(ctx, cfg.WireGuard)
		if err != nil {
			log.Fatal().Err(err).Msg("APP", "failed to create WireGuard route")
		}

		group = append(group, cbR)
	}

	// INFO: Add more protocols here
	// protocol must implement:
	//   LookupHost(ctx context.Context, req *dns.Msg) (*dns.Msg, error)
	//   HandleTCP(conn net.Conn)
	//   HandleUDP(conn net.Conn)

	group = append(group, local.New())

	if err := srv.ListenAndServe(ctx, group); err != nil {
		log.Fatal().Err(err).Msg("APP", "failed to run service")
	}
}
