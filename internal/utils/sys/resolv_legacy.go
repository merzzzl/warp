package sys

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var (
	errNoNetworkService = errors.New("no service found for interface")
	errInvalidDNS       = errors.New("invalid DNS configuration")
)

type resolvHandler struct {
	Name       string
	Device     string
	DNSServers []string
	GatewayIP  string
}

var resolv *resolvHandler

func init() {
	r, err := newResolvHandler()
	if err != nil {
		panic(err)
	}

	resolv = r
}

func newResolvHandler() (*resolvHandler, error) {
	iface, err := Command("route -n get default | grep 'interface' | awk 'NR==1{print $2}'")
	if err != nil {
		return nil, fmt.Errorf("failed to get default iface: %w", err)
	}

	gateway, err := Command("route -n get default | grep 'gateway' | awk '{print $2}'")
	if err != nil {
		return nil, fmt.Errorf("failed to get default gateway: %w", err)
	}

	iface = strings.TrimSpace(iface)
	gateway = strings.TrimSpace(gateway)

	network, err := Command("networksetup -listnetworkserviceorder")
	if err != nil {
		return nil, fmt.Errorf("failed to get network services: %w", err)
	}

	re := regexp.MustCompile(`\((\d+)\) (.+)\n\(Hardware Port: .+, Device: ` + iface + `\)`)

	networkServices := re.FindStringSubmatch(network)
	if len(networkServices) == 0 {
		return nil, fmt.Errorf("%w: %s", errNoNetworkService, iface)
	}

	networkService := networkServices[2]

	dnsServers, err := getCurrentDNSServers(networkService)
	if err != nil {
		return nil, err
	}

	return &resolvHandler{
		Name:       networkService,
		Device:     iface,
		DNSServers: dnsServers,
		GatewayIP:  gateway,
	}, nil
}

func getCurrentDNSServers(serviceName string) ([]string, error) {
	out, err := Command("networksetup -getdnsservers %s", serviceName)
	if err != nil {
		return nil, fmt.Errorf("failed to get DNS servers: %w", err)
	}

	if strings.Contains(out, "There aren't any DNS Servers set") {
		return nil, nil
	}

	return strings.Fields(out), nil
}

func (r *resolvHandler) SetDNS(dns []string) error {
	var servers string
	if len(dns) == 0 {
		servers = "Empty"
	} else {
		servers = strings.Join(dns, " ")
	}

	if _, err := Command("networksetup -setdnsservers %s %s", r.Name, servers); err != nil {
		return fmt.Errorf("%w: %w", errNetworkSetup, err)
	}

	return r.flushDNSCache()
}

func (resolvHandler) flushDNSCache() error {
	if _, err := Command("killall -HUP mDNSResponder"); err != nil {
		return fmt.Errorf("failed to flush DNS cache: %w", err)
	}

	return nil
}

func (r *resolvHandler) GetOriginalDNS() []string {
	if len(r.DNSServers) == 0 {
		return []string{r.GatewayIP}
	}

	return r.DNSServers
}

func (r *resolvHandler) RestoreDNS() error {
	var result []string
	for _, elem := range r.DNSServers {
		if elem != r.GatewayIP {
			result = append(result, elem)
		}
	}
	return r.SetDNS(result)
}

func LSetDNS(dns []string) error {
	return resolv.SetDNS(append(dns, resolv.DNSServers...))
}

func LRestoreDNS() error {
	return resolv.RestoreDNS()
}
