import base64
import secrets
from io import BytesIO
import qrcode
from qrcode.constants import ERROR_CORRECT_M

import qrcode.image.svg

def generate_qr_base64(content: str, size: int, format: str = "png") -> str:
    qr = qrcode.QRCode(
        version=None,
        error_correction=ERROR_CORRECT_M,
        box_size=10,
        border=1,
    )
    qr.add_data(content or " ")
    qr.make(fit=True)

    if format == "svg":
        img = qr.make_image(image_factory=qrcode.image.svg.SvgPathImage)
        buf = BytesIO()
        img.save(buf)
        return base64.b64encode(buf.getvalue()).decode()

    img = qr.make_image(fill_color="black", back_color="white")
    # Resize to exact requested dimensions
    img = img.resize((size, size))
    buf = BytesIO()
    img.save(buf, format="PNG")
    return base64.b64encode(buf.getvalue()).decode()

def generate_short_code(length: int = 8) -> str:
    return secrets.token_urlsafe(length)[:length]


PREMIUM_TYPES = {"pdf", "images", "video", "mp3", "menu", "coupon", "business"}


def is_premium_type(qr_type: str) -> bool:
    return qr_type in PREMIUM_TYPES
