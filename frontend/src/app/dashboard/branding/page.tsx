"use client";

import { useState } from "react";
import { KeyRound, Palette } from "lucide-react";

import { useMutation, useResource, WorkspaceGate } from "@/components/enterprise/shell";
import {
  Badge, Btn, ErrorState, Field, LoadingPanel, PageHeader, Panel, TextInput, Toggle,
} from "@/components/enterprise/ui";
import { enterprise } from "@/lib/enterprise";
import { useWorkspace } from "@/lib/workspace";
import type { Branding } from "@/types/enterprise";

interface BrandForm {
  brand_color: string;
  brand_logo: string;
  favicon_url: string;
  custom_footer: string;
  remove_branding: boolean;
}

function toForm(branding: Branding): BrandForm {
  return {
    brand_color: branding.brand_color || "#8B5CF6",
    brand_logo: branding.brand_logo ?? "",
    favicon_url: branding.favicon_url ?? "",
    custom_footer: branding.custom_footer ?? "",
    remove_branding: branding.remove_branding,
  };
}

export default function BrandingPage() {
  return (
    <WorkspaceGate>
      <BrandSettings />
    </WorkspaceGate>
  );
}

function BrandSettings() {
  const { workspaceId, atLeast } = useWorkspace();
  const canEdit = atLeast("admin");
  const brand = useResource((id) => enterprise.getBranding(id), []);
  const sso = useResource((id) => enterprise.getSso(id), []);
  const [draft, setForm] = useState<BrandForm | null>(null);
  const [saved, setSaved] = useState(false);
  const form = draft ?? (brand.data ? toForm(brand.data) : null);

  const save = useMutation(
    () =>
      enterprise.updateBranding(workspaceId!, {
        brand_color: form!.brand_color,
        brand_logo: form!.brand_logo.trim() || null,
        favicon_url: form!.favicon_url.trim() || null,
        custom_footer: form!.custom_footer.trim() || null,
        remove_branding: form!.remove_branding,
      }),
    {
      onSuccess: (result) => {
        brand.setData(result);
        setForm(null);
        setSaved(true);
        window.setTimeout(() => setSaved(false), 3000);
      },
    },
  );

  const validColor = /^#[0-9a-fA-F]{6}$/.test(form?.brand_color ?? "");

  return (
    <>
      <PageHeader
        title="Brand & domains"
        description="How your workspace looks on hosted pages, and how members sign in."
      />

      <div className="space-y-5">
        <Panel
          title="Brand"
          description="Colour, logo and footer used on lead pages and other hosted pages."
          actions={<Palette className="size-4 text-zinc-400" aria-hidden />}
        >
          {brand.error ? (
            <ErrorState message={brand.error.message} onRetry={brand.reload} />
          ) : !form ? (
            <LoadingPanel rows={4} />
          ) : (
            <div className="space-y-5">
              {save.error && <ErrorState message={save.error} />}
              {saved && (
                <p role="status" className="rounded-lg border border-emerald-800/60 bg-emerald-950/30 px-3 py-2 text-sm text-emerald-300">
                  Branding saved.
                </p>
              )}
              <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
                <Field label="Brand colour" error={validColor ? undefined : "Use a #RRGGBB colour"}>
                  <div className="flex items-center gap-2">
                    <input
                      type="color"
                      value={validColor ? form.brand_color : "#8B5CF6"}
                      onChange={(e) => setForm({ ...form, brand_color: e.target.value })}
                      disabled={!canEdit}
                      aria-label="Pick brand colour"
                      className="h-10 w-12 shrink-0 cursor-pointer rounded-lg border border-zinc-700 bg-transparent"
                    />
                    <TextInput
                      value={form.brand_color}
                      onChange={(e) => setForm({ ...form, brand_color: e.target.value })}
                      disabled={!canEdit}
                      aria-label="Brand colour hex value"
                    />
                  </div>
                </Field>
                <Field label="Logo URL" hint="An https:// link to a PNG or SVG.">
                  <TextInput
                    value={form.brand_logo}
                    onChange={(e) => setForm({ ...form, brand_logo: e.target.value })}
                    placeholder="https://cdn.example.com/logo.png"
                    disabled={!canEdit}
                  />
                </Field>
                <Field label="Favicon URL">
                  <TextInput
                    value={form.favicon_url}
                    onChange={(e) => setForm({ ...form, favicon_url: e.target.value })}
                    placeholder="https://cdn.example.com/favicon.ico"
                    disabled={!canEdit}
                  />
                </Field>
                <Field label="Footer text">
                  <TextInput
                    value={form.custom_footer}
                    onChange={(e) => setForm({ ...form, custom_footer: e.target.value })}
                    placeholder="© 2026 Your Company"
                    maxLength={300}
                    disabled={!canEdit}
                  />
                </Field>
              </div>

              <Toggle
                checked={form.remove_branding}
                onChange={(next) => setForm({ ...form, remove_branding: next })}
                disabled={!canEdit || !brand.data?.can_white_label}
                label="Remove “Powered by QRit”"
                description={
                  brand.data?.can_white_label
                    ? "Hide QRit branding on hosted pages."
                    : "Available on the Business plan."
                }
              />

              <BrandPreview form={form} />

              {canEdit && (
                <div className="flex justify-end">
                  <Btn variant="primary" onClick={() => save.run()} loading={save.busy} disabled={!validColor}>
                    Save branding
                  </Btn>
                </div>
              )}
            </div>
          )}
        </Panel>

        <Panel
          title="Single sign-on"
          description="SAML or OIDC sign-in for your organisation."
          actions={<KeyRound className="size-4 text-zinc-400" aria-hidden />}
        >
          {sso.error ? (
            <ErrorState message={sso.error.message} onRetry={sso.reload} />
          ) : !sso.data ? (
            <LoadingPanel rows={2} />
          ) : (
            <div className="space-y-3 text-sm">
              <div className="flex flex-wrap items-center gap-2">
                <span className="text-zinc-300">Status</span>
                <Badge tone={sso.data.is_enabled ? "success" : "neutral"}>
                  {sso.data.is_enabled ? `${sso.data.provider.toUpperCase()} enabled` : "Not configured"}
                </Badge>
              </div>
              {sso.data.available === false && (
                <p className="text-zinc-400">
                  Single sign-on setup isn&apos;t available on this server yet. Members sign in with
                  email and password, Google or GitHub.
                </p>
              )}
            </div>
          )}
        </Panel>
      </div>
    </>
  );
}

