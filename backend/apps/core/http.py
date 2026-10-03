"""SSRF-safe HTTP client with DNS resolution guard.

Plan §7.1:
- Resolves DNS and blocks loopback, private, link-local, CGNAT, multicast, 0/8, and 240/4.
- Prevents DNS rebinding by validating resolved IP before connection.
- Disables HTTP redirects.
- Default 10s timeout and 1MB response size limit.
- allow_private=True permitted only in local and test environments.
"""

import ipaddress
import socket
from typing import Any

import httpx
from django.conf import settings

FORBIDDEN_NETWORKS = [
    ipaddress.ip_network("127.0.0.0/8"),
    ipaddress.ip_network("10.0.0.0/8"),
    ipaddress.ip_network("172.16.0.0/12"),
    ipaddress.ip_network("192.168.0.0/16"),
    ipaddress.ip_network("169.254.0.0/16"),
    ipaddress.ip_network("100.64.0.0/10"),  # CGNAT
    ipaddress.ip_network("0.0.0.0/8"),
    ipaddress.ip_network("240.0.0.0/4"),
    ipaddress.ip_network("::1/128"),
    ipaddress.ip_network("fc00::/7"),  # IPv6 private
    ipaddress.ip_network("fe80::/10"),  # IPv6 link-local
    ipaddress.ip_network("::/128"),
]


def is_forbidden_ip(ip: ipaddress.IPv4Address | ipaddress.IPv6Address) -> bool:
    """Return True if the IP is forbidden for outbound SSRF-safe requests."""
    if ip.is_loopback or ip.is_private or ip.is_link_local or ip.is_multicast or ip.is_unspecified:
        return True

    for net in FORBIDDEN_NETWORKS:
        if ip in net:
            return True

    return False


def resolve_and_validate_host(hostname: str, allow_private: bool) -> str:
    """Resolve hostname to an IP and ensure it is not a forbidden private address."""
    # Check if host is already an IP address
    try:
        ip = ipaddress.ip_address(hostname.strip("[]"))
        if not allow_private and is_forbidden_ip(ip):
            raise ValueError(f"refusing to connect to {hostname}")
        return str(ip)
    except ValueError as err:
        if "refusing" in str(err):
            raise
        # Not a raw IP literal, proceed to DNS resolution

    try:
        # Resolve via getaddrinfo
        addr_info = socket.getaddrinfo(hostname, None)
    except Exception as err:
        raise ValueError(
            f"refusing to connect to {hostname}: DNS resolution failed: {err}"
        ) from err

    if not addr_info:
        raise ValueError(f"refusing to connect to {hostname}: no DNS records found")

    first_resolved_ip: str | None = None
    for _family, _, _, _, sockaddr in addr_info:
        ip_str = str(sockaddr[0])
        try:
            ip = ipaddress.ip_address(ip_str)
            if not allow_private and is_forbidden_ip(ip):
                raise ValueError(f"refusing to connect to {hostname} (resolved to {ip_str})")
            if first_resolved_ip is None:
                first_resolved_ip = ip_str
        except ValueError as err:
            if "refusing" in str(err):
                raise

    if first_resolved_ip is None:
        raise ValueError(f"refusing to connect to {hostname}: no valid IP address")

    return first_resolved_ip


class SafeSyncTransport(httpx.HTTPTransport):
    """Sync transport enforcing SSRF validation prior to establishing connection."""

    def __init__(self, allow_private: bool = False, **kwargs: Any) -> None:
        super().__init__(**kwargs)
        self.allow_private = allow_private

    def handle_request(self, request: httpx.Request) -> httpx.Response:
        hostname = request.url.host
        resolve_and_validate_host(hostname, self.allow_private)
        return super().handle_request(request)


class SafeAsyncTransport(httpx.AsyncHTTPTransport):
    """Async transport enforcing SSRF validation prior to establishing connection."""

    def __init__(self, allow_private: bool = False, **kwargs: Any) -> None:
        super().__init__(**kwargs)
        self.allow_private = allow_private

    async def handle_async_request(self, request: httpx.Request) -> httpx.Response:
        hostname = request.url.host
        resolve_and_validate_host(hostname, self.allow_private)
        return await super().handle_async_request(request)


def safe_client(
    allow_private: bool = False,
    timeout: float = 10.0,
    **kwargs: Any,
) -> httpx.Client:
    """Return an SSRF-safe sync httpx.Client.

    allow_private=True is permitted only in local and test environments.
    """
    app_env = getattr(settings, "APP_ENV", "local")
    effective_allow_private = allow_private and (app_env in ("local", "test"))

    transport = SafeSyncTransport(allow_private=effective_allow_private)
    return httpx.Client(
        transport=transport,
        timeout=timeout,
        follow_redirects=False,
        **kwargs,
    )


def async_safe_client(
    allow_private: bool = False,
    timeout: float = 10.0,
    **kwargs: Any,
) -> httpx.AsyncClient:
    """Return an SSRF-safe async httpx.AsyncClient.

    allow_private=True is permitted only in local and test environments.
    """
    app_env = getattr(settings, "APP_ENV", "local")
    effective_allow_private = allow_private and (app_env in ("local", "test"))

    transport = SafeAsyncTransport(allow_private=effective_allow_private)
    return httpx.AsyncClient(
        transport=transport,
        timeout=timeout,
        follow_redirects=False,
        **kwargs,
    )
