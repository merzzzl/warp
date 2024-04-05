package sys

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/merzzzl/warp/internal/utils/log"
)

type resolvHandler struct {
	interfaceName  string
	serviceName    string
	restoreServers []string
}

var errNoNetworkService = errors.New("no service found for interface")

var resolv = prepareResolvHandler()

func prepareResolvHandler() *resolvHandler {
	i, err := defaultRouteInterface()
	if err != nil {
		panic(err)
	}

	r, err := newResolvHandler(i)
	if err != nil {
		panic(err)
	}

	return r
}

func defaultRouteInterface() (string, error) {
	out, err := Command("route -n get default | grep 'interface' | awk 'NR==1{print $2}'")
	if err != nil {
		return "", fmt.Errorf("failed to get default iface: %w", err)
	}

	return strings.TrimSpace(out), nil
}

func newResolvHandler(interfaceName string) (*resolvHandler, error) {
	out, err := Command("networksetup -listnetworkserviceorder")
	if err != nil {
		return nil, fmt.Errorf("failed to get net services: %w", err)
	}

	re, err := regexp.Compile(`\((\d+)\) (.+)\n\(Hardware Port: .+, Device: ` + interfaceName + `\)`)
	if err != nil {
		return nil, err
	}

	matches := re.FindStringSubmatch(out)
	if len(matches) == 0 {
		return nil, fmt.Errorf("%w: %s", errNoNetworkService, interfaceName)
	}

	serviceName := matches[2]

	out, err = Command("networksetup -getdnsservers %s", serviceName)
	if err != nil {
		return nil, fmt.Errorf("failed to get dns servers: %w", err)
	}

	var restoreServers []string
	if !strings.Contains(out, "There aren't any DNS Servers set") {
		restoreServers = strings.Fields(out)
	}

	return &resolvHandler{
		interfaceName:  interfaceName,
		serviceName:    serviceName,
		restoreServers: restoreServers,
	}, nil
}

// SetDNS sets the DNS server for a network service.
func SetDNS(dns []string) error {
	servers := strings.Join(dns, " ")
	if servers == "" {
		servers = "empty"
	}

	if _, err := Command("networksetup -setdnsservers %s %s", resolv.serviceName, servers); err != nil {
		return fmt.Errorf("failed to set dns server: %w", err)
	}

	if _, err := Command("killall -HUP mDNSResponder"); err != nil {
		log.Error().Err(err).Str("DNS", "failed to flush cache")
	}

	return nil
}

// GetOriginalDNS returns the original DNS servers for a network service.
func GetOriginalDNS() []string {
	if len(resolv.restoreServers) == 0 {
		return []string{
			"1.1.1.1",
			"8.8.8.8",
		}
	}

	return resolv.restoreServers
}

// RestoreDNS restores the original DNS servers for a network service.
func RestoreDNS() error {
	return SetDNS(resolv.restoreServers)
}
