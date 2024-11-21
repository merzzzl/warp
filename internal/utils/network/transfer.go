package network

import (
	"errors"
	"io"
	"net"
	"sort"
	"sync"
	"time"

	"github.com/merzzzl/warp/internal/utils/log"
)

type Pipe struct {
	addr1  net.Addr
	addr2  net.Addr
	openAt time.Time
	tag    string
}

var openPipes = make(map[Pipe]struct{}, 100)

var mutex sync.RWMutex

func Transfer(tag string, conn1, conn2 net.Conn) {
	var wg sync.WaitGroup

	wg.Add(2)

	end := open(tag, conn1.LocalAddr(), conn1.RemoteAddr())
	defer end()

	go func() {
		defer wg.Done()
		defer conn2.Close()

		_, err := io.Copy(conn1, conn2)
		if err != nil && !errors.Is(err, io.EOF) {
			if _, ok := err.(net.Error); !ok {
				log.Warn().Err(err).Msg(tag, "failed to read data")
			}
		}
	}()

	go func() {
		defer wg.Done()
		defer conn1.Close()

		_, err := io.Copy(conn2, conn1)
		if err != nil && !errors.Is(err, io.EOF) {
			if _, ok := err.(net.Error); !ok {
				log.Warn().Err(err).Msg(tag, "failed to write data")
			}
		}
	}()

	wg.Wait()
}

func open(tag string, addr1, addr2 net.Addr) func() {
	mutex.Lock()
	defer mutex.Unlock()

	p := Pipe{
		tag:    tag,
		addr1:  addr1,
		addr2:  addr2,
		openAt: time.Now(),
	}

	openPipes[p] = struct{}{}

	return func() {
		mutex.Lock()
		defer mutex.Unlock()

		delete(openPipes, p)
	}
}

func List() []Pipe {
	mutex.RLock()
	defer mutex.RUnlock()

	list := make([]Pipe, 0, len(openPipes))

	for k := range openPipes {
		list = append(list, k)
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].openAt.Before(list[j].openAt)
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
