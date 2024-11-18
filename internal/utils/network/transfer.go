package network

import (
	"errors"
	"io"
	"net"
	"sync"

	"github.com/merzzzl/warp/internal/utils/log"
)

func Transfer(tag string, conn1, conn2 net.Conn) {
	var wg sync.WaitGroup

	wg.Add(2)

	go func() {
		defer wg.Done()
		defer conn2.Close()

		_, err := io.Copy(conn1, conn2)
		if err != nil && !errors.Is(err, io.EOF) {
			if _, ok := err.(net.Error); !ok {
				log.Error().Err(err).Msg(tag, "failed to read data")
			}
		}
	}()

	go func() {
		defer wg.Done()
		defer conn1.Close()

		_, err := io.Copy(conn2, conn1)
		if err != nil && !errors.Is(err, io.EOF) {
			if _, ok := err.(net.Error); !ok {
				log.Error().Err(err).Msg(tag, "failed to write data")
			}
		}
	}()

	wg.Wait()
}
