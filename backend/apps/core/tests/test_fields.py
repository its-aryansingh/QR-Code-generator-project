"""Tests for custom database fields in apps.core.fields."""

from unittest.mock import MagicMock

from apps.core.fields import CIDRField, CIText, FixedCharField


def test_citext_db_type() -> None:
    field = CIText()
    pg_conn = MagicMock()
    pg_conn.vendor = "postgresql"
    assert field.db_type(pg_conn) == "citext"

    other_conn = MagicMock()
    other_conn.vendor = "sqlite"
    assert field.db_type(other_conn) == "text"


def test_fixed_char_field_db_type() -> None:
    field = FixedCharField(max_length=14)
    pg_conn = MagicMock()
    pg_conn.vendor = "postgresql"
    assert field.db_type(pg_conn) == "char(14)"

    other_conn = MagicMock()
    other_conn.vendor = "sqlite"
    assert field.db_type(other_conn) == "varchar(14)"


def test_cidr_field_db_type() -> None:
    field = CIDRField()
    pg_conn = MagicMock()
    pg_conn.vendor = "postgresql"
    assert field.db_type(pg_conn) == "cidr"

    other_conn = MagicMock()
    other_conn.vendor = "sqlite"
    assert field.db_type(other_conn) == "varchar(45)"
