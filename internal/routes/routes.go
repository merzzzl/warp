package routes

import (
	"math/rand"
	"net"
	"sync"

	"github.com/merzzzl/warp/internal/log"
	"github.com/merzzzl/warp/internal/sys/iface"
)

var (
	routes    map[string]struct{} = make(map[string]struct{})
	mutex     sync.Mutex
	ifaceName string = "lo0"
	subnet    net.IP
)

func SetSubnet(ip net.IP) {
	subnet = ip
}

func GetAll() []string {
	mutex.Lock()
	defer mutex.Unlock()

	var list []string
	for route := range routes {
		list = append(list, route)
	}

	return list
}

func Free() error {
	mutex.Lock()
	defer mutex.Unlock()

	for route := range routes {
		log.Info().Str("ip", route).Msg("ROT", "remove route")

		if err := iface.DeleteAlias(ifaceName, route); err != nil {
			return err
		}

		delete(routes, route)
	}

	return nil
}

func GetFreeHost() (net.IP, error) {
	mutex.Lock()
	defer mutex.Unlock()

	for {
		ip := net.IPv4(subnet[12], subnet[13], subnet[14], byte(rand.Intn(255)))
		if _, ok := routes[ip.String()]; !ok {
			log.Info().Str("ip", ip.String()).Msg("ROT", "add route")

			if err := iface.AddAlias(ifaceName, ip.String()); err != nil {
				return nil, err
			}

			routes[ip.String()] = struct{}{}

			return ip, nil
		}
	}
}

func ApplyRoute(ip string, gw string) error {
	mutex.Lock()
	defer mutex.Unlock()

	log.Info().Str("ip", ip).Msg("ROT", "add route")

	if err := iface.AddRoute(ip, gw); err != nil {
		return err
	}

	return nil
}
