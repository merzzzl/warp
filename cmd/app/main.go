package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/merzzzl/warp/internal/dns"
	"github.com/merzzzl/warp/internal/kube"
	"github.com/merzzzl/warp/internal/log"
	"github.com/merzzzl/warp/internal/routes"
	"github.com/merzzzl/warp/internal/tarification"
	"github.com/merzzzl/warp/internal/tui"
	"github.com/merzzzl/warp/internal/warp"
	"golang.org/x/crypto/ssh"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	runtime.GOMAXPROCS(2)

	ctx, cancel := context.WithCancel(context.Background())
	cfg := loadConfig()

	sshClient, err := ssh.Dial("tcp", cfg.sshHost+":22", cfg.ssh)
	if err != nil {
		log.Fatal().Err(err).Msg("APP", "failed to connect to the SSH server")
	}

	wrp, err := warp.NewService(cfg.tunelName, cfg.tunelIP)
	if err != nil {
		log.Fatal().Err(err).Msg("APP", "failed to create WARP")
	}

	var k8sClient *kube.KubeRoute
	if cfg.kubeConfig != "" {
		restcfg, err := clientcmd.BuildConfigFromFlags("", cfg.kubeConfig)
		if err != nil {
			log.Fatal().Err(err).Msg("APP", "failed to load kube config")
		}

		clientset, err := kubernetes.NewForConfig(restcfg)
		if err != nil {
			log.Fatal().Err(err).Msg("APP", "failed to create kube client")
		}

		k8sClient = kube.NewKubeRoute(restcfg, clientset, cfg.kubeNamespace, cfg.localNet)
	}

	routes := routes.NewRoutes(wrp.GetTUN(), k8sClient)
	meter := tarification.NewDataMeter()
	ns := dns.NewDNS(sshClient, k8sClient, routes, cfg.dnsDomain)

	go meter.Run(ctx)

	go func() {
		defer cancel()
		if cfg.tui != nil {
			ui := tui.NewTUI(meter, routes, cfg.tui)
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

	defer func() {
		_ = ns.Restore()
	}()

	if err := wrp.Start(ctx, sshClient, meter, ns); err != nil {
		log.Fatal().Err(err).Msg("APP", "failed to run Tunnel service")
	}
}
