"""CSV downloads for the dashboard's Export buttons."""

import csv
import io
from typing import Any

from django.http import HttpResponse


def _cell(value: Any) -> Any:
    """Stops spreadsheet apps from running a cell as a formula (CSV injection)."""
    if isinstance(value, str) and value[:1] in ("=", "+", "-", "@", "\t", "\r"):
        return "'" + value
    return "" if value is None else value


def csv_response(filename: str, header: list[str], rows: list[list[Any]]) -> HttpResponse:
    buffer = io.StringIO()
    writer = csv.writer(buffer)
    writer.writerow(header)
    writer.writerows([[_cell(v) for v in row] for row in rows])
    response = HttpResponse(buffer.getvalue(), content_type="text/csv; charset=utf-8")
    response["Content-Disposition"] = f'attachment; filename="{filename}"'
    response["Cache-Control"] = "private, no-store"
    return response
