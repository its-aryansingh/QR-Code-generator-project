"use client";

import { useState } from "react";

import { useMutation, useResource, WorkspaceGate } from "@/components/enterprise/shell";
import {
  Avatar, Badge, Btn, ConfirmModal, CopyButton, EmptyState, ErrorState, Field,
  Grid, LoadingPanel, Modal, Panel, PageHeader, RoleBadge, Select, StatCard,
  TextInput, formatDate, formatNumber, formatRelative,
} from "@/components/enterprise/ui";
import { enterprise } from "@/lib/enterprise";
import { useWorkspace } from "@/lib/workspace";
import type { Invite, Member, Role } from "@/types/enterprise";

const ASSIGNABLE: Role[] = ["admin", "editor", "viewer"];

const ROLE_HELP: Record<Role, string> = {
  owner: "Full control, including billing, SSO and deleting the workspace.",
  admin: "Manage members, roles, webhooks, API keys, branding and the audit log.",
  editor: "Create and edit QR codes, campaigns, folders and lead pages.",
  viewer: "Read-only access to dashboards, analytics and exports.",
};

export default function TeamPage() {
  return (
    <WorkspaceGate>
      <Team />
    </WorkspaceGate>
  );
}

function Team() {
  const { workspaceId, atLeast, limit, usage } = useWorkspace();
  const canManage = atLeast("admin");

  const [inviteOpen, setInviteOpen] = useState(false);
  const [email, setEmail] = useState("");
  const [role, setRole] = useState<Role>("editor");
  const [lastInvite, setLastInvite] = useState<Invite | null>(null);
  const [removing, setRemoving] = useState<Member | null>(null);

  const members = useResource((id) => enterprise.listMembers(id), []);
  const invites = useResource(
    (id) => (canManage ? enterprise.listInvites(id) : Promise.resolve([] as Invite[])),
    [canManage],
  );

  const invite = useMutation(
    () => enterprise.createInvite(workspaceId!, email.trim(), role),
    {
      onSuccess: (created) => {
        setLastInvite(created);
        setEmail("");
        setInviteOpen(false);
        invites.reload();
      },
    },
  );

  const changeRole = useMutation(
    (memberId: string, next: Role) => enterprise.updateMemberRole(workspaceId!, memberId, next),
    { onSuccess: () => members.reload() },
  );

  const remove = useMutation(
    (memberId: string) => enterprise.removeMember(workspaceId!, memberId),
    { onSuccess: () => { setRemoving(null); members.reload(); } },
  );

  const revoke = useMutation(
    (inviteId: string) => enterprise.revokeInvite(workspaceId!, inviteId),
    { onSuccess: () => invites.reload() },
  );

  const pending = (invites.data ?? []).filter((row) => row.status === "pending");
  const seats = limit("max_members");
  const used = (members.data?.length ?? usage("members")) + pending.length;

  return (
    <>
      <PageHeader
        title="Team"
        description="Who has access to this workspace, and what they can do."
        actions={
          canManage && (
            <Btn variant="primary" onClick={() => setInviteOpen(true)} disabled={used >= seats}>
              + Invite member
            </Btn>
          )
        }
      />

      <Grid cols={3} className="mb-5">
        <StatCard label="Members" value={formatNumber(members.data?.length ?? 0)} />
        <StatCard label="Pending invites" value={formatNumber(pending.length)} />
        <StatCard
          label="Seats used"
          value={`${formatNumber(used)} / ${formatNumber(seats)}`}
          hint={used >= seats ? "Upgrade to add more seats" : undefined}
        />
      </Grid>

      {lastInvite?.invite_url && (
        <div className="mb-5 rounded-xl border border-emerald-900/40 bg-emerald-950/20 p-4">
          <p className="text-sm font-medium text-emerald-200">
            Invite created for {lastInvite.email}
          </p>
          <p className="mt-0.5 text-xs text-emerald-300/70">
            Send them this link — it expires {formatDate(lastInvite.expires_at)}.
          </p>
          <div className="mt-2.5 flex items-center gap-2 rounded-lg border border-emerald-900/40 bg-zinc-950 px-3 py-2">
            <code className="min-w-0 flex-1 truncate text-xs text-zinc-300">
              {lastInvite.invite_url}
            </code>
            <CopyButton value={lastInvite.invite_url} />
          </div>
          <button
            onClick={() => setLastInvite(null)}
            className="mt-2 text-xs text-emerald-300/70 underline underline-offset-2 hover:text-emerald-200"
          >
            Dismiss
          </button>
        </div>
      )}

      {members.error && <ErrorState message={members.error.message} onRetry={members.reload} />}
      {changeRole.error && <div className="mb-4"><ErrorState message={changeRole.error} /></div>}

      <div className="space-y-5">
        <Panel title="Members" description={`${members.data?.length ?? 0} people in this workspace`}>
          {members.loading && !members.data ? (
            <LoadingPanel rows={4} />
          ) : (
            <ul className="divide-y divide-zinc-800/60">
              {(members.data ?? []).map((member) => (
                <li key={member.id} className="flex flex-wrap items-center gap-3 py-3">
                  <Avatar
                    name={member.user.name}
                    email={member.user.email}
                    url={member.user.avatar_url}
                    size={36}
                  />
                  <div className="min-w-0 flex-1">
                    <p className="flex items-center gap-2 truncate text-sm text-zinc-100">
                      {member.user.name || member.user.email}
                      {member.is_you && <Badge tone="info">you</Badge>}
                    </p>
                    <p className="truncate text-xs text-zinc-600">
                      {member.user.email} · joined {formatRelative(member.joined_at)}
                    </p>
                  </div>
                  {canManage && member.role !== "owner" ? (
                    <div className="flex items-center gap-2">
                      <Select
                        value={member.role}
                        disabled={changeRole.busy}
                        onChange={(e) => changeRole.run(member.id, e.target.value as Role)}
                        className="w-32"
                      >
                        {ASSIGNABLE.map((option) => (
                          <option key={option} value={option} className="capitalize">{option}</option>
                        ))}
                      </Select>
                      <Btn
                        size="sm"
                        variant="ghost"
                        className="text-red-400 hover:bg-red-500/10"
                        onClick={() => setRemoving(member)}
                      >
                        Remove
                      </Btn>
                    </div>
                  ) : (
                    <RoleBadge role={member.role} />
                  )}
                </li>
              ))}
            </ul>
          )}
        </Panel>

        {canManage && pending.length > 0 && (
          <Panel title="Pending invites" description="Invites that have not been accepted yet">
            <ul className="divide-y divide-zinc-800/60">
              {pending.map((row) => (
                <li key={row.id} className="flex flex-wrap items-center gap-3 py-3">
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm text-zinc-200">{row.email}</p>
                    <p className="text-xs text-zinc-600">
                      Expires {formatDate(row.expires_at)}
                    </p>
                  </div>
                  <RoleBadge role={row.role} />
                  {row.invite_url && <CopyButton value={row.invite_url} label="Copy link" />}
                  <Btn
                    size="sm"
                    variant="ghost"
                    className="text-red-400 hover:bg-red-500/10"
                    loading={revoke.busy}
                    onClick={() => revoke.run(row.id)}
                  >
                    Revoke
                  </Btn>
                </li>
              ))}
            </ul>
          </Panel>
        )}

        <Panel title="What each role can do">
          <dl className="space-y-3">
            {(Object.keys(ROLE_HELP) as Role[]).map((key) => (
              <div key={key} className="flex flex-wrap items-baseline gap-3">
                <dt className="w-20 shrink-0"><RoleBadge role={key} /></dt>
                <dd className="min-w-0 flex-1 text-sm text-zinc-400">{ROLE_HELP[key]}</dd>
              </div>
            ))}
          </dl>
        </Panel>
      </div>

      <Modal
        open={inviteOpen}
        onClose={() => setInviteOpen(false)}
        title="Invite a team member"
        description="They will get a link that expires in seven days."
        footer={
          <>
            <Btn variant="ghost" onClick={() => setInviteOpen(false)}>Cancel</Btn>
            <Btn
              variant="primary"
              loading={invite.busy}
              disabled={!email.includes("@")}
              onClick={() => invite.run()}
            >
              Create invite
            </Btn>
          </>
        }
      >
        {invite.error && <div className="mb-4"><ErrorState message={invite.error} /></div>}
        <div className="space-y-4">
          <Field label="Email address" required>
            <TextInput
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="teammate@company.com"
              autoFocus
            />
          </Field>
          <Field label="Role" hint={ROLE_HELP[role]}>
            <Select value={role} onChange={(e) => setRole(e.target.value as Role)}>
              {ASSIGNABLE.map((option) => (
                <option key={option} value={option} className="capitalize">{option}</option>
              ))}
            </Select>
          </Field>
        </div>
      </Modal>

      <ConfirmModal
        open={Boolean(removing)}
        onClose={() => setRemoving(null)}
        onConfirm={() => removing && remove.run(removing.id)}
        busy={remove.busy}
        title="Remove this member?"
        message={`${removing?.user.name || removing?.user.email} will immediately lose access to every code, campaign and report in this workspace. The codes they created keep working.`}
        confirmLabel="Remove member"
      />

      {!canManage && (
        <div className="mt-5">
          <EmptyState
            title="You have read-only access to the team list"
            description="Ask an admin or the workspace owner if you need to invite people or change roles."
          />
        </div>
      )}
    </>
  );
}
