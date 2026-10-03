from typing import Any

from django.db import migrations, models

import apps.core.fields
import apps.core.ids


def create_scan_events(apps: Any, schema_editor: Any) -> None:
    if schema_editor.connection.vendor == "postgresql":
        schema_editor.execute("""
            CREATE TABLE IF NOT EXISTS scan_events (
                event_id         uuid        NOT NULL,
                occurred_at      timestamptz NOT NULL,
                workspace_id     uuid        NOT NULL,
                qr_code_id       uuid        NOT NULL,
                version_id       uuid,
                campaign_id      uuid,
                domain_id        uuid        NOT NULL,
                rule_id          text,
                outcome          text        NOT NULL CHECK (outcome IN (
                                     'redirect','hosted_page','password_prompt','password_ok','password_fail',
                                     'geo_blocked','paused','expired','not_started','limit_reached','blocked')),
                method           text        NOT NULL DEFAULT 'GET',
                is_bot           boolean     NOT NULL DEFAULT false,
                bot_reason       text,
                is_duplicate     boolean     NOT NULL DEFAULT false,
                is_unique        boolean     NOT NULL DEFAULT false,
                visitor_hash     bytea       NOT NULL,
                device_type      text CHECK (device_type IN ('mobile','tablet','desktop','other')),
                os               text,
                os_version       text,
                browser          text,
                browser_version  text,
                country          char(2),
                region           text,
                city             text,
                language         text,
                referrer_host    text,
                utm_source       text,
                utm_medium       text,
                utm_campaign     text,
                serial           text,
                source           text        NOT NULL DEFAULT 'live' CHECK (source IN ('live','v1_import')),
                PRIMARY KEY (event_id, occurred_at)
            ) PARTITION BY RANGE (occurred_at);

            CREATE INDEX IF NOT EXISTS scan_events_qr_time_idx ON scan_events (qr_code_id, occurred_at DESC);
            CREATE INDEX IF NOT EXISTS scan_events_ws_time_idx ON scan_events (workspace_id, occurred_at DESC);

            CREATE TABLE IF NOT EXISTS scan_events_default PARTITION OF scan_events DEFAULT;
            CREATE TABLE IF NOT EXISTS scan_events_2026_09 PARTITION OF scan_events
                FOR VALUES FROM ('2026-09-01 00:00:00+00') TO ('2026-10-01 00:00:00+00');
            CREATE TABLE IF NOT EXISTS scan_events_2026_10 PARTITION OF scan_events
                FOR VALUES FROM ('2026-10-01 00:00:00+00') TO ('2026-11-01 00:00:00+00');
            CREATE TABLE IF NOT EXISTS scan_events_2026_11 PARTITION OF scan_events
                FOR VALUES FROM ('2026-11-01 00:00:00+00') TO ('2026-12-01 00:00:00+00');
        """)
    else:
        schema_editor.execute("""
            CREATE TABLE IF NOT EXISTS scan_events (
                event_id         uuid        NOT NULL,
                occurred_at      datetime    NOT NULL,
                workspace_id     uuid        NOT NULL,
                qr_code_id       uuid        NOT NULL,
                version_id       uuid,
                campaign_id      uuid,
                domain_id        uuid        NOT NULL,
                rule_id          text,
                outcome          text        NOT NULL,
                method           text        NOT NULL DEFAULT 'GET',
                is_bot           boolean     NOT NULL DEFAULT 0,
                bot_reason       text,
                is_duplicate     boolean     NOT NULL DEFAULT 0,
                is_unique        boolean     NOT NULL DEFAULT 0,
                visitor_hash     blob        NOT NULL,
                device_type      text,
                os               text,
                os_version       text,
                browser          text,
                browser_version  text,
                country          char(2),
                region           text,
                city             text,
                language         text,
                referrer_host    text,
                utm_source       text,
                utm_medium       text,
                utm_campaign     text,
                serial           text,
                source           text        NOT NULL DEFAULT 'live' CHECK (source IN ('live','v1_import')),
                PRIMARY KEY (event_id, occurred_at)
            );
        """)


def drop_scan_events(apps: Any, schema_editor: Any) -> None:
    schema_editor.execute("DROP TABLE IF EXISTS scan_events CASCADE;")


