"""Custom database fields matching the PostgreSQL schema types.

Plan §6.1 rule 2:
- citext -> core.fields.CIText (TextField subclass with db_type="citext")
- char(n) -> core.fields.FixedCharField (CharField subclass with db_type="char(n)")
- cidr -> core.fields.CIDRField (Field subclass with db_type="cidr")
"""

from typing import TYPE_CHECKING, Any

from django.db import models

if TYPE_CHECKING:
    _TextFieldBase = models.TextField[Any, Any]
    _CharFieldBase = models.CharField[Any, Any]
    _FieldBase = models.Field[Any, Any]
else:
    _TextFieldBase = models.TextField
    _CharFieldBase = models.CharField
    _FieldBase = models.Field


class CIText(_TextFieldBase):
    """Case-insensitive text column using PostgreSQL's citext extension."""

    description = "Case-insensitive text"

    def db_type(self, connection: Any) -> str:
        if connection.vendor == "postgresql":
            return "citext"
        return "text"


class FixedCharField(_CharFieldBase):
    """Fixed-length character column char(n)."""

    description = "Fixed-length character field"

    def db_type(self, connection: Any) -> str:
        if connection.vendor == "postgresql":
            return f"char({self.max_length})"
        return f"varchar({self.max_length})"


class CIDRField(_FieldBase):
    """PostgreSQL CIDR network address column."""

    description = "PostgreSQL CIDR network address"

    def get_internal_type(self) -> str:
        return "CharField"

    def db_type(self, connection: Any) -> str:
        if connection.vendor == "postgresql":
            return "cidr"
        return "varchar(45)"
