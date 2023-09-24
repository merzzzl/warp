package tun

import (
	"fmt"
	"regexp"
	"strings"
)

type ResolvHandler struct {
	interfaceName  string
	serviceName    string
	restoreServers []string
}

func NewHandler(interfaceName string) (*ResolvHandler, error) {
	out, err := command("networksetup -listnetworkserviceorder")
	if err != nil {
		return nil, fmt.Errorf("failed to get net services: %e", err)
	}

	re := regexp.MustCompile(`\((\d+)\) (.+)\n\(Hardware Port: .+, Device: ` + interfaceName + `\)`)
	matches := re.FindStringSubmatch(out)
	if len(matches) == 0 {
		return nil, fmt.Errorf("service for interface %s not found", interfaceName)
	}

	serviceName := matches[2]

	out, err = command("networksetup -getdnsservers %s", serviceName)
	if err != nil {
		return nil, fmt.Errorf("failed to get dns servers: %e", err)
	}

	var restoreServers []string
	if !strings.Contains(out, "There aren't any DNS Servers set") {
		restoreServers = strings.Fields(out)
	}

	return &ResolvHandler{
		interfaceName:  interfaceName,
		serviceName:    serviceName,
		restoreServers: restoreServers,
	}, nil
}

func (h *ResolvHandler) OriginalDNS() ([]string, error) {
	if len(h.restoreServers) == 0 {
		out, err := command("route -n get default | awk '/gateway:/{print $2}'")
		if err != nil {
			return nil, fmt.Errorf("failed to get default gateway: %e", err)
		}

		return []string{strings.TrimSpace(out)}, nil
	}

	return h.restoreServers, nil
}

func (h *ResolvHandler) Set(dns []string) error {
	servers := strings.Join(dns, " ")
	if servers == "" {
		servers = "empty"
	}

	if _, err := command("networksetup -setdnsservers %s %s", h.serviceName, servers); err != nil {
		return fmt.Errorf("failed to set dns server: %e", err)
	}

	return nil
}

func (h *ResolvHandler) Restore() error {
	return h.Set(h.restoreServers)
}
