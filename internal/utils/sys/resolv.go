package sys

import (
	"errors"
	"fmt"
)

var (
	errNetworkSetup = errors.New("network setup failed")
)

func SetDNS(addr, domain string) error {
	if _, err := Command(`mkdir -p /etc/resolver && echo nameserver %s > /etc/resolver/%s`, addr, domain); err != nil {
		return fmt.Errorf("%w: %w", errNetworkSetup, err)
	}

	return flushDNSCache()
}

func flushDNSCache() error {
	if _, err := Command("killall -HUP mDNSResponder"); err != nil {
		return fmt.Errorf("failed to flush DNS cache: %w", err)
	}

	return nil
}

func RestoreDNS(domain string) error {
	if _, err := Command(`[ -f /etc/resolver/%s ] && rm /etc/resolver/%s`, domain, domain); err != nil {
		return err
	}

	return flushDNSCache()
}
