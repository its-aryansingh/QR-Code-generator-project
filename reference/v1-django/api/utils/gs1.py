"""GS1 Digital Link.

GS1 Sunrise 2027 asks retail point-of-sale systems to read 2D barcodes that
carry a GTIN as well as a web address, so one printed QR both rings up at the
till and opens a product page. Puma, PepsiCo and P&G are already shipping it,
and Bitly ships GS1 Digital Link QR codes today.

A Digital Link URI looks like:

    https://id.acme.com/01/09506000134352/10/LOT42/21/SERIAL9?exp=271231

where the numeric path segments are GS1 Application Identifiers.
"""

import re
from urllib.parse import quote

# Application Identifiers that belong in the URI path, in GS1 canonical order.
PRIMARY_AI = "01"  # GTIN

PATH_AIS = [
    ("01", "gtin", "GTIN"),
    ("22", "cpv", "Consumer product variant"),
    ("10", "lot", "Batch / lot"),
    ("21", "serial", "Serial number"),
]

QUERY_AIS = [
    ("17", "expiry", "Expiration date (YYMMDD)"),
    ("11", "production_date", "Production date (YYMMDD)"),
    ("15", "best_before", "Best before (YYMMDD)"),
    ("3103", "net_weight", "Net weight (kg, 3dp)"),
    ("427", "country_of_origin", "Country of origin subdivision"),
]

AI_LABELS = {ai: label for ai, _, label in PATH_AIS + QUERY_AIS}
KEY_TO_AI = {key: ai for ai, key, _ in PATH_AIS + QUERY_AIS}


def gtin_check_digit(digits):
    """GS1 mod-10 check digit over the payload without its final digit."""
    total = 0
    for index, char in enumerate(reversed(digits)):
        weight = 3 if index % 2 == 0 else 1
        total += int(char) * weight
    return (10 - total % 10) % 10


def normalise_gtin(raw):
    """Return (gtin14, error). GTIN-8/12/13/14 are padded to 14 digits."""
    if not raw:
        return None, "A GTIN is required"
    digits = re.sub(r"\D", "", str(raw))
    if len(digits) not in (8, 12, 13, 14):
        return None, "A GTIN must be 8, 12, 13 or 14 digits"
    if gtin_check_digit(digits[:-1]) != int(digits[-1]):
        return None, f"GTIN {digits} has an invalid check digit"
    return digits.rjust(14, "0"), None


def validate_date(value, label):
    if not value:
        return None
    digits = re.sub(r"\D", "", str(value))
    if len(digits) != 6:
        return f"{label} must be 6 digits in YYMMDD form"
    month = int(digits[2:4])
    if not 1 <= month <= 12:
        return f"{label} has an invalid month"
    return None


def build_digital_link(domain, attributes):
    """Compose a GS1 Digital Link URI.

    `attributes` uses friendly keys (gtin, lot, serial, expiry, ...).
    Returns (uri, errors).
    """
    errors = []
    domain = (domain or "").strip().rstrip("/")
    domain = re.sub(r"^https?://", "", domain)
    if not domain:
        errors.append("A resolver domain is required, e.g. id.acme.com")

    gtin, gtin_error = normalise_gtin(attributes.get("gtin"))
    if gtin_error:
        errors.append(gtin_error)

    for key, label in (("expiry", "Expiration date"), ("production_date", "Production date"),
                       ("best_before", "Best before date")):
        error = validate_date(attributes.get(key), label)
        if error:
            errors.append(error)

    if errors:
        return None, errors

    path = f"/{PRIMARY_AI}/{gtin}"
    for ai, key, _ in PATH_AIS:
        if ai == PRIMARY_AI:
            continue
        value = attributes.get(key)
        if value:
            path += f"/{ai}/{quote(str(value), safe='')}"

    query = []
    for ai, key, _ in QUERY_AIS:
        value = attributes.get(key)
        if value:
            query.append(f"{ai}={quote(str(value), safe='')}")

    uri = f"https://{domain}{path}"
    if query:
        uri += "?" + "&".join(query)
    return uri, []


def parse_digital_link(uri):
    """Extract the AI key/value pairs from a Digital Link URI."""
    match = re.search(r"https?://[^/]+(/.*)$", uri or "")
    if not match:
        return {}
    path, _, query = match.group(1).partition("?")
    segments = [seg for seg in path.split("/") if seg]
    parsed = {}
    for index in range(0, len(segments) - 1, 2):
        ai, value = segments[index], segments[index + 1]
        if ai in AI_LABELS:
            parsed[ai] = value
    for pair in query.split("&"):
        ai, _, value = pair.partition("=")
        if ai in AI_LABELS:
            parsed[ai] = value
    return parsed


def describe(uri):
    """Human-readable breakdown for the dashboard preview panel."""
    return [
        {"ai": ai, "label": AI_LABELS.get(ai, ai), "value": value}
        for ai, value in parse_digital_link(uri).items()
    ]
