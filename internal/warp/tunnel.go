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
	hand *sshHandler
	tun  *tun.Tunnel
}

type sshHandler struct {
	client *ssh.Client
	dns    *dns.DNS
	meter  *tarification.DataMeter
}

func NewService(tunName string, tunAddr string) (*Service, error) {
	hand := &sshHandler{}

	tn, err := tun.CreateTUN(tunName, tunAddr, tun.DefaultMTU, hand)
	if err != nil {
		return nil, err
	}

	log.Info().Str("name", tunName).Msg("WRP", "register")

	return &Service{
		tun:  tn,
		hand: hand,
	}, nil
}

func (s *Service) GetTUN() *tun.Tunnel {
	return s.tun
}

func (s *Service) TunStart(ctx context.Context, client *ssh.Client, meter *tarification.DataMeter, dns *dns.DNS) error {
	defer log.Info().Str("name", s.tun.GetName()).Msg("WRP", "unregister")
	defer s.tun.Close()

	s.hand.client = client
	s.hand.dns = dns
	s.hand.meter = meter

	if err := s.hand.dns.Apply(s.tun.GetAddr()); err != nil {
		return err
	}
	defer s.hand.dns.Restore()

	<-ctx.Done()
	return nil
}

func (h *sshHandler) HandleTCP(conn tun.TCPConn) {
	log.Info().Str("dest", conn.LocalAddr().String()).Msg("TUN", "handle conn")

	remoteConn, err := h.client.Dial(conn.LocalAddr().Network(), conn.LocalAddr().String())
	if err != nil {
		log.Error().Err(err).Msg("TUN", "failed to connect to remote host")
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

func (h *sshHandler) HandleUDP(conn tun.UDPConn) {
	if strings.HasSuffix(conn.LocalAddr().String(), ":53") {
		h.dns.Handle(conn)
	}
}
