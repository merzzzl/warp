package ssh

import (
	"io"
	"sync"

	"github.com/merzzzl/warp/internal/log"
	"github.com/merzzzl/warp/internal/sys/tun"
	"github.com/merzzzl/warp/internal/tarification"
	"golang.org/x/crypto/ssh"
)

type handler struct {
	client *ssh.Client
}

func (h *handler) HandleTCP(conn tun.TCPConn) {
	log.Info().Str("dest", conn.LocalAddr().String()).Str("type", "TCP").Msg("SSH", "handle conn")

	remoteConn, err := h.client.Dial(conn.LocalAddr().Network(), conn.LocalAddr().String())
	if err != nil {
		log.Error().Err(err).Msg("SSH", "failed to connect to remote host")
		return
	}

	localConn := tarification.NewTarificationConn(conn)

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
	log.Info().Str("dest", conn.LocalAddr().String()).Str("type", "UDP").Msg("SSH", "handle conn")
}
