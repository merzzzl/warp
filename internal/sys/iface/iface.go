package iface

import (
	"fmt"
	"strings"

	"github.com/merzzzl/warp/internal/sys"
)

func DefaultRouteInterface() (string, error) {
	out, err := sys.Command("route -n get default | grep 'interface' | awk 'NR==1{print $2}'")
	if err != nil {
		return "", fmt.Errorf("failed to get default iface: %e", err)
	}

	return strings.TrimSpace(out), nil
}

func AddAlias(i string, ip string) error {
	if _, err := sys.Command("ifconfig %s alias %s", i, ip); err != nil {
		return fmt.Errorf("failed to add alias: %s", err.Error())
	}

	return nil
}

func CreateTun(name string, ip string, mtu uint32) error {
	if _, err := sys.Command("ifconfig %s inet %s %s mtu %d up", name, ip, ip, mtu); err != nil {
		return fmt.Errorf("failed to create tun: %s", err.Error())
	}

	return nil
}

func DeleteTun(name string) error {
	if _, err := sys.Command("ifconfig %s down", name); err != nil {
		return fmt.Errorf("failed to delete tun: %s", err.Error())
	}

	return nil
}

func AddRoute(destination, gateway string) error {
	destination = strings.TrimSpace(destination)
	if _, err := sys.Command("route add -net %s %s", destination, gateway); err != nil {
		return fmt.Errorf("failed to add route: %e", err)
	}

	return nil
}

func DeleteAlias(i string, ip string) error {
	if _, err := sys.Command("ifconfig %s -alias %s", i, ip); err != nil {
		return fmt.Errorf("failed to delete alias: %s", err.Error())
	}

	return nil
}
