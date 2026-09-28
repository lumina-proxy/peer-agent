package netguard

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestIsBlockedIP(t *testing.T) {
	cases := map[string]bool{
		"10.0.0.1":             true,
		"172.16.5.4":           true,
		"192.168.1.1":          true,
		"127.0.0.1":            true,
		"169.254.1.1":          true,
		"100.64.0.1":           true,
		"0.0.0.0":              true,
		"::1":                  true,
		"fe80::1":              true,
		"fc00::1":              true,
		"8.8.8.8":              false,
		"1.1.1.1":              false,
		"93.184.216.34":        false,
		"2606:4700:4700::1111": false,
	}
	for s, want := range cases {
		ip := net.ParseIP(s)
		if got := IsBlockedIP(ip); got != want {
			t.Errorf("IsBlockedIP(%s) = %v, want %v", s, got, want)
		}
	}
	if !IsBlockedIP(nil) {
		t.Error("nil IP must be blocked")
	}
}

func TestAllowedPorts(t *testing.T) {
	for _, addr := range []string{"example.com:80", "example.com:443", "[2001:db8::1]:443"} {
		if !TargetAllowed(addr) {
			t.Errorf("%s should be allowed", addr)
		}
	}
	for _, addr := range []string{"example.com:25", "example.com:22", "example.com:8080", "example.com", "example.com:465"} {
		if TargetAllowed(addr) {
			t.Errorf("%s should be refused", addr)
		}
	}
}

func TestDialRefusesNonWebPort(t *testing.T) {
	if _, err := DialContext(context.Background(), "tcp", "example.com:25", time.Second); err != ErrPortNotAllowed {
		t.Fatalf("got %v, want ErrPortNotAllowed", err)
	}
}

func TestNAT64EmbeddedAddresses(t *testing.T) {
	cases := map[string]bool{
		"64:ff9b::a00:1":    true,
		"64:ff9b::7f00:1":   true,
		"64:ff9b::c0a8:101": true,
		"64:ff9b::808:808":  false,
		"::ffff:127.0.0.1":  true,
		"::ffff:10.1.2.3":   true,
		"2606:4700::1111":   false,
	}
	for s, want := range cases {
		if got := IsBlockedIP(net.ParseIP(s)); got != want {
			t.Errorf("IsBlockedIP(%s) = %v, want %v", s, got, want)
		}
	}
}
