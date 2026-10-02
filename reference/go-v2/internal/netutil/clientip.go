// Package netutil resolves the real client IP safely.
//
// Forwarding headers are attacker-controlled unless the TCP peer is a proxy we operate
// (Cloudflare, Fly's edge, the Next.js server). Rate limits and IP allowlists depend on
// this, so headers are honoured only when the direct peer is in the trusted set.
package netutil

import (
	"net"
	"net/http"
	"strings"
)

type ClientIPResolver struct {
	trusted []*net.IPNet
}

func NewClientIPResolver(trusted []*net.IPNet) *ClientIPResolver {
	return &ClientIPResolver{trusted: trusted}
}

func (c *ClientIPResolver) isTrusted(ip net.IP) bool {
	if ip == nil {
		return false
	}
	for _, n := range c.trusted {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// PeerIP returns the IP of the direct TCP peer.
func PeerIP(r *http.Request) net.IP {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return net.ParseIP(strings.Trim(host, "[]"))
}

// ClientIP returns the best-known client IP for the request.
func (c *ClientIPResolver) ClientIP(r *http.Request) net.IP {
	peer := PeerIP(r)
	if !c.isTrusted(peer) {
		return peer
	}
	if cf := net.ParseIP(strings.TrimSpace(r.Header.Get("CF-Connecting-IP"))); cf != nil {
		return cf
	}
	// X-Forwarded-For: walk from the right, skipping our own trusted hops.
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		for i := len(parts) - 1; i >= 0; i-- {
			ip := net.ParseIP(strings.TrimSpace(parts[i]))
			if ip == nil {
				break
			}
			if !c.isTrusted(ip) {
				return ip
			}
		}
	}
	return peer
}

// ClientIPString is ClientIP formatted, or "" if unknown.
func (c *ClientIPResolver) ClientIPString(r *http.Request) string {
	if ip := c.ClientIP(r); ip != nil {
		return ip.String()
	}
	return ""
}

// InAnyCIDR reports whether ip is inside any of the CIDR strings (invalid entries are ignored).
func InAnyCIDR(ip net.IP, cidrs []string) bool {
	if ip == nil {
		return false
	}
	for _, s := range cidrs {
		_, n, err := net.ParseCIDR(strings.TrimSpace(s))
		if err != nil {
			if single := net.ParseIP(strings.TrimSpace(s)); single != nil && single.Equal(ip) {
				return true
			}
			continue
		}
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// Prefix truncates an IP to /24 (IPv4) or /48 (IPv6) for privacy-preserving storage.
func Prefix(ip net.IP) string {
	if ip == nil {
		return ""
	}
	if v4 := ip.To4(); v4 != nil {
		return v4.Mask(net.CIDRMask(24, 32)).String() + "/24"
	}
	return ip.Mask(net.CIDRMask(48, 128)).String() + "/48"
}

// TrustedPeer reports whether the TCP peer is a trusted proxy, i.e. whether edge-supplied
// headers (CF-IPCountry, cf-ipcity, ...) can be believed.
func (c *ClientIPResolver) TrustedPeer(r *http.Request) bool { return c.isTrusted(PeerIP(r)) }
