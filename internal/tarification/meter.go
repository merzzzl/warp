package tarification

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type dataMeter struct {
	// 1000 500 200 100 50 20 10 5 1 0.1
	mutex          sync.RWMutex
	transferredIn  atomic.Int64
	transferredOut atomic.Int64
	outRate        float64
	inRate         float64
}

var m = newDataMeter()

type TarificationConn struct {
	dm   *dataMeter
	conn net.Conn
}

func newDataMeter() *dataMeter {
	return new(dataMeter)
}

func GetRates() (float64, float64) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return m.inRate, m.outRate
}

func Run(ctx context.Context) {
	for ctx.Err() == nil {
		time.Sleep(1 * time.Second)

		in := m.transferredIn.Swap(0)
		out := m.transferredOut.Swap(0)

		m.mutex.Lock()
		m.inRate = float64(in * 8)
		m.outRate = float64(out * 8)
		m.mutex.Unlock()
	}
}

func NewTarificationConn(conn net.Conn) *TarificationConn {
	return &TarificationConn{
		dm:   m,
		conn: conn,
	}
}

func (t *TarificationConn) Read(p []byte) (n int, err error) {
	s, err := t.conn.Read(p)
	t.dm.transferredIn.Add(int64(s))
	return s, err
}

func (t *TarificationConn) Write(p []byte) (n int, err error) {
	s, err := t.conn.Write(p)
	t.dm.transferredOut.Add(int64(s))
	return s, err
}

func (t *TarificationConn) Close() error {
	return t.conn.Close()
}