function BrandPreview({ form }: { form: BrandForm }) {
  const color = /^#[0-9a-fA-F]{6}$/.test(form.brand_color) ? form.brand_color : "#8B5CF6";
  return (
    <div className="border-t border-zinc-800/70 pt-4">
      <p className="mb-3 text-xs font-medium uppercase tracking-wide text-zinc-400">Preview</p>
      {/* The brand colour is chosen by the user, so it can't be a static utility class. */}
      <div className="rounded-lg border border-zinc-800 border-t-[3px] bg-zinc-950 p-5" style={{ borderTopColor: color }}>
        <div className="mb-4 flex items-center gap-3">
          {form.brand_logo.startsWith("https://") ? (
            // eslint-disable-next-line @next/next/no-img-element
            <img src={form.brand_logo} alt="Logo preview" className="size-8 rounded object-cover" />
          ) : (
            <div className="flex size-8 items-center justify-center rounded text-sm font-bold text-zinc-50" style={{ backgroundColor: color }}>
              Q
            </div>
          )}
          <span className="font-semibold text-zinc-100">Your QR experience</span>
        </div>
        <div className="flex h-20 items-center justify-center rounded-lg bg-zinc-900 text-sm text-zinc-400">
          Page content
        </div>
        {!form.remove_branding && <p className="mt-3 text-center text-xs text-zinc-400">Powered by QRit</p>}
        {form.custom_footer && <p className="mt-2 text-center text-xs text-zinc-400">{form.custom_footer}</p>}
      </div>
    </div>
  );
}
