package network

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/merzzzl/warp/internal/utils/log"
)

type Pipe struct {
	tag      string
	protocol atomic.Uint32
	addr1    net.Addr
	addr2    net.Addr
	openAt   time.Time
}

type PipeGroup struct {
	Tag       string
	Protocol  string
	Dest      net.Addr
	OpenAt    time.Time
	OpenCount int
}

var openPipes = sync.Map{}

func Transfer(tag string, conn1, conn2 net.Conn) {
	var wg sync.WaitGroup

	wg.Add(2)

	pipe, end := open(tag, conn1.LocalAddr(), conn1.RemoteAddr())
	defer end()

	go func() {
		defer wg.Done()

		err := universalCopy(&pipe.protocol, conn1, conn2)
		if err != nil {
			if _, ok := err.(net.Error); !ok {
				log.Warn().Err(err).Msg(tag, "failed to read data")
			}
		}
		_ = conn2.Close()
	}()

	go func() {
		defer wg.Done()

		err := universalCopy(nil, conn2, conn1)
		if err != nil {
			if _, ok := err.(net.Error); !ok {
				log.Warn().Err(err).Msg(tag, "failed to write data")
			}
		}
		_ = conn1.Close()
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

func List() []*PipeGroup {
	list := make(map[string]*PipeGroup)

	openPipes.Range(func(k, _ any) bool {
		p, ok := k.(*Pipe)
		if !ok {
			return true
		}

		key := fmt.Sprintf("%s:%s", p.addr1.String(), p.tag)
		pgr := &PipeGroup{}

		if v, ok := list[key]; ok {
			pgr = v
		} else {
			pgr.Tag = p.tag
			pgr.Dest = p.addr1
			pgr.OpenCount = 1
			pgr.Protocol = protocols[p.protocol.Load()]
		}

		pgr.OpenCount++

		if p.openAt.Before(pgr.OpenAt) {
			pgr.OpenAt = p.openAt
		}

		list[key] = pgr

		return true
	})

	groups := make([]*PipeGroup, 0, len(list))
	for _, v := range list {
		groups = append(groups, v)
	}

	return groups
}

func universalCopy(proto *atomic.Uint32, conn1, conn2 net.Conn) error {
	buf := make([]byte, 32*1024)
	protoDetected := false

	if proto == nil {
		protoDetected = true
	}

	for {
		n, err := conn1.Read(buf)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}

			return err
		}

		if !protoDetected {
			go func() {
				protoDetected = true
				id := detectProtocol(buf)
				proto.Store(id)
			}()
		}

		_, writeErr := conn2.Write(buf[:n])
		if writeErr != nil {
			return writeErr
		}
	}
}
