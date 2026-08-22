from django.db import migrations


def add_missing_legacy_columns(apps, schema_editor):
    """Bring the pre-Django PostgreSQL tables up to the managed-model schema."""
    connection = schema_editor.connection
    existing_tables = set(connection.introspection.table_names())
    model_names = (
        "User",
        "QRRecord",
        "QRScan",
        "QRFile",
        "FreeTierUsage",
        "Workspace",
        "WorkspaceMember",
        "WorkspaceInvite",
        "Folder",
        "AuditLog",
        "Webhook",
        "WebhookLog",
        "LeadCapturePage",
        "Lead",
    )

    for model_name in model_names:
        model = apps.get_model("api", model_name)
        table_name = model._meta.db_table
        if table_name not in existing_tables:
            continue

        existing_columns = {
            column.name
            for column in connection.introspection.get_table_description(
                connection.cursor(), table_name
            )
        }
        for field in model._meta.local_fields:
            if field.primary_key or field.column in existing_columns:
                continue
            schema_editor.add_field(model, field)
            existing_columns.add(field.column)


class Migration(migrations.Migration):
    dependencies = [("api", "0001_initial")]

    operations = [migrations.RunPython(add_missing_legacy_columns, migrations.RunPython.noop)]
