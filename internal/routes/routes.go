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
	aliases   map[string]struct{} = make(map[string]struct{})
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
	for alias := range aliases {
		list = append(list, alias)
	}

	for route := range aliases {
		list = append(list, route)
	}

	return list
}

func Free() error {
	mutex.Lock()
	defer mutex.Unlock()

	for route := range aliases {
		log.Info().Str("ip", route).Msg("ROT", "remove route")

		if err := iface.DeleteAlias(ifaceName, route); err != nil {
			return err
		}

		delete(aliases, route)
	}

	return nil
}

func GetFreeHost() (net.IP, error) {
	mutex.Lock()
	defer mutex.Unlock()

	for {
		ip := net.IPv4(subnet[12], subnet[13], subnet[14], byte(rand.Intn(255)))
		if _, ok := aliases[ip.String()]; !ok {
			log.Info().Str("ip", ip.String()).Msg("ROT", "add route")

			if err := iface.AddAlias(ifaceName, ip.String()); err != nil {
				return nil, err
			}

			aliases[ip.String()] = struct{}{}

			return ip, nil
		}
	}
}

func ApplyRoute(ip string, gw string) error {
	mutex.Lock()
	defer mutex.Unlock()

	if _, ok := routes[ip]; ok {
		return nil
	}

	log.Info().Str("ip", ip).Msg("ROT", "add route")

	if err := iface.AddRoute(ip, gw); err != nil {
		return err
	}

	routes[ip] = struct{}{}

	return nil
}
