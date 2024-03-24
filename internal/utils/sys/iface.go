package sys

import (
	"fmt"
	"strings"
)

// CreateTun creates a new TUN device with the given parameters.
func CreateTun(name, ip string, mtu uint32) error {
	if _, err := Command("ifconfig %s inet %s %s mtu %d up", name, ip, ip, mtu); err != nil {
		return fmt.Errorf("failed to create tun: %w", err)
	}

	return nil
}

// DeleteTun deletes the given TUN device.
func DeleteTun(name string) error {
	if _, err := Command("ifconfig %s down", name); err != nil {
		return fmt.Errorf("failed to delete tun: %w", err)
	}

	return nil
}

// AddRoute adds a new static route with the given parameters.
func AddRoute(destination, gateway string) error {
	destination = strings.TrimSpace(destination)
	if _, err := Command("route add -net %s %s", destination, gateway); err != nil {
		return fmt.Errorf("failed to add route: %w", err)
	}

	return nil
}
