package netguard

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"
)

var blockedNets []*net.IPNet

var nat64 = &net.IPNet{IP: net.ParseIP("64:ff9b::"), Mask: net.CIDRMask(96, 128)}

func init() {
	for _, cidr := range []string{
		"0.0.0.0/8",
		"10.0.0.0/8",
		"100.64.0.0/10",
		"127.0.0.0/8",
		"169.254.0.0/16",
		"172.16.0.0/12",
		"192.0.0.0/24",
		"192.0.2.0/24",
		"192.168.0.0/16",
		"198.18.0.0/15",
		"198.51.100.0/24",
		"203.0.113.0/24",
		"224.0.0.0/4",
		"240.0.0.0/4",
		"255.255.255.255/32",
		"::1/128",
		"::/128",
		"64:ff9b:1::/48",
		"100::/64",
		"2001:db8::/32",
		"fc00::/7",
		"fe80::/10",
		"ff00::/8",
	} {
		if _, n, err := net.ParseCIDR(cidr); err == nil {
			blockedNets = append(blockedNets, n)
		}
	}
}

func IsBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast() {
		return true
	}
	for _, n := range blockedNets {
		if n.Contains(ip) {
			return true
		}
	}
	if ip16 := ip.To16(); ip16 != nil && ip.To4() == nil && nat64.Contains(ip16) {
		return IsBlockedIP(net.IP(ip16[12:16]))
	}
	return false
}

func vetIPs(host string) ([]net.IP, error) {
	if ip := net.ParseIP(host); ip != nil {
		if IsBlockedIP(ip) {
			return nil, fmt.Errorf("netguard: refusing blocked address %s", ip)
		}
		return []net.IP{ip}, nil
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, err
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("netguard: no addresses for %q", host)
	}
	for _, ip := range ips {
		if IsBlockedIP(ip) {
			return nil, fmt.Errorf("netguard: %q resolves to blocked address %s", host, ip)
		}
	}
	return ips, nil
}

var ErrPortNotAllowed = errors.New("netguard: only web ports 80 and 443 are allowed")

func AllowedPort(port string) bool {
	return port == "80" || port == "443"
}

func TargetAllowed(addr string) bool {
	_, port, err := net.SplitHostPort(addr)
	return err == nil && AllowedPort(port)
}

func DialContext(ctx context.Context, network, addr string, timeout time.Duration) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	if !AllowedPort(port) {
		return nil, ErrPortNotAllowed
	}
	ips, err := vetIPs(host)
	if err != nil {
		return nil, err
	}
	d := net.Dialer{Timeout: timeout}
	var lastErr error
	for _, ip := range ips {
		conn, err := d.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	return nil, lastErr
}
