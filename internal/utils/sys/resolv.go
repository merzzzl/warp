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
	errNetworkSetup     = errors.New("network setup failed")
)

type DNSConfig struct {
	Servers []string
}

type NetworkService struct {
	Name      string
	Device    string
	DNSConfig *DNSConfig
	GatewayIP string
}

type resolvHandler struct {
	service   *NetworkService
	backupDNS *DNSConfig
}

var resolv = prepareResolvHandler()

func prepareResolvHandler() *resolvHandler {
	i, err := defaultRouteInterface()
	if err != nil {
		panic(err)
	}

	g, err := defaultRouteGateway()
	if err != nil {
		panic(err)
	}

	r, err := newResolvHandler(i, g)
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

func defaultRouteGateway() (string, error) {
	out, err := Command("route -n get default | grep 'gateway' | awk '{print $2}'")
	if err != nil {
		return "", fmt.Errorf("failed to get default gateway: %w", err)
	}

	return strings.TrimSpace(out), nil
}

func newResolvHandler(interfaceName, gatewayAddr string) (*resolvHandler, error) {
	service, err := detectNetworkService(interfaceName)
	if err != nil {
		return nil, fmt.Errorf("failed to detect network service: %w", err)
	}

	service.GatewayIP = gatewayAddr

	dnsServers, err := getCurrentDNSServers(service.Name)
	if err != nil {
		return nil, err
	}

	return &resolvHandler{
		service:   service,
		backupDNS: &DNSConfig{Servers: dnsServers},
	}, nil
}

func detectNetworkService(interfaceName string) (*NetworkService, error) {
	out, err := Command("networksetup -listnetworkserviceorder")
	if err != nil {
		return nil, fmt.Errorf("failed to get network services: %w", err)
	}

	re := regexp.MustCompile(`\((\d+)\) (.+)\n\(Hardware Port: .+, Device: ` + interfaceName + `\)`)

	matches := re.FindStringSubmatch(out)
	if len(matches) == 0 {
		return nil, fmt.Errorf("%w: %s", errNoNetworkService, interfaceName)
	}

	return &NetworkService{
		Name:   matches[2],
		Device: interfaceName,
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

	if _, err := Command("networksetup -setdnsservers %s %s", r.service.Name, servers); err != nil {
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
	if len(r.backupDNS.Servers) == 0 {
		return []string{r.service.GatewayIP}
	}

	return r.backupDNS.Servers
}

func (r *resolvHandler) RestoreDNS(currentAddr string) error {
	var result []string
	for _, elem := range r.backupDNS.Servers {
		if elem != currentAddr {
			result = append(result, elem)
		}
	}
	return r.SetDNS(result)
}

func SetDNS(dns []string) error {
	return resolv.SetDNS(dns)
}

func GetOriginalDNS() []string {
	return resolv.GetOriginalDNS()
}

func RestoreDNS(currentAddr string) error {
	return resolv.RestoreDNS(currentAddr)
}
