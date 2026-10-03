"""Tests for client IP resolution and CIDR matching.

Ported from reference/go-v2/internal/netutil/clientip_test.go:
- TestClientIPIgnoresSpoofedHeadersFromUntrustedPeer -> test_client_ip_ignores_spoofed_headers_from_untrusted_peer
- TestClientIPHonoursTrustedProxy -> test_client_ip_honours_trusted_proxy
- TestInAnyCIDRAndPrefix -> test_in_any_cidr_and_prefix
"""

from unittest.mock import Mock

from apps.core.net.client_ip import ClientIPResolver, in_any_cidr, truncate_ip_to_prefix


def test_client_ip_ignores_spoofed_headers_from_untrusted_peer() -> None:
    """Port of TestClientIPIgnoresSpoofedHeadersFromUntrustedPeer."""
    res = ClientIPResolver(["10.0.0.0/8"])

    req = Mock()
    req.META = {
        "REMOTE_ADDR": "203.0.113.7",
        "HTTP_CF_CONNECTING_IP": "1.2.3.4",
        "HTTP_X_FORWARDED_FOR": "1.2.3.4",
    }
    req.headers = {
        "CF-Connecting-IP": "1.2.3.4",
        "X-Forwarded-For": "1.2.3.4",
    }

    assert res.client_ip_string(req) == "203.0.113.7"


def test_client_ip_honours_trusted_proxy() -> None:
    """Port of TestClientIPHonoursTrustedProxy."""
    res = ClientIPResolver(["10.0.0.0/8"])

    # X-Forwarded-For right-to-left skipping trusted proxy
    req1 = Mock()
    req1.META = {
        "REMOTE_ADDR": "10.1.2.3",
        "HTTP_X_FORWARDED_FOR": "198.51.100.9, 10.9.9.9",
    }
    req1.headers = {
        "X-Forwarded-For": "198.51.100.9, 10.9.9.9",
    }
    assert res.client_ip_string(req1) == "198.51.100.9"

    # CF-Connecting-IP takes precedence when from trusted proxy
    req2 = Mock()
    req2.META = {
        "REMOTE_ADDR": "10.1.2.3",
        "HTTP_CF_CONNECTING_IP": "192.0.2.44",
        "HTTP_X_FORWARDED_FOR": "198.51.100.9, 10.9.9.9",
    }
    req2.headers = {
        "CF-Connecting-IP": "192.0.2.44",
        "X-Forwarded-For": "198.51.100.9, 10.9.9.9",
    }
    assert res.client_ip_string(req2) == "192.0.2.44"


def test_in_any_cidr_and_prefix() -> None:
    """Port of TestInAnyCIDRAndPrefix."""
    ip = "192.0.2.55"

    assert in_any_cidr(ip, ["192.0.2.0/24"])
    assert not in_any_cidr(ip, ["198.51.100.0/24", "bad"])
    assert in_any_cidr(ip, ["192.0.2.55"])  # Single IP entry match

    assert truncate_ip_to_prefix(ip) == "192.0.2.0/24"
    assert truncate_ip_to_prefix("2001:db8:1:2::1") == "2001:db8:1::/48"
