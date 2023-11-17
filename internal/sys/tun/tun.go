package tun

import (
	"github.com/merzzzl/warp/internal/sys/iface"
	"github.com/xjasonlyu/tun2socks/v2/core"
	"github.com/xjasonlyu/tun2socks/v2/core/device"
	"github.com/xjasonlyu/tun2socks/v2/core/device/tun"
	"github.com/xjasonlyu/tun2socks/v2/core/option"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

type Tunnel struct {
	name   string
	addr   string
	device device.Device
	stack  *stack.Stack
	trans  *tunTransportHandler
}

var defaultMTU uint32 = 1480

func CreateTUN(name string, addr string, hand ConnHandler) (*Tunnel, error) {
	dev, err := tun.Open(name, defaultMTU)
	if err != nil {
		return nil, err
	}

	handler := newTunTransportHandler()
	handler.registerConnHandler(hand)

	stack, err := core.CreateStack(&core.Config{
		LinkEndpoint:     dev,
		TransportHandler: handler,
		Options:          []option.Option{},
	})
	if err != nil {
		return nil, err
	}

	err = iface.CreateTun(name, addr, defaultMTU)
	if err != nil {
		return nil, err
	}

	handler.run()

	return &Tunnel{
		name:   name,
		addr:   addr,
		device: dev,
		stack:  stack,
		trans:  handler,
	}, nil
}

func (t *Tunnel) GetName() string {
	return t.name
}

func (t *Tunnel) GetAddr() string {
	return t.addr
}

func (t *Tunnel) Close() {
	t.trans.finish()
	t.stack.Close()
	t.device.Close()
	iface.DeleteTun(t.name)
}
