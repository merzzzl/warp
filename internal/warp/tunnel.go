package warp

import (
	"context"
	"io"
	"strings"
	"sync"

	"github.com/merzzzl/warp/internal/dns"
	"github.com/merzzzl/warp/internal/log"
	"github.com/merzzzl/warp/internal/tarification"
	"github.com/merzzzl/warp/internal/tun"
	"golang.org/x/crypto/ssh"
)

type Service struct {
	hand *handler
	tun  *tun.Tunnel
}

type handler struct {
	client *ssh.Client
	dns    *dns.DNS
	meter  *tarification.DataMeter
}

func NewService(tunName string, tunAddr string) (*Service, error) {
	h := &handler{}

	tn, err := tun.CreateTUN(tunName, tunAddr, tun.DefaultMTU, h)
	if err != nil {
		return nil, err
	}

	log.Info().Str("name", tunName).Msg("WRP", "register")

	return &Service{
		tun:  tn,
		hand: h,
	}, nil
}

func (s *Service) GetTUN() *tun.Tunnel {
	return s.tun
}

func (s *Service) Start(ctx context.Context, client *ssh.Client, meter *tarification.DataMeter, ns *dns.DNS) error {
	defer log.Info().Str("name", s.tun.GetName()).Msg("WRP", "unregister")
	defer s.tun.Close()

	s.hand.client = client
	s.hand.dns = ns
	s.hand.meter = meter

	if err := ns.ApplySSH(s.tun.GetAddr()); err != nil {
		return err
	}

	if err := ns.ApplyK8S(ctx); err != nil {
		return err
	}

	<-ctx.Done()

	return nil
}

func (h *handler) HandleTCP(conn tun.TCPConn) {
	log.Info().Str("dest", conn.LocalAddr().String()).Str("type", "TCP").Msg("WRP", "handle conn")

	remoteConn, err := h.client.Dial(conn.LocalAddr().Network(), conn.LocalAddr().String())
	if err != nil {
		log.Error().Err(err).Msg("WRP", "failed to connect to remote host")
		return
	}

	localConn := h.meter.TarificationConn(conn)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		_, err := io.Copy(remoteConn, localConn)
		if err != nil {
			return
		}
		remoteConn.Close()
		wg.Done()
	}()

	go func() {
		_, err := io.Copy(localConn, remoteConn)
		if err != nil {
			return
		}
		localConn.Close()
		wg.Done()
	}()

	wg.Wait()
}

func (h *handler) HandleUDP(conn tun.UDPConn) {
	if strings.HasSuffix(conn.LocalAddr().String(), ":53") {
		h.dns.Handle(conn)

		return
	}

	log.Info().Str("dest", conn.LocalAddr().String()).Str("type", "UDP").Msg("WRP", "handle conn")
	log.Error().Str("dest", conn.LocalAddr().String()).Str("type", "UDP").Msg("WRP", "not supported")
}