class Migration(migrations.Migration):
    initial = True

    dependencies = []

    operations = [
        migrations.RunPython(create_scan_events, drop_scan_events),
        migrations.CreateModel(
            name="ScanEvent",
            fields=[
                (
                    "pk",
                    models.CompositePrimaryKey(
                        "event_id",
                        "occurred_at",
                        blank=True,
                        editable=False,
                        primary_key=True,
                        serialize=False,
                    ),
                ),
                (
                    "event_id",
                    models.UUIDField(default=apps.core.ids.uuid7, editable=False),
                ),
                ("occurred_at", models.DateTimeField()),
                ("workspace_id", models.UUIDField()),
                ("qr_code_id", models.UUIDField()),
                ("version_id", models.UUIDField(blank=True, null=True)),
                ("campaign_id", models.UUIDField(blank=True, null=True)),
                ("domain_id", models.UUIDField()),
                ("rule_id", models.TextField(blank=True, null=True)),
                ("outcome", models.TextField()),
                ("method", models.TextField(default="GET")),
                ("is_bot", models.BooleanField(default=False)),
                ("bot_reason", models.TextField(blank=True, null=True)),
                ("is_duplicate", models.BooleanField(default=False)),
                ("is_unique", models.BooleanField(default=False)),
                ("visitor_hash", models.BinaryField()),
                ("device_type", models.TextField(blank=True, null=True)),
                ("os", models.TextField(blank=True, null=True)),
                ("os_version", models.TextField(blank=True, null=True)),
                ("browser", models.TextField(blank=True, null=True)),
                ("browser_version", models.TextField(blank=True, null=True)),
                (
                    "country",
                    apps.core.fields.FixedCharField(blank=True, max_length=2, null=True),
                ),
                ("region", models.TextField(blank=True, null=True)),
                ("city", models.TextField(blank=True, null=True)),
                ("language", models.TextField(blank=True, null=True)),
                ("referrer_host", models.TextField(blank=True, null=True)),
                ("utm_source", models.TextField(blank=True, null=True)),
                ("utm_medium", models.TextField(blank=True, null=True)),
                ("utm_campaign", models.TextField(blank=True, null=True)),
            ],
            options={
                "db_table": "scan_events",
                "managed": False,
            },
        ),
        migrations.CreateModel(
            name="ScanVisitorDaily",
            fields=[
                (
                    "pk",
                    models.CompositePrimaryKey(
                        "qr_code_id",
                        "day",
                        "visitor_hash",
                        blank=True,
                        editable=False,
                        primary_key=True,
                        serialize=False,
                    ),
                ),
                ("qr_code_id", models.UUIDField()),
                ("day", models.DateField()),
                ("visitor_hash", models.BinaryField()),
                ("first_seen_at", models.DateTimeField()),
            ],
            options={
                "db_table": "scan_visitors_daily",
            },
        ),
        migrations.CreateModel(
            name="ScanStat15m",
            fields=[
                (
                    "pk",
                    models.CompositePrimaryKey(
                        "qr_code_id",
                        "bucket_start",
                        blank=True,
                        editable=False,
                        primary_key=True,
                        serialize=False,
                    ),
                ),
                ("qr_code_id", models.UUIDField()),
                ("bucket_start", models.DateTimeField()),
                ("workspace_id", models.UUIDField()),
                ("campaign_id", models.UUIDField(blank=True, null=True)),
                ("scans", models.IntegerField(default=0)),
                ("unique_scans", models.IntegerField(default=0)),
                ("bot_hits", models.IntegerField(default=0)),
                ("blocked_hits", models.IntegerField(default=0)),
            ],
            options={
                "db_table": "scan_stats_15m",
                "indexes": [
                    models.Index(
                        fields=["workspace_id", "bucket_start"],
                        name="scan_stats_15m_ws_idx",
                    ),
                    models.Index(
                        condition=models.Q(("campaign_id__isnull", False)),
                        fields=["campaign_id", "bucket_start"],
                        name="scan_stats_15m_campaign_idx",
                    ),
                ],
            },
        ),
        migrations.CreateModel(
            name="ScanStatDailyDim",
            fields=[
                (
                    "pk",
                    models.CompositePrimaryKey(
                        "qr_code_id",
                        "day",
                        "dimension",
                        "key",
                        blank=True,
                        editable=False,
                        primary_key=True,
                        serialize=False,
                    ),
                ),
                ("qr_code_id", models.UUIDField()),
                ("day", models.DateField()),
                ("dimension", models.TextField()),
                ("key", models.TextField()),
                ("workspace_id", models.UUIDField()),
                ("campaign_id", models.UUIDField(blank=True, null=True)),
                ("scans", models.IntegerField(default=0)),
                ("unique_scans", models.IntegerField(default=0)),
            ],
            options={
                "db_table": "scan_stats_daily_dim",
                "indexes": [
                    models.Index(
                        fields=["workspace_id", "dimension", "day"],
                        name="scan_stats_dim_ws_idx",
                    )
                ],
                "constraints": [
                    models.CheckConstraint(
                        condition=models.Q(
                            (
                                "dimension__in",
                                [
                                    "country",
                                    "region",
                                    "city",
                                    "device",
                                    "os",
                                    "browser",
                                    "language",
                                    "referrer",
                                    "rule",
                                    "version",
                                    "utm_source",
                                ],
                            )
                        ),
                        name="scan_stats_daily_dim_dimension_check",
                    )
                ],
            },
        ),
    ]
