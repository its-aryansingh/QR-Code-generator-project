"""Seed system roles matching 00004_enterprise.sql lines 222-237.

Plan §6.1 rule 3 & §6.3:
5 system roles (owner, admin, editor, reviewer, analyst) with fixed UUIDs.
"""

from django.db import migrations

SYSTEM_ROLES_UP = """
INSERT INTO roles (id, org_id, key, name, description, permissions, created_at, updated_at) VALUES
 ('00000000-0000-7000-8000-00000000f001', NULL, 'owner', 'Owner', 'Everything, including billing and deletion',
  ARRAY['*'], now(), now()),
 ('00000000-0000-7000-8000-00000000f002', NULL, 'admin', 'Admin', 'Manage people, domains, keys, integrations and policies',
  ARRAY['workspace.read','workspace.update','member.manage','role.manage','qr.read','qr.create','qr.update','qr.delete',
        'qr.destination.update','qr.destination.approve','qr.design.bypass_lock','folder.manage','campaign.manage',
        'template.manage','analytics.read','analytics.export','analytics.raw','domain.manage','apikey.manage',
        'webhook.manage','integration.manage','policy.manage','audit.read','audit.export','form.manage',
        'lead.read','lead.export','pixel.manage','alert.manage','report.manage','serial.manage','gs1.manage','bulk.run'], now(), now()),
 ('00000000-0000-7000-8000-00000000f003', NULL, 'editor', 'Editor', 'Create and edit QR codes and campaigns',
  ARRAY['workspace.read','qr.read','qr.create','qr.update','qr.destination.update','folder.manage','campaign.manage',
        'template.manage','analytics.read','analytics.export','form.manage','lead.read','alert.manage','report.manage','bulk.run'], now(), now()),
 ('00000000-0000-7000-8000-00000000f004', NULL, 'reviewer', 'Reviewer', 'Approve or reject changes; read everything',
  ARRAY['workspace.read','qr.read','qr.destination.approve','analytics.read','audit.read','lead.read'], now(), now()),
 ('00000000-0000-7000-8000-00000000f005', NULL, 'analyst', 'Analyst', 'Read-only access to codes and analytics',
  ARRAY['workspace.read','qr.read','analytics.read','analytics.export'], now(), now())
ON CONFLICT (id) DO NOTHING;
"""

SYSTEM_ROLES_DOWN = """
DELETE FROM roles WHERE org_id IS NULL AND id IN (
  '00000000-0000-7000-8000-00000000f001',
  '00000000-0000-7000-8000-00000000f002',
  '00000000-0000-7000-8000-00000000f003',
  '00000000-0000-7000-8000-00000000f004',
  '00000000-0000-7000-8000-00000000f005'
);
"""


class Migration(migrations.Migration):
    dependencies = [
        ("access", "0003_initial"),
    ]

    operations = [
        migrations.RunSQL(SYSTEM_ROLES_UP, SYSTEM_ROLES_DOWN),
    ]
