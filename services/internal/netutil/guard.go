package netutil

import (
	"fmt"
	"net"
	"net/http"
	"syscall"
	"time"
)

// Forbidden reports addresses outbound calls to customer-configured URLs must never reach:
// loopback, private, link-local (incl. cloud metadata), CGNAT, multicast and unspecified.
func Forbidden(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified() || ip.IsMulticast() || ip.IsInterfaceLocalMulticast() {
		return true
	}
	if v4 := ip.To4(); v4 != nil && (v4[0] == 100 && v4[1]&0xc0 == 64 || v4[0] == 0 || v4[0] >= 240) {
		return true // 100.64/10 CGNAT, 0/8, 240/4
	}
	return false
}

// SafeHTTPClient returns a client whose dialer refuses forbidden addresses at connect time
// (after DNS resolution, so DNS rebinding can't slip through), never follows redirects and
// ignores proxy environment variables. allowPrivate disables the guard (local development).
func SafeHTTPClient(allowPrivate bool, timeout time.Duration) *http.Client {
	d := &net.Dialer{Timeout: 5 * time.Second}
	if !allowPrivate {
		d.Control = func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			if Forbidden(net.ParseIP(host)) {
				return fmt.Errorf("refusing to connect to %s", host)
			}
			return nil
		}
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &http.Client{Timeout: timeout, Transport: &http.Transport{DialContext: d.DialContext,
		TLSHandshakeTimeout: 5 * time.Second, MaxIdleConns: 50, IdleConnTimeout: 60 * time.Second, ResponseHeaderTimeout: timeout},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}
