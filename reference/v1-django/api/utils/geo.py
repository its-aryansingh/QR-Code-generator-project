"""Scan geolocation.

Scans were being written with country_code, city and region left null, so
every geographic chart in the dashboard was permanently empty regardless of
traffic.

Resolution order:

1. CDN geo headers (Cloudflare, Vercel, Fastly, AWS CloudFront, Render).
   Free, instant and accurate -- this is what production actually hits.
2. An optional lookup service, if GEOIP_LOOKUP_URL is configured.
3. Nothing, leaving the fields null rather than inventing a location.
"""

import json
import logging
import os
import urllib.request

logger = logging.getLogger(__name__)

GEOIP_LOOKUP_URL = os.environ.get("GEOIP_LOOKUP_URL", "")
GEOIP_TIMEOUT = float(os.environ.get("GEOIP_TIMEOUT", "2.0"))

# header -> field, in priority order
COUNTRY_HEADERS = [
    "HTTP_CF_IPCOUNTRY",
    "HTTP_X_VERCEL_IP_COUNTRY",
    "HTTP_CLOUDFRONT_VIEWER_COUNTRY",
    "HTTP_X_GEO_COUNTRY",
    "HTTP_X_COUNTRY_CODE",
]
CITY_HEADERS = [
    "HTTP_CF_IPCITY",
    "HTTP_X_VERCEL_IP_CITY",
    "HTTP_CLOUDFRONT_VIEWER_CITY",
    "HTTP_X_GEO_CITY",
]
REGION_HEADERS = [
    "HTTP_X_VERCEL_IP_COUNTRY_REGION",
    "HTTP_CLOUDFRONT_VIEWER_COUNTRY_REGION",
    "HTTP_CF_REGION",
    "HTTP_X_GEO_REGION",
]
LAT_HEADERS = ["HTTP_X_VERCEL_IP_LATITUDE", "HTTP_CLOUDFRONT_VIEWER_LATITUDE", "HTTP_CF_IPLATITUDE"]
LON_HEADERS = ["HTTP_X_VERCEL_IP_LONGITUDE", "HTTP_CLOUDFRONT_VIEWER_LONGITUDE", "HTTP_CF_IPLONGITUDE"]

COUNTRY_NAMES = {
    "US": "United States", "GB": "United Kingdom", "IN": "India", "CA": "Canada",
    "AU": "Australia", "DE": "Germany", "FR": "France", "ES": "Spain", "IT": "Italy",
    "NL": "Netherlands", "SE": "Sweden", "NO": "Norway", "DK": "Denmark", "FI": "Finland",
    "IE": "Ireland", "PL": "Poland", "PT": "Portugal", "CH": "Switzerland", "AT": "Austria",
    "BE": "Belgium", "CZ": "Czechia", "GR": "Greece", "RO": "Romania", "HU": "Hungary",
    "JP": "Japan", "CN": "China", "KR": "South Korea", "SG": "Singapore", "HK": "Hong Kong",
    "TW": "Taiwan", "TH": "Thailand", "VN": "Vietnam", "MY": "Malaysia", "ID": "Indonesia",
    "PH": "Philippines", "BD": "Bangladesh", "PK": "Pakistan", "LK": "Sri Lanka",
    "AE": "United Arab Emirates", "SA": "Saudi Arabia", "IL": "Israel", "TR": "Turkey",
    "QA": "Qatar", "KW": "Kuwait", "EG": "Egypt", "ZA": "South Africa", "NG": "Nigeria",
    "KE": "Kenya", "GH": "Ghana", "MA": "Morocco", "BR": "Brazil", "MX": "Mexico",
    "AR": "Argentina", "CL": "Chile", "CO": "Colombia", "PE": "Peru", "NZ": "New Zealand",
    "RU": "Russia", "UA": "Ukraine",
}


def _first_header(meta, names):
    for name in names:
        value = (meta.get(name) or "").strip()
        if value and value.upper() not in ("XX", "T1", "UNKNOWN"):
            return value
    return None


def _to_decimal(value):
    try:
        return round(float(value), 6)
    except (TypeError, ValueError):
        return None


def country_name(code):
    if not code:
        return None
    return COUNTRY_NAMES.get(code.upper(), code.upper())


def resolve(request, ip_address=None):
    """Return a dict of geo fields; values are None when unknown."""
    meta = request.META
    code = _first_header(meta, COUNTRY_HEADERS)
    result = {
        "country_code": code.upper()[:2] if code else None,
        "country_name": country_name(code),
        "city": _first_header(meta, CITY_HEADERS),
        "region": _first_header(meta, REGION_HEADERS),
        "latitude": _to_decimal(_first_header(meta, LAT_HEADERS)),
        "longitude": _to_decimal(_first_header(meta, LON_HEADERS)),
    }
    if result["country_code"] or not GEOIP_LOOKUP_URL or not ip_address:
        return result
    return {**result, **_lookup_service(ip_address)}


def _lookup_service(ip_address):
    """Query the configured geo service. Shape is normalised defensively
    because the free providers all disagree on field names."""
    if ip_address in ("unknown", "127.0.0.1", "::1") or ip_address.startswith(("10.", "192.168.")):
        return {}
    try:
        url = GEOIP_LOOKUP_URL.replace("{ip}", ip_address)
        req = urllib.request.Request(url, headers={"User-Agent": "QRit/1.0"})
        with urllib.request.urlopen(req, timeout=GEOIP_TIMEOUT) as response:
            payload = json.loads(response.read(8192).decode("utf-8", "replace"))
    except Exception as exc:
        logger.debug("geo lookup failed for %s: %s", ip_address, exc)
        return {}

    code = payload.get("country_code") or payload.get("countryCode") or payload.get("country")
    code = code.upper()[:2] if isinstance(code, str) else None
    return {
        "country_code": code,
        "country_name": payload.get("country_name") or payload.get("country") or country_name(code),
        "city": payload.get("city"),
        "region": payload.get("region") or payload.get("region_name") or payload.get("regionName"),
        "latitude": _to_decimal(payload.get("latitude") or payload.get("lat")),
        "longitude": _to_decimal(payload.get("longitude") or payload.get("lon")),
    }
