"""Client IP resolution, CIDR validation, and IP prefix truncation.

Plan §7.1:
- Resolves real client IP safely without trusting spoofed forwarding headers.
- Evaluates peer trust against Cloudflare ranges and configured proxy CIDRs.
- Constant-time validation of edge secret for redirect service.
- Prefix truncation: /24 IPv4 and /48 IPv6 for privacy-preserving storage in audit/sessions.
"""

import hmac
import ipaddress
import os
from typing import Any

from django.conf import settings

from apps.core.net.cloudflare_ranges import CLOUDFLARE_RANGES


def parse_ip(raw: str) -> ipaddress.IPv4Address | ipaddress.IPv6Address | None:
    """Parse an IP string, returning None if invalid."""
    try:
        clean = raw.strip().strip("[]")
        # Strip port if present e.g. "1.2.3.4:5678" or "[2001:db8::1]:8080"
        if ":" in clean and not (clean.startswith(":") or "::" in clean):
            # IPv4 with port or IPv6 with port
            if clean.count(":") == 1:
                clean = clean.split(":")[0]
        return ipaddress.ip_address(clean)
    except Exception:
        return None


def in_any_cidr(ip: str | ipaddress.IPv4Address | ipaddress.IPv6Address, cidrs: list[str]) -> bool:
    """Report whether ip is inside any of the CIDR strings (invalid entries ignored)."""
    parsed_ip = (
        parse_ip(str(ip))
        if not isinstance(ip, (ipaddress.IPv4Address, ipaddress.IPv6Address))
        else ip
    )
    if parsed_ip is None:
        return False

    for s in cidrs:
        s = s.strip()
        if not s:
            continue
        try:
            net = ipaddress.ip_network(s, strict=False)
            if parsed_ip in net:
                return True
        except ValueError:
            single = parse_ip(s)
            if single is not None and single == parsed_ip:
                return True

    return False


def truncate_ip_to_prefix(ip_str: str) -> str:
    """Truncate an IP address to /24 (IPv4) or /48 (IPv6) for privacy-preserving storage."""
    ip = parse_ip(ip_str)
    if ip is None:
        return ""

    if isinstance(ip, ipaddress.IPv4Address):
        net = ipaddress.ip_network(f"{ip}/24", strict=False)
        return str(net)
    elif isinstance(ip, ipaddress.IPv6Address):
        net = ipaddress.ip_network(f"{ip}/48", strict=False)
        return str(net)

    return ""


class ClientIPResolver:
    """Resolves client IP addresses from HTTP requests safely."""

    def __init__(self, trusted_cidrs: list[str] | None = None) -> None:
        self.trusted_networks: list[ipaddress.IPv4Network | ipaddress.IPv6Network] = []
        if trusted_cidrs is not None:
            for item in trusted_cidrs:
                item = item.strip()
                if not item:
                    continue
                if item == "cloudflare":
                    for cf_cidr in CLOUDFLARE_RANGES:
                        self.trusted_networks.append(ipaddress.ip_network(cf_cidr, strict=False))
                    continue
                try:
                    self.trusted_networks.append(ipaddress.ip_network(item, strict=False))
                except ValueError:
                    single = parse_ip(item)
                    if single is not None:
                        self.trusted_networks.append(
                            ipaddress.ip_network(
                                f"{single}/32" if single.version == 4 else f"{single}/128",
                                strict=False,
                            )
                        )

    def is_trusted(self, ip: ipaddress.IPv4Address | ipaddress.IPv6Address | None) -> bool:
        """Check if an IP address is a trusted proxy."""
        if ip is None:
            return False
        for net in self.trusted_networks:
            if ip in net:
                return True
        return False

    def get_peer_ip(self, request: Any) -> ipaddress.IPv4Address | ipaddress.IPv6Address | None:
        """Extract the direct TCP peer IP from request.META."""
        meta = getattr(request, "META", {})
        # RemoteAddr in Go / REMOTE_ADDR in WSGI
        raw = meta.get("REMOTE_ADDR") or meta.get("RemoteAddr", "")
        if not raw:
            # Check attribute if httptest-like object
            raw = getattr(request, "RemoteAddr", "")
        return parse_ip(str(raw))

    def get_header(self, request: Any, header_name: str) -> str:
        """Get header from Django request.META or dictionary/object."""
        # Django request headers e.g. "CF-Connecting-IP" -> "HTTP_CF_CONNECTING_IP"
        meta = getattr(request, "META", {})
        django_name = "HTTP_" + header_name.upper().replace("-", "_")
        if django_name in meta:
            return str(meta[django_name])
        if header_name in meta:
            return str(meta[header_name])
        headers = getattr(request, "headers", None)
        if headers is not None:
            val = headers.get(header_name)
            if val is not None:
                return str(val)
        Header = getattr(request, "Header", None)
        if isinstance(Header, dict):
            return str(Header.get(header_name, ""))
        return ""

    def client_ip(self, request: Any) -> ipaddress.IPv4Address | ipaddress.IPv6Address | None:
        """Resolve the client IP address from a request."""
        # Rule 1: Edge worker shared secret on redirect
        edge_secret = getattr(settings, "EDGE_SHARED_SECRET", "")
        req_edge_secret = self.get_header(request, "X-QRit-Edge-Secret")
        if edge_secret and req_edge_secret and hmac.compare_digest(edge_secret, req_edge_secret):
            edge_ip_str = self.get_header(request, "X-QRit-Client-IP")
            edge_ip = parse_ip(edge_ip_str)
            if edge_ip is not None:
                return edge_ip

        peer = self.get_peer_ip(request)

        # Rule 2: Railway environment - check X-Real-IP
        if os.environ.get("RAILWAY_ENVIRONMENT_NAME"):
            real_ip_str = self.get_header(request, "X-Real-IP")
            real_ip = parse_ip(real_ip_str)
            if real_ip is not None:
                # If Railway edge connected from Cloudflare range, trust CF-Connecting-IP
                if in_any_cidr(real_ip, CLOUDFLARE_RANGES):
                    cf_ip_str = self.get_header(request, "CF-Connecting-IP")
                    cf_ip = parse_ip(cf_ip_str)
                    if cf_ip is not None:
                        return cf_ip
                return real_ip

        if not self.is_trusted(peer):
            return peer

        # Trusted peer: honour CF-Connecting-IP first
        cf_ip_str = self.get_header(request, "CF-Connecting-IP")
        if cf_ip_str:
            cf_ip = parse_ip(cf_ip_str)
            if cf_ip is not None:
                return cf_ip

        # Trusted peer: walk X-Forwarded-For right-to-left skipping trusted proxies
        xff = self.get_header(request, "X-Forwarded-For")
        if xff:
            parts = xff.split(",")
            for part in reversed(parts):
                ip = parse_ip(part.strip())
                if ip is None:
                    break
                if not self.is_trusted(ip):
                    return ip

        return peer

    def client_ip_string(self, request: Any) -> str:
        """Resolve client IP and return as string (or empty string if unknown)."""
        ip = self.client_ip(request)
        return str(ip) if ip is not None else ""

    def trusted_peer(self, request: Any) -> bool:
        """Report whether the direct TCP peer is a trusted proxy."""
        peer = self.get_peer_ip(request)
        return self.is_trusted(peer)


_DEFAULT_RESOLVER = ClientIPResolver(["cloudflare"])


def client_ip(request: Any) -> str:
    """Return the resolved client IP for a Django request as a string."""
    return _DEFAULT_RESOLVER.client_ip_string(request)
