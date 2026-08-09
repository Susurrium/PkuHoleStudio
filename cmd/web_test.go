package main

import (
	"strings"
	"testing"
)

func TestNormalizeWebHostAcceptsLoopbackHosts(t *testing.T) {
	tests := []struct {
		name string
		host string
		want string
	}{
		{name: "localhost", host: "localhost", want: "127.0.0.1"},
		{name: "localhost whitespace", host: "  LOCALHOST  ", want: "127.0.0.1"},
		{name: "IPv4", host: "127.0.0.1", want: "127.0.0.1"},
		{name: "IPv6", host: "::1", want: "::1"},
		{name: "bracketed IPv6", host: " [::1] ", want: "::1"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := normalizeWebHost(test.host)
			if err != nil {
				t.Fatalf("normalizeWebHost(%q) returned error: %v", test.host, err)
			}
			if got != test.want {
				t.Fatalf("normalizeWebHost(%q) = %q, want %q", test.host, got, test.want)
			}
		})
	}
}

func TestNormalizeWebHostRejectsNonLoopbackHosts(t *testing.T) {
	hosts := []string{
		"",
		"   ",
		"0.0.0.0",
		"::",
		"[::]",
		"192.168.1.10",
		"example.com",
		"[::1",
		"::1]",
	}

	for _, host := range hosts {
		t.Run(host, func(t *testing.T) {
			if got, err := normalizeWebHost(host); err == nil {
				t.Fatalf("normalizeWebHost(%q) = %q, want an error", host, got)
			}
		})
	}
}

func TestWebHostFlagDescribesLoopbackRestriction(t *testing.T) {
	flag := newWebCmd().Flags().Lookup("host")
	if flag == nil {
		t.Fatal("web command has no --host flag")
	}
	if !strings.Contains(strings.ToLower(flag.Usage), "loopback") {
		t.Fatalf("--host help %q does not describe the loopback restriction", flag.Usage)
	}
}
