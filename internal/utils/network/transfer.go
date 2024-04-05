package network

import (
	"errors"
	"io"
	"net"
	"sync"

	"github.com/merzzzl/warp/internal/utils/log"
)

func transferData(tag string, src, dst net.Conn, wg *sync.WaitGroup) {
	defer wg.Done()

	buf := make([]byte, 4096)

	for {
		n, err := src.Read(buf)
		if err != nil {
			if _, ok := err.(net.Error); !ok && !errors.Is(err, io.EOF) {
				log.Error().Err(err).Msg(tag, "failed to read data")
			}

			break
		}

		_, err = dst.Write(buf[:n])
		if err != nil {
			if _, ok := err.(net.Error); !ok {
				log.Error().Err(err).Msg(tag, "failed to write data")
			}

			break
		}
	}
}

func Transfer(tag string, conn1, conn2 net.Conn) {
	var wg sync.WaitGroup

	wg.Add(2)

	go transferData(tag, conn1, conn2, &wg)
	go transferData(tag, conn2, conn1, &wg)

	wg.Wait()

	if err := conn1.Close(); err != nil {
		log.Error().Err(err).Msg(tag, "failed to close conn")
	}

	if err := conn2.Close(); err != nil {
		log.Error().Err(err).Msg(tag, "failed to close conn")
	}
}
