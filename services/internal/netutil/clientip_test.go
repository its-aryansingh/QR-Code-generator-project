package netutil

import (
	"net"
	"net/http/httptest"
	"testing"
)

func mustCIDR(s string) *net.IPNet { _, n, _ := net.ParseCIDR(s); return n }

func TestClientIPIgnoresSpoofedHeadersFromUntrustedPeer(t *testing.T) {
	res := NewClientIPResolver([]*net.IPNet{mustCIDR("10.0.0.0/8")})
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "203.0.113.7:5555"
	r.Header.Set("CF-Connecting-IP", "1.2.3.4")
	r.Header.Set("X-Forwarded-For", "1.2.3.4")
	if got := res.ClientIPString(r); got != "203.0.113.7" {
		t.Fatalf("spoofed header honoured: %s", got)
	}
}

func TestClientIPHonoursTrustedProxy(t *testing.T) {
	res := NewClientIPResolver([]*net.IPNet{mustCIDR("10.0.0.0/8")})
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.1.2.3:5555"
	r.Header.Set("X-Forwarded-For", "198.51.100.9, 10.9.9.9")
	if got := res.ClientIPString(r); got != "198.51.100.9" {
		t.Fatalf("want 198.51.100.9 got %s", got)
	}
	r.Header.Set("CF-Connecting-IP", "192.0.2.44")
	if got := res.ClientIPString(r); got != "192.0.2.44" {
		t.Fatalf("want CF header, got %s", got)
	}
}

func TestInAnyCIDRAndPrefix(t *testing.T) {
	ip := net.ParseIP("192.0.2.55")
	if !InAnyCIDR(ip, []string{"192.0.2.0/24"}) || InAnyCIDR(ip, []string{"198.51.100.0/24", "bad"}) {
		t.Fatal("cidr match wrong")
	}
	if !InAnyCIDR(ip, []string{"192.0.2.55"}) {
		t.Fatal("single IP entry should match")
	}
	if Prefix(ip) != "192.0.2.0/24" || Prefix(net.ParseIP("2001:db8:1:2::1")) != "2001:db8:1::/48" {
		t.Fatalf("prefix wrong: %s %s", Prefix(ip), Prefix(net.ParseIP("2001:db8:1:2::1")))
	}
}
