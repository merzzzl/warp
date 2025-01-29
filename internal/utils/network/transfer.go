package network

import (
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/merzzzl/warp/internal/utils/log"
)

type Pipe struct {
	tag    string
	addr1  net.Addr
	addr2  net.Addr
	openAt time.Time
	rx     atomic.Uint32
	tx     atomic.Uint32
}

var openPipes = sync.Map{}

func Transfer(tag string, conn1, conn2 net.Conn) {
	if tcpConn1, ok := conn1.(*net.TCPConn); ok {
		if err := tcpConn1.SetKeepAlive(true); err != nil {
			log.Warn().Err(err).Msg(tag, "failed to enable keep-alive")
		}

		if err := tcpConn1.SetKeepAlivePeriod(60 * time.Second); err != nil {
			log.Warn().Err(err).Msg(tag, "failed to set keep-alive period")
		}
	}

	if tcpConn2, ok := conn2.(*net.TCPConn); ok {
		if err := tcpConn2.SetKeepAlive(true); err != nil {
			log.Warn().Err(err).Msg(tag, "failed to enable keep-alive")
		}

		if err := tcpConn2.SetKeepAlivePeriod(60 * time.Second); err != nil {
			log.Warn().Err(err).Msg(tag, "failed to set keep-alive period")
		}
	}

	var wg sync.WaitGroup

	wg.Add(2)

	pipe, end := open(tag, conn1.LocalAddr(), conn1.RemoteAddr())

	defer end()
	defer conn2.Close()
	defer conn1.Close()

	go func() {
		defer wg.Done()

		err := universalCopy(&pipe.tx, conn1, conn2)
		if err != nil {
			if _, ok := err.(net.Error); !ok {
				log.Warn().Err(err).Msg(tag, "failed to read data")
			}
		}
	}()

	go func() {
		defer wg.Done()

		err := universalCopy(&pipe.rx, conn2, conn1)
		if err != nil {
			if _, ok := err.(net.Error); !ok {
				log.Warn().Err(err).Msg(tag, "failed to write data")
			}
		}
	}()

	wg.Wait()
}

func open(tag string, addr1, addr2 net.Addr) (*Pipe, func()) {
	p := Pipe{
		tag:    tag,
		addr1:  addr1,
		addr2:  addr2,
		openAt: time.Now(),
	}

	openPipes.Store(&p, struct{}{})

	return &p, func() {
		openPipes.Delete(&p)
	}
}

func List() []*Pipe {
	list := make([]*Pipe, 0)

	openPipes.Range(func(k, _ any) bool {
		p, ok := k.(*Pipe)
		if !ok {
			return true
		}

		list = append(list, p)

		return true
	})

	return list
}

func (p *Pipe) Network() string {
	return p.addr1.Network()
}

func (p *Pipe) Tag() string {
	return p.tag
}

func (p *Pipe) From() string {
	return p.addr2.String()
}

func (p *Pipe) To() string {
	return p.addr1.String()
}

func (p *Pipe) OpenAt() time.Time {
	return p.openAt
}

func (p *Pipe) TxRx() (uint32, uint32) {
	return p.tx.Load(), p.rx.Load()
}

func universalCopy(txrx *atomic.Uint32, conn1, conn2 net.Conn) error {
	buf := make([]byte, 32*1024)

	for {
		n, err := conn1.Read(buf)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}

			return err
		}

		_, writeErr := conn2.Write(buf[:n])
		if writeErr != nil {
			return writeErr
		}

		txrx.Add(1)
	}
}
