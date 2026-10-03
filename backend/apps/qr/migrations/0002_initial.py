from typing import Any

import django.db.models.deletion
from django.conf import settings
from django.db import migrations, models


def create_trgm_index(apps: Any, schema_editor: Any) -> None:
    if schema_editor.connection.vendor == "postgresql":
        schema_editor.execute(
            "CREATE INDEX IF NOT EXISTS qr_codes_name_trgm_idx ON qr_codes USING gin (name gin_trgm_ops);"
        )


def drop_trgm_index(apps: Any, schema_editor: Any) -> None:
    if schema_editor.connection.vendor == "postgresql":
        schema_editor.execute("DROP INDEX IF EXISTS qr_codes_name_trgm_idx;")


class Migration(migrations.Migration):
    initial = True

    dependencies = [
        ("qr", "0001_initial"),
        ("workspaces", "0001_initial"),
        migrations.swappable_dependency(settings.AUTH_USER_MODEL),
    ]

    operations = [
        migrations.RunPython(create_trgm_index, drop_trgm_index),
        migrations.AddField(
            model_name="campaign",
            name="workspace",
            field=models.ForeignKey(
                db_column="workspace_id",
                on_delete=django.db.models.deletion.DB_CASCADE,
                related_name="campaigns",
                to="workspaces.workspace",
            ),
        ),
        migrations.AddField(
            model_name="domain",
            name="workspace",
            field=models.ForeignKey(
                blank=True,
                db_column="workspace_id",
                null=True,
                on_delete=django.db.models.deletion.DB_CASCADE,
                related_name="domains",
                to="workspaces.workspace",
            ),
        ),
        migrations.AddField(
            model_name="file",
            name="created_by",
            field=models.ForeignKey(
                blank=True,
                db_column="created_by",
                null=True,
                on_delete=django.db.models.deletion.DB_SET_NULL,
                related_name="uploaded_files",
                to=settings.AUTH_USER_MODEL,
            ),
        ),
        migrations.AddField(
            model_name="file",
            name="workspace",
            field=models.ForeignKey(
                db_column="workspace_id",
                on_delete=django.db.models.deletion.DB_CASCADE,
                related_name="files",
                to="workspaces.workspace",
            ),
        ),
        migrations.AddField(
            model_name="folder",
            name="parent",
            field=models.ForeignKey(
                blank=True,
                db_column="parent_id",
                null=True,
                on_delete=django.db.models.deletion.DB_CASCADE,
                related_name="children",
                to="qr.folder",
            ),
        ),
        migrations.AddField(
            model_name="folder",
            name="workspace",
            field=models.ForeignKey(
                db_column="workspace_id",
                on_delete=django.db.models.deletion.DB_CASCADE,
                related_name="folders",
                to="workspaces.workspace",
            ),
        ),
        migrations.AddField(
            model_name="qrcode",
            name="campaign",
            field=models.ForeignKey(
                blank=True,
                db_column="campaign_id",
                null=True,
                on_delete=django.db.models.deletion.DB_SET_NULL,
                related_name="qr_codes",
                to="qr.campaign",
            ),
        ),
        migrations.AddField(
            model_name="qrcode",
            name="created_by",
            field=models.ForeignKey(
                blank=True,
                db_column="created_by",
                null=True,
                on_delete=django.db.models.deletion.DB_SET_NULL,
                related_name="created_qr_codes",
                to=settings.AUTH_USER_MODEL,
            ),
        ),
        migrations.AddField(
            model_name="qrcode",
            name="domain",
            field=models.ForeignKey(
                blank=True,
                db_column="domain_id",
                null=True,
                on_delete=django.db.models.deletion.DO_NOTHING,
                related_name="qr_codes",
                to="qr.domain",
            ),
        ),
        migrations.AddField(
            model_name="qrcode",
            name="folder",
            field=models.ForeignKey(
                blank=True,
                db_column="folder_id",
                null=True,
                on_delete=django.db.models.deletion.DB_SET_NULL,
                related_name="qr_codes",
                to="qr.folder",
            ),
        ),
        migrations.AddField(
            model_name="qrcode",
            name="workspace",
            field=models.ForeignKey(
                db_column="workspace_id",
                on_delete=django.db.models.deletion.DB_CASCADE,
                related_name="qr_codes",
                to="workspaces.workspace",
            ),
        ),
        migrations.AddField(
            model_name="qrcodetag",
            name="qr_code",
            field=models.ForeignKey(
                db_column="qr_code_id",
                on_delete=django.db.models.deletion.DB_CASCADE,
                related_name="code_tags",
                to="qr.qrcode",
            ),
        ),
        migrations.AddField(
            model_name="qrversion",
            name="created_by",
            field=models.ForeignKey(
                blank=True,
                db_column="created_by",
                null=True,
                on_delete=django.db.models.deletion.DB_SET_NULL,
                related_name="created_versions",
                to=settings.AUTH_USER_MODEL,
            ),
        ),
        migrations.AddField(
            model_name="qrversion",
            name="qr_code",
            field=models.ForeignKey(
                db_column="qr_code_id",
                on_delete=django.db.models.deletion.DB_CASCADE,
                related_name="versions",
                to="qr.qrcode",
            ),
        ),
        migrations.AddField(
            model_name="qrversion",
            name="restored_from",
            field=models.ForeignKey(
                blank=True,
                db_column="restored_from",
                null=True,
                on_delete=django.db.models.deletion.DO_NOTHING,
                related_name="restored_to",
                to="qr.qrversion",
            ),
        ),
        migrations.AddField(
            model_name="qrcode",
            name="current_version",
            field=models.ForeignKey(
                blank=True,
                db_column="current_version_id",
                null=True,
                on_delete=django.db.models.deletion.DO_NOTHING,
                related_name="current_for_qr_codes",
                to="qr.qrversion",
            ),
        ),
        migrations.AddField(
            model_name="shortcodetombstone",
            name="domain",
            field=models.ForeignKey(
                db_column="domain_id",
                on_delete=django.db.models.deletion.DO_NOTHING,
                related_name="tombstones",
                to="qr.domain",
            ),
        ),
        migrations.AddField(
            model_name="tag",
            name="workspace",
            field=models.ForeignKey(
                db_column="workspace_id",
                on_delete=django.db.models.deletion.DB_CASCADE,
                related_name="tags",
                to="workspaces.workspace",
            ),
        ),
        migrations.AddField(
            model_name="qrcodetag",
            name="tag",
            field=models.ForeignKey(
                db_column="tag_id",
                on_delete=django.db.models.deletion.DB_CASCADE,
                related_name="tagged_codes",
                to="qr.tag",
            ),
        ),
        migrations.AddField(
            model_name="template",
            name="created_by",
            field=models.ForeignKey(
                blank=True,
                db_column="created_by",
                null=True,
                on_delete=django.db.models.deletion.DB_SET_NULL,
                related_name="created_templates",
                to=settings.AUTH_USER_MODEL,
            ),
        ),
        migrations.AddField(
            model_name="template",
            name="workspace",
            field=models.ForeignKey(
                db_column="workspace_id",
                on_delete=django.db.models.deletion.DB_CASCADE,
                related_name="templates",
                to="workspaces.workspace",
            ),
        ),
        migrations.AddField(
            model_name="qrcode",
            name="template",
            field=models.ForeignKey(
                blank=True,
                db_column="template_id",
                null=True,
                on_delete=django.db.models.deletion.DB_SET_NULL,
                related_name="qr_codes",
                to="qr.template",
            ),
        ),
        migrations.AddIndex(
            model_name="campaign",
            index=models.Index(fields=["workspace", "status"], name="campaigns_ws_status_idx"),
        ),
        migrations.AddConstraint(
            model_name="campaign",
            constraint=models.CheckConstraint(
                condition=models.Q(
                    ("status__in", ["draft", "active", "paused", "ended", "archived"])
                ),
                name="campaigns_status_check",
            ),
        ),
        migrations.AddConstraint(
            model_name="campaign",
            constraint=models.CheckConstraint(
                condition=models.Q(
                    ("goal_scans__isnull", True), ("goal_scans__gt", 0), _connector="OR"
                ),
                name="campaigns_goal_scans_check",
            ),
        ),
        migrations.AddIndex(
            model_name="domain",
            index=models.Index(fields=["workspace"], name="domains_workspace_idx"),
        ),
        migrations.AddConstraint(
            model_name="domain",
            constraint=models.CheckConstraint(
                condition=models.Q(
                    (
                        "status__in",
                        ["pending", "verifying", "active", "failed", "disabled"],
                    )
                ),
                name="domains_status_check",
            ),
        ),
        migrations.AddConstraint(
            model_name="domain",
            constraint=models.CheckConstraint(
                condition=models.Q(("tls_status__in", ["pending", "active", "failed"])),
                name="domains_tls_status_check",
            ),
        ),
        migrations.AddIndex(
            model_name="file",
            index=models.Index(fields=["workspace", "purpose"], name="files_ws_purpose_idx"),
        ),
        migrations.AddConstraint(
            model_name="file",
            constraint=models.CheckConstraint(
                condition=models.Q(
                    (
                        "purpose__in",
                        ["logo", "hosted_asset", "export", "bulk_input", "bulk_output"],
                    )
                ),
                name="files_purpose_check",
            ),
        ),
        migrations.AddConstraint(
            model_name="file",
            constraint=models.CheckConstraint(
                condition=models.Q(("size_bytes__gte", 0)),
                name="files_size_bytes_check",
            ),
        ),
        migrations.AddConstraint(
            model_name="folder",
            constraint=models.UniqueConstraint(
                fields=("workspace", "parent", "name"),
                name="folders_workspace_id_parent_id_name_key",
                nulls_distinct=False,
            ),
        ),
        migrations.AddIndex(
            model_name="qrversion",
            index=models.Index(
                fields=["qr_code", "effective_at", "version_no"],
                name="qr_versions_effective_idx",
            ),
        ),
        migrations.AddConstraint(
            model_name="qrversion",
            constraint=models.UniqueConstraint(
                fields=("qr_code", "version_no"),
                name="qr_versions_qr_code_id_version_no_key",
            ),
        ),
        migrations.AddConstraint(
            model_name="qrversion",
            constraint=models.CheckConstraint(
                condition=models.Q(("version_no__gt", 0)),
                name="qr_versions_version_no_check",
            ),
        ),
        migrations.AddConstraint(
            model_name="qrversion",
            constraint=models.CheckConstraint(
                condition=models.Q(("destination_kind__in", ["url", "hosted_page"])),
                name="qr_versions_destination_kind_check",
            ),
        ),
        migrations.AddConstraint(
            model_name="qrversion",
            constraint=models.CheckConstraint(
                condition=models.Q(
                    ("safety_status__in", ["pending", "safe", "flagged", "blocked"])
                ),
                name="qr_versions_safety_status_check",
            ),
        ),
        migrations.AddConstraint(
            model_name="qrversion",
            constraint=models.CheckConstraint(
                condition=models.Q(
                    models.Q(
                        ("destination_kind", "url"),
                        ("destination_url__isnull", False),
                        ("hosted_page__isnull", True),
                    ),
                    models.Q(
                        ("destination_kind", "hosted_page"),
                        ("hosted_page__isnull", False),
                    ),
                    _connector="OR",
                ),
                name="qr_version_destination",
            ),
        ),
        migrations.AddConstraint(
            model_name="tag",
            constraint=models.UniqueConstraint(
                fields=("workspace", "name"), name="tags_workspace_id_name_key"
            ),
        ),
        migrations.AddIndex(
            model_name="qrcodetag",
            index=models.Index(fields=["tag"], name="qr_code_tags_tag_idx"),
        ),
        migrations.AddConstraint(
            model_name="template",
            constraint=models.UniqueConstraint(
                condition=models.Q(("is_default", True)),
                fields=("workspace",),
                name="templates_one_default_uniq",
            ),
        ),
        migrations.AddIndex(
            model_name="qrcode",
            index=models.Index(
                condition=models.Q(("deleted_at__isnull", True)),
                fields=["workspace", "created_at", "id"],
                name="qr_codes_ws_list_idx",
            ),
        ),
        migrations.AddIndex(
            model_name="qrcode",
            index=models.Index(
                condition=models.Q(("deleted_at__isnull", True)),
                fields=["workspace", "folder"],
                name="qr_codes_ws_folder_idx",
            ),
        ),
        migrations.AddIndex(
            model_name="qrcode",
            index=models.Index(
                condition=models.Q(("deleted_at__isnull", True)),
                fields=["workspace", "campaign"],
                name="qr_codes_ws_campaign_idx",
            ),
        ),
        migrations.AddConstraint(
            model_name="qrcode",
            constraint=models.CheckConstraint(
                condition=models.Q(("mode__in", ["static", "dynamic"])),
                name="qr_codes_mode_check",
            ),
        ),
        migrations.AddConstraint(
            model_name="qrcode",
            constraint=models.CheckConstraint(
                condition=models.Q(
                    (
                        "content_type__in",
                        [
                            "url",
                            "text",
                            "email",
                            "phone",
                            "sms",
                            "whatsapp",
                            "wifi",
                            "vcard",
                            "event",
                            "upi",
                            "location",
                            "links_page",
                            "file",
                            "app_store",
                            "gs1",
                        ],
                    )
                ),
                name="qr_codes_content_type_check",
            ),
        ),
        migrations.AddConstraint(
            model_name="qrcode",
            constraint=models.CheckConstraint(
                condition=models.Q(("status__in", ["active", "paused", "archived", "blocked"])),
                name="qr_codes_status_check",
            ),
        ),
        migrations.AddConstraint(
            model_name="qrcode",
            constraint=models.CheckConstraint(
                condition=models.Q(
                    ("safety_status__in", ["pending", "safe", "flagged", "blocked"])
                ),
                name="qr_codes_safety_status_check",
            ),
        ),
        migrations.AddConstraint(
            model_name="qrcode",
            constraint=models.CheckConstraint(
                condition=models.Q(
                    ("scan_limit__isnull", True), ("scan_limit__gt", 0), _connector="OR"
                ),
                name="qr_codes_scan_limit_check",
            ),
        ),
        migrations.AddConstraint(
            model_name="qrcode",
            constraint=models.CheckConstraint(
                condition=models.Q(
                    models.Q(
                        ("domain__isnull", False),
                        ("mode", "dynamic"),
                        ("short_code__isnull", False),
                        ("static_payload__isnull", True),
                    ),
                    models.Q(
                        ("mode", "static"),
                        ("short_code__isnull", True),
                        ("static_payload__isnull", False),
                    ),
                    _connector="OR",
                ),
                name="qr_dynamic_has_link",
            ),
        ),
        migrations.AddConstraint(
            model_name="qrcode",
            constraint=models.CheckConstraint(
                condition=models.Q(
                    ("mode", "dynamic"),
                    models.Q(
                        (
                            "content_type__in",
                            ["links_page", "file", "app_store", "gs1"],
                        ),
                        _negated=True,
                    ),
                    _connector="OR",
                ),
                name="qr_static_types",
            ),
        ),
        migrations.AddConstraint(
            model_name="qrcode",
            constraint=models.CheckConstraint(
                condition=models.Q(
                    ("mode", "static"),
                    models.Q(
                        ("content_type__in", ["wifi", "upi", "text", "location"]),
                        _negated=True,
                    ),
                    _connector="OR",
                ),
                name="qr_dynamic_types",
            ),
        ),
        migrations.AddConstraint(
            model_name="qrcode",
            constraint=models.UniqueConstraint(
                condition=models.Q(("short_code__isnull", False)),
                fields=("domain", "short_code"),
                name="qr_codes_domain_code_uniq",
            ),
        ),
        migrations.AddConstraint(
            model_name="qrcode",
            constraint=models.UniqueConstraint(
                condition=models.Q(("legacy_short_code__isnull", False)),
                fields=("legacy_short_code",),
                name="qr_codes_legacy_code_uniq",
            ),
        ),
        migrations.AddConstraint(
            model_name="qrcode",
            constraint=models.UniqueConstraint(
                condition=models.Q(("gs1_gtin__isnull", False)),
                fields=("domain", "gs1_gtin"),
                name="qr_codes_domain_gtin_uniq",
            ),
        ),
    ]
