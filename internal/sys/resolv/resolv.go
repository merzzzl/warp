package resolv

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/merzzzl/warp/internal/sys"
	"github.com/merzzzl/warp/internal/sys/iface"
)

type resolvHandler struct {
	interfaceName  string
	serviceName    string
	restoreServers []string
}

var resolv *resolvHandler

func init() {
	iface, err := iface.DefaultRouteInterface()
	if err != nil {
		panic(err)
	}

	resolv, err = newResolvHandler(iface)
	if err != nil {
		panic(err)
	}
}

func newResolvHandler(interfaceName string) (*resolvHandler, error) {
	out, err := sys.Command("networksetup -listnetworkserviceorder")
	if err != nil {
		return nil, fmt.Errorf("failed to get net services: %e", err)
	}

	re := regexp.MustCompile(`\((\d+)\) (.+)\n\(Hardware Port: .+, Device: ` + interfaceName + `\)`)
	matches := re.FindStringSubmatch(out)
	if len(matches) == 0 {
		return nil, fmt.Errorf("service for interface %s not found", interfaceName)
	}

	serviceName := matches[2]

	out, err = sys.Command("networksetup -getdnsservers %s", serviceName)
	if err != nil {
		return nil, fmt.Errorf("failed to get dns servers: %e", err)
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

func ListDNS() ([]string, error) {
	out, err := sys.Command("route -n get default | awk '/gateway:/{print $2}'")
	if err != nil {
		return nil, fmt.Errorf("failed to get default gateway: %e", err)
	}

	return []string{strings.TrimSpace(out)}, nil
}

func SetDNS(dns []string) error {
	servers := strings.Join(dns, " ")
	if servers == "" {
		servers = "empty"
	}

	if _, err := sys.Command("networksetup -setdnsservers %s %s", resolv.serviceName, servers); err != nil {
		return fmt.Errorf("failed to set dns server: %e", err)
	}

	return nil
}

func GetOriginalDNS() []string {
	return resolv.restoreServers
}

func Restore() error {
	return SetDNS(resolv.restoreServers)
}
