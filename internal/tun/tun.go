package tun

import (
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

var DefaultMTU uint32 = 1480

func CreateTUN(name string, addr string, mtu uint32, hand ConnHandler) (*Tunnel, error) {
	dev, err := tun.Open(name, mtu)
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

	err = setTunAddress(name, addr+"/32", mtu)
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

func (t *Tunnel) AddTunRoute(addr string) error {
	return addRoute(addr, t.addr)
}

func (t *Tunnel) GetTunRoutes() ([]string, error) {
	return getRoutes(t.addr)
}

func (t *Tunnel) Close() {
	t.trans.finish()
	t.stack.Close()
	t.device.Close()
}
