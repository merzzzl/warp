package tun

import (
	"fmt"
	"net"
	"strings"
)

func addRoute(destination, gateway string) error {
	destination = strings.TrimSpace(destination)
	if _, err := command("route add -net %s %s", destination, gateway); err != nil {
		return fmt.Errorf("failed to add route: %e", err)
	}

	return nil
}

func getRoutes(gateway string) ([]string, error) {
	out, err := command("netstat -rn | awk '$2==\"%s\" {print $0}'", gateway)
	if err != nil {
		return nil, fmt.Errorf("failed to get routes: %e", err)
	}

	output := strings.TrimSpace(out)
	if output == "" {
		return nil, nil
	}

	routes := strings.Split(output, "\n")
	return routes, nil
}

func setTunAddress(name string, addr string, mtu uint32) error {
	ip, _, _ := net.ParseCIDR(addr)
	if _, err := command("ifconfig %s inet %s %s mtu %d up", name, addr, ip.String(), mtu); err != nil {
		return fmt.Errorf("failed to set addr: %e", err)
	}

	return nil
}

func DefaultRouteInterface() (string, error) {
	out, err := command("route -n get default | grep 'interface' | awk 'NR==1{print $2}'")
	if err != nil {
		return "", fmt.Errorf("failed to get default iface: %e", err)
	}

	return strings.TrimSpace(out), nil
}
