"use client";

/**
 * Create a QR code on one screen: choose what it does, fill in the details,
 * style it, create. The code on the right is the exact code that will be
 * printed, redrawn as you type.
 */
import Link from "next/link";
import { useCallback, useEffect, useId, useMemo, useRef, useState } from "react";
import { Check, ChevronDown, Copy, Download, ImagePlus, Loader2, Plus, X } from "lucide-react";

import {
  DEFAULT_FIELDS, QR_TYPES, TRACKABLE, TYPE_BY_ID, buildContent, suggestName,
  type FieldDef, type Fields, type QrTypeId,
} from "@/components/create/qr-types";
import {
  DEFAULT_STYLE, QrProof, QrThumb, downloadQr,
  type CornerStyle, type DotStyle, type QrStyle,
} from "@/components/create/qr-proof";
import { getContrastRatio } from "@/components/qr-scannability";
import { ApiError, enterprise } from "@/lib/enterprise";
import { useWorkspace, useWorkspaceStore } from "@/lib/workspace";
import type { Folder, WorkspaceQR } from "@/types/enterprise";

const INKS = [
  { name: "Ink", value: "#111827" },
  { name: "Violet", value: "#5B21B6" },
  { name: "Ocean", value: "#0E4A7B" },
  { name: "Forest", value: "#14532D" },
  { name: "Wine", value: "#7F1D3B" },
];

const DOTS: { id: DotStyle; label: string }[] = [
  { id: "square", label: "Square" },
  { id: "rounded", label: "Rounded" },
  { id: "dots", label: "Dots" },
  { id: "classy-rounded", label: "Soft" },
];

const CORNERS: { id: CornerStyle; label: string }[] = [
  { id: "square", label: "Square" },
  { id: "extra-rounded", label: "Rounded" },
  { id: "dot", label: "Circle" },
];

const MAX_LOGO_BYTES = 300 * 1024;

const field =
  "w-full rounded-lg border border-zinc-800 bg-zinc-900/70 px-3.5 py-2.5 text-[15px] text-zinc-100 placeholder:text-zinc-600 " +
  "transition-colors hover:border-zinc-700 focus:border-violet-500 focus:outline-none focus:ring-2 focus:ring-violet-500/25";
const fieldError = "border-red-500/70 focus:border-red-500 focus:ring-red-500/20";

function flattenFolders(folders: Folder[], depth = 0): { id: string; name: string }[] {
  return folders.flatMap((f) => [
    { id: f.id, name: `${"  ".repeat(depth)}${f.name}` },
    ...flattenFolders((f as Folder & { children?: Folder[] }).children ?? [], depth + 1),
  ]);
}

export default function CreateQRPage() {
  const { workspaceId, workspace, entitlements, atLeast, loaded } = useWorkspace();
  const loadWorkspaces = useWorkspaceStore((s) => s.load);
  const refreshEntitlements = useWorkspaceStore((s) => s.refreshEntitlements);

  const [type, setType] = useState<QrTypeId>("url");
  const [values, setValues] = useState<Record<QrTypeId, Fields>>(() => structuredClone(DEFAULT_FIELDS));
  const [name, setName] = useState("");
  const [tracked, setTracked] = useState(true);
  const [style, setStyle] = useState<QrStyle>(DEFAULT_STYLE);
  const [moreOpen, setMoreOpen] = useState(false);
  const [folderId, setFolderId] = useState("");
  const [folders, setFolders] = useState<{ id: string; name: string }[]>([]);
  const [password, setPassword] = useState("");
  const [expiresAt, setExpiresAt] = useState("");
  const [maxScans, setMaxScans] = useState("");

  const [showErrors, setShowErrors] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [serverError, setServerError] = useState<{ field?: string; message: string; upgrade?: boolean } | null>(null);
  const [created, setCreated] = useState<WorkspaceQR | null>(null);
  const [copied, setCopied] = useState(false);
  const [host, setHost] = useState("");

  const fieldRefs = useRef<Record<string, HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement | null>>({});
  const logoInput = useRef<HTMLInputElement>(null);
  const formId = useId();

  useEffect(() => {
    void loadWorkspaces();
    setHost(window.location.host);
  }, [loadWorkspaces]);

  useEffect(() => {
    if (!workspaceId) return;
    enterprise.listFolders(workspaceId).then((f) => setFolders(flattenFolders(f))).catch(() => setFolders([]));
  }, [workspaceId]);

  const def = TYPE_BY_ID[type];
  const fields = values[type];
  const built = useMemo(() => buildContent(type, fields), [type, fields]);
  const canTrack = TRACKABLE.has(type);
  const dynamic = canTrack && tracked;
  const suggestion = suggestName(type, fields);
  const contrast = style.background === "transparent" ? 21 : getContrastRatio(style.color, style.background);
  const used = entitlements?.usage?.qr_codes;
  const limit = entitlements?.limits?.max_qr_codes;
  const canCreate = !loaded || atLeast("editor");

  // What the printed code will contain. Before a tracked code exists its
  // short link isn't known yet; a stand-in of the same length gives the same
  // density, so the preview looks like the final code.
  const proofData = created
    ? (created.is_dynamic && created.short_url) || created.content
    : dynamic && !built.error
      ? `https://${host || "qrit.app"}/r/0000000`
      : built.content;
  const isPlaceholder = !created && (!built.content || (type === "url" && !!built.error));

  const errorFor = (key: string): string | undefined => {
    if (serverError?.field === key) return serverError.message;
    if (showErrors && built.error?.field === key) return built.error.message;
    return undefined;
  };

  const setField = (key: string, value: string) => {
    setValues((prev) => ({ ...prev, [type]: { ...prev[type], [key]: value } }));
    if (serverError?.field) setServerError(null);
  };

  const pickType = (next: QrTypeId) => {
    setType(next);
    setShowErrors(false);
    setServerError(null);
    window.requestAnimationFrame(() => fieldRefs.current[TYPE_BY_ID[next].fields[0].key]?.focus());
  };

  const onLogo = (file: File | undefined) => {
    if (!file) return;
    if (!file.type.startsWith("image/")) return setServerError({ message: "The logo must be an image (PNG, JPG or SVG)." });
    if (file.size > MAX_LOGO_BYTES) return setServerError({ message: "Use a logo under 300 KB. A small square PNG works best." });
    const reader = new FileReader();
    reader.onload = () => setStyle((s) => ({ ...s, logo: String(reader.result) }));
    reader.readAsDataURL(file);
  };

  const submit = useCallback(async () => {
    if (submitting || created) return;
    setShowErrors(true);
    setServerError(null);
    if (built.error) {
      fieldRefs.current[built.error.field]?.focus();
      return;
    }
    if (!workspaceId) {
      setServerError({ message: "Your workspace is still loading. Try again in a moment." });
      return;
    }
    setSubmitting(true);
    const body: Record<string, unknown> = {
      title: name.trim() || suggestion || def.label,
      content: built.content,
      qr_type: type,
      size: 1024,
      is_dynamic: dynamic,
      folder_id: folderId || undefined,
      customization: {
        foreground_color: style.color,
        background_color: style.background,
        body_style: style.dots,
        corner_style: style.corners,
        ...(style.logo ? { logo: { url: style.logo, size: 0.32 } } : {}),
      },
    };
    if (dynamic && password) body.password = password;
    if (dynamic && expiresAt) body.expires_at = new Date(expiresAt).toISOString();
    if (dynamic && maxScans) body.max_scans = Number(maxScans);
    try {
      const qr = await enterprise.createQR(workspaceId, body);
      setCreated(qr);
      void refreshEntitlements();
    } catch (error) {
      if (error instanceof ApiError) {
        if (error.code === "invalid_url" || error.code === "content_required") {
          setServerError({ field: def.fields[0].key, message: error.message });
        } else if (error.isUpgradeRequired) {
          setServerError({ message: error.message, upgrade: true });
        } else {
          setServerError({ message: error.message });
        }
      } else {
        setServerError({ message: "Couldn't reach the server. Check your connection and try again." });
      }
    } finally {
      setSubmitting(false);
    }
  }, [submitting, created, built, workspaceId, name, suggestion, def, type, dynamic, folderId, style, password, expiresAt, maxScans, refreshEntitlements]);

  // Ctrl/⌘ + Enter creates from anywhere on the page, textareas included.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
        e.preventDefault();
        void submit();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [submit]);

  const startOver = () => {
    setCreated(null);
    setValues(structuredClone(DEFAULT_FIELDS));
    setName("");
    setPassword("");
    setExpiresAt("");
    setMaxScans("");
    setShowErrors(false);
    setServerError(null);
    window.requestAnimationFrame(() => fieldRefs.current[def.fields[0].key]?.focus());
  };

  const copyLink = async (text: string) => {
    await navigator.clipboard.writeText(text);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1600);
  };

  const fileName = (created?.title || name || suggestion || "qr-code").slice(0, 60);

  const renderField = (f: FieldDef, index: number) => {
    const id = `${formId}-${f.key}`;
    const err = errorFor(f.key);
    const common = {
      id,
      value: fields[f.key] ?? "",
      "aria-invalid": err ? true : undefined,
      "aria-describedby": err ? `${id}-error` : undefined,
      className: `${field} ${err ? fieldError : ""}`,
      placeholder: f.placeholder,
      autoComplete: f.autoComplete,
      autoFocus: index === 0 && type === "url",
    };
    return (
      <div key={f.key} className={f.wide || f.multiline ? "sm:col-span-2" : ""}>
        <label htmlFor={id} className="mb-1.5 block text-[13px] font-medium text-zinc-300">
          {f.label}
        </label>
        {f.options ? (
          <select
            {...common}
            ref={(el) => { fieldRefs.current[f.key] = el; }}
            onChange={(e) => setField(f.key, e.target.value)}
          >
            {f.options.map((o) => <option key={o.value} value={o.value}>{o.label}</option>)}
          </select>
        ) : f.multiline ? (
          <textarea
            {...common}
            rows={3}
            ref={(el) => { fieldRefs.current[f.key] = el; }}
            onChange={(e) => setField(f.key, e.target.value)}
            className={`${common.className} resize-y min-h-[84px]`}
          />
        ) : (
          <input
            {...common}
            type={f.input === "url" ? "text" : f.input ?? "text"}
            inputMode={f.input === "url" ? "url" : undefined}
            spellCheck={f.input === "url" ? false : undefined}
            ref={(el) => { fieldRefs.current[f.key] = el; }}
            onChange={(e) => setField(f.key, e.target.value)}
          />
        )}
        {err && (
          <p id={`${id}-error`} role="alert" className="mt-1.5 text-[13px] text-red-400">
            {err}
          </p>
        )}
      </div>
    );
  };

  const createButton = (extra = "") => (
    <button
      type="submit"
      form={formId}
      disabled={submitting || !canCreate}
      className={`inline-flex h-11 items-center justify-center gap-2 rounded-xl bg-violet-600 px-5 text-[15px] font-semibold text-white shadow-[inset_0_1px_0_rgba(255,255,255,0.18)] transition-colors hover:bg-violet-500 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-violet-400 focus-visible:ring-offset-2 focus-visible:ring-offset-zinc-950 disabled:cursor-not-allowed disabled:opacity-50 ${extra}`}
    >
      {submitting ? <Loader2 size={17} className="animate-spin" /> : null}
      {submitting ? "Creating" : "Create QR code"}
    </button>
  );

  const shortLink = created?.is_dynamic ? created.short_url : null;

  return (
    <div className="mx-auto w-full max-w-[1120px] pb-28 lg:pb-10">
      <header className="mb-7 flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-[26px] font-semibold tracking-[-0.02em] text-zinc-50">
            {created ? "Your QR code is ready" : "New QR code"}
          </h1>
          <p className="mt-1 text-[15px] text-zinc-400">
            {created
              ? created.is_dynamic
                ? "Download it for print or copy the link. You can change where it sends people at any time."
                : "Download it for print. Everything it needs is inside the code."
              : "Fill in what it should open. Your code updates as you type."}
          </p>
        </div>
        {typeof used === "number" && typeof limit === "number" && (
          <p className="text-[13px] text-zinc-500">
            {used} of {limit} codes used{workspace ? ` in ${workspace.name}` : ""}
          </p>
        )}
      </header>

      <div className="grid gap-8 lg:grid-cols-[minmax(0,1fr)_380px] lg:gap-12">
        {/* ------------------------------------------------------------ form */}
        {created ? (
          <section className="motion-safe:animate-[fadein_200ms_ease-out] space-y-6">
            <div className="rounded-2xl border border-zinc-800 bg-zinc-900/50 p-6">
              <p className="text-[13px] font-medium text-zinc-400">{TYPE_BY_ID[type].label}</p>
              <h2 className="mt-1 text-xl font-semibold text-zinc-50">{created.title}</h2>
              <dl className="mt-5 space-y-3 text-[15px]">
                {shortLink && (
                  <div>
                    <dt className="text-[13px] text-zinc-500">Printed link</dt>
                    <dd className="mt-1 flex items-center gap-2">
                      <a href={shortLink} target="_blank" rel="noreferrer" className="truncate text-violet-300 hover:text-violet-200 underline-offset-4 hover:underline">
                        {shortLink.replace(/^https?:\/\//, "")}
                      </a>
                      <button
                        type="button"
                        onClick={() => copyLink(shortLink)}
                        className="inline-flex h-8 shrink-0 items-center gap-1.5 rounded-lg border border-zinc-700 px-2.5 text-[13px] text-zinc-200 hover:bg-zinc-800"
                      >
                        {copied ? <Check size={14} /> : <Copy size={14} />}
                        {copied ? "Copied" : "Copy"}
                      </button>
                    </dd>
                  </div>
                )}
                <div>
                  <dt className="text-[13px] text-zinc-500">{created.is_dynamic ? "Sends people to" : "Contains"}</dt>
                  <dd className="mt-1 break-words text-zinc-200 whitespace-pre-line line-clamp-4">{created.content}</dd>
                </div>
              </dl>
            </div>
            <div className="flex flex-wrap gap-3">
              <button
                type="button"
                onClick={startOver}
                className="inline-flex h-11 items-center gap-2 rounded-xl bg-violet-600 px-5 text-[15px] font-semibold text-white hover:bg-violet-500 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-violet-400"
              >
                <Plus size={17} /> Create another
              </button>
              <Link
                href={`/dashboard/qr-codes/${created.id}`}
                className="inline-flex h-11 items-center rounded-xl border border-zinc-700 px-5 text-[15px] font-medium text-zinc-200 hover:bg-zinc-800"
              >
                Open details
              </Link>
              <Link href="/dashboard/qr-codes" className="inline-flex h-11 items-center px-2 text-[15px] text-zinc-400 hover:text-zinc-200">
                All QR codes
              </Link>
            </div>
          </section>
        ) : (
          <form
            id={formId}
            noValidate
            onSubmit={(e) => {
              e.preventDefault();
              void submit();
            }}
            className="space-y-8"
          >
            {!canCreate && (
              <p className="rounded-xl border border-amber-500/30 bg-amber-500/10 px-4 py-3 text-[14px] text-amber-200">
                You can view codes in this workspace, but creating them needs the editor role. Ask a workspace admin for access.
              </p>
            )}

            {/* What it does */}
            <fieldset>
              <legend className="mb-3 text-[13px] font-medium text-zinc-300">What should it open?</legend>
              <div role="radiogroup" aria-label="QR code type" className="flex flex-wrap gap-2">
                {QR_TYPES.map(({ id, label, icon: Icon }) => {
                  const active = id === type;
                  return (
                    <button
                      key={id}
                      type="button"
                      role="radio"
                      aria-checked={active}
                      onClick={() => pickType(id)}
                      className={`inline-flex h-9 items-center gap-2 rounded-full border px-3.5 text-[14px] transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-violet-400 ${
                        active
                          ? "border-violet-500/60 bg-violet-500/15 text-violet-100"
                          : "border-zinc-800 text-zinc-400 hover:border-zinc-700 hover:text-zinc-200"
                      }`}
                    >
                      <Icon size={15} strokeWidth={2} />
                      {label}
                    </button>
                  );
                })}
              </div>
            </fieldset>

            {/* Details */}
            <div>
              <div className="grid gap-4 sm:grid-cols-2">{def.fields.map(renderField)}</div>
              {def.hint && <p className="mt-2.5 text-[13px] text-zinc-500">{def.hint}</p>}
            </div>

            <div className="grid gap-4 sm:grid-cols-2">
              <div className="sm:col-span-2">
                <label htmlFor={`${formId}-name`} className="mb-1.5 block text-[13px] font-medium text-zinc-300">
                  Name <span className="font-normal text-zinc-500">(only you see this)</span>
                </label>
                <input
                  id={`${formId}-name`}
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder={suggestion || "Lunch menu, table 4"}
                  maxLength={200}
                  className={field}
                />
              </div>
            </div>

            {canTrack && (
              <label className="flex cursor-pointer items-start justify-between gap-6 rounded-xl border border-zinc-800 px-4 py-3.5 hover:border-zinc-700">
                <span>
                  <span className="block text-[15px] font-medium text-zinc-100">Track scans and change the link later</span>
                  <span className="mt-0.5 block text-[13px] text-zinc-500">
                    {tracked
                      ? "The code prints a short link, so you can see scans and send people somewhere new without reprinting."
                      : "The address is printed straight into the code. It can't be changed or tracked later."}
                  </span>
                </span>
                <span className="relative mt-0.5 inline-flex shrink-0">
                  <input type="checkbox" role="switch" checked={tracked} onChange={(e) => setTracked(e.target.checked)} className="peer sr-only" />
                  <span className="h-6 w-10 rounded-full bg-zinc-700 transition-colors peer-checked:bg-violet-600 peer-focus-visible:ring-2 peer-focus-visible:ring-violet-400" />
                  <span className="absolute left-0.5 top-0.5 h-5 w-5 rounded-full bg-white transition-transform peer-checked:translate-x-4 motion-reduce:transition-none" />
                </span>
              </label>
            )}

            {/* Look */}
            <fieldset className="space-y-5 border-t border-zinc-800/80 pt-7">
              <legend className="sr-only">Style</legend>
              <div className="flex flex-wrap items-center gap-x-8 gap-y-4">
                <div>
                  <p className="mb-2 text-[13px] font-medium text-zinc-300">Color</p>
                  <div className="flex items-center gap-2">
                    {INKS.map((ink) => (
                      <button
                        key={ink.value}
                        type="button"
                        title={ink.name}
                        aria-label={`${ink.name} color`}
                        aria-pressed={style.color === ink.value}
                        onClick={() => setStyle((s) => ({ ...s, color: ink.value }))}
                        className={`h-8 w-8 rounded-full border-2 transition-transform focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-violet-400 ${
                          style.color === ink.value ? "scale-110 border-white" : "border-zinc-700 hover:scale-105"
                        }`}
                        style={{ backgroundColor: ink.value }}
                      />
                    ))}
                    <label
                      title="Custom color"
                      className={`relative h-8 w-8 cursor-pointer overflow-hidden rounded-full border-2 ${
                        INKS.some((i) => i.value === style.color) ? "border-zinc-700" : "scale-110 border-white"
                      }`}
                      style={{ background: "conic-gradient(#ef4444,#f59e0b,#22c55e,#06b6d4,#6366f1,#d946ef,#ef4444)" }}
                    >
                      <span className="sr-only">Custom color</span>
                      <input
                        type="color"
                        value={style.color}
                        onChange={(e) => setStyle((s) => ({ ...s, color: e.target.value }))}
                        className="absolute inset-0 cursor-pointer opacity-0"
                      />
                    </label>
                  </div>
                </div>

                <div>
                  <p className="mb-2 text-[13px] font-medium text-zinc-300">Background</p>
                  <Segmented
                    value={style.background === "transparent" ? "transparent" : "white"}
                    options={[{ id: "white", label: "White" }, { id: "transparent", label: "None" }]}
                    onChange={(v) => setStyle((s) => ({ ...s, background: v === "transparent" ? "transparent" : "#FFFFFF" }))}
                  />
                </div>
              </div>

              <div className="flex flex-wrap gap-x-8 gap-y-4">
                <div>
                  <p className="mb-2 text-[13px] font-medium text-zinc-300">Pattern</p>
                  <div className="flex gap-2">
                    {DOTS.map((d) => (
                      <ShapeButton key={d.id} label={d.label} active={style.dots === d.id} onClick={() => setStyle((s) => ({ ...s, dots: d.id }))}>
                        <DotGlyph kind={d.id} />
                      </ShapeButton>
                    ))}
                  </div>
                </div>
                <div>
                  <p className="mb-2 text-[13px] font-medium text-zinc-300">Corners</p>
                  <div className="flex gap-2">
                    {CORNERS.map((c) => (
                      <ShapeButton key={c.id} label={c.label} active={style.corners === c.id} onClick={() => setStyle((s) => ({ ...s, corners: c.id }))}>
                        <CornerGlyph kind={c.id} />
                      </ShapeButton>
                    ))}
                  </div>
                </div>
                <div>
                  <p className="mb-2 text-[13px] font-medium text-zinc-300">Logo</p>
                  {style.logo ? (
                    <div className="flex h-11 items-center gap-2 rounded-xl border border-zinc-800 pl-1.5 pr-1">
                      {/* eslint-disable-next-line @next/next/no-img-element */}
                      <img src={style.logo} alt="" className="h-8 w-8 rounded-md bg-white object-contain p-0.5" />
                      <button
                        type="button"
                        onClick={() => setStyle((s) => ({ ...s, logo: null }))}
                        className="inline-flex h-8 items-center gap-1 rounded-lg px-2 text-[13px] text-zinc-400 hover:bg-zinc-800 hover:text-zinc-200"
                      >
                        <X size={14} /> Remove
                      </button>
                    </div>
                  ) : (
                    <button
                      type="button"
                      onClick={() => logoInput.current?.click()}
                      className="inline-flex h-11 items-center gap-2 rounded-xl border border-dashed border-zinc-700 px-3.5 text-[14px] text-zinc-300 hover:border-zinc-500 hover:text-zinc-100"
                    >
                      <ImagePlus size={16} /> Add logo
                    </button>
                  )}
                  <input
                    ref={logoInput}
                    type="file"
                    accept="image/png,image/jpeg,image/svg+xml,image/webp"
                    className="hidden"
                    onChange={(e) => {
                      onLogo(e.target.files?.[0]);
                      e.target.value = "";
                    }}
                  />
                </div>
              </div>

              {contrast < 4 && (
                <p className="text-[13px] text-amber-300">
                  This color is light against the background, so some phones may not scan it. Pick a darker color.
                </p>
              )}
            </fieldset>

            {/* Extras */}
            <div className="border-t border-zinc-800/80 pt-5">
              <button
                type="button"
                aria-expanded={moreOpen}
                onClick={() => setMoreOpen((o) => !o)}
                className="inline-flex items-center gap-1.5 text-[14px] font-medium text-zinc-300 hover:text-zinc-100"
              >
                <ChevronDown size={16} className={`transition-transform motion-reduce:transition-none ${moreOpen ? "rotate-180" : ""}`} />
                More options
                <span className="font-normal text-zinc-500">folder{dynamic ? ", password, expiry, scan limit" : ""}</span>
              </button>
              {moreOpen && (
                <div className="mt-5 grid gap-4 sm:grid-cols-2">
                  <div>
                    <label htmlFor={`${formId}-folder`} className="mb-1.5 block text-[13px] font-medium text-zinc-300">Folder</label>
                    <select id={`${formId}-folder`} value={folderId} onChange={(e) => setFolderId(e.target.value)} className={field}>
                      <option value="">No folder</option>
                      {folders.map((f) => <option key={f.id} value={f.id}>{f.name}</option>)}
                    </select>
                  </div>
                  {dynamic ? (
                    <>
                      <div>
                        <label htmlFor={`${formId}-password`} className="mb-1.5 block text-[13px] font-medium text-zinc-300">Password to open</label>
                        <input id={`${formId}-password`} type="password" autoComplete="new-password" value={password} onChange={(e) => setPassword(e.target.value)} placeholder="Leave empty for none" className={field} />
                      </div>
                      <div>
                        <label htmlFor={`${formId}-expires`} className="mb-1.5 block text-[13px] font-medium text-zinc-300">Stops working on</label>
                        <input id={`${formId}-expires`} type="datetime-local" value={expiresAt} onChange={(e) => setExpiresAt(e.target.value)} className={`${field} [color-scheme:dark]`} />
                      </div>
                      <div>
                        <label htmlFor={`${formId}-limit`} className="mb-1.5 block text-[13px] font-medium text-zinc-300">Scan limit</label>
                        <input id={`${formId}-limit`} type="number" min={1} inputMode="numeric" value={maxScans} onChange={(e) => setMaxScans(e.target.value)} placeholder="No limit" className={field} />
                      </div>
                    </>
                  ) : (
                    <p className="self-end pb-2.5 text-[13px] text-zinc-500">
                      Password, expiry and scan limits work on tracked links only.
                    </p>
                  )}
                </div>
              )}
            </div>
          </form>
        )}

        {/* ------------------------------------------------------------ proof */}
        <aside id={`${formId}-proof`} className="lg:sticky lg:top-6 lg:self-start">
          <div className="rounded-3xl border border-zinc-800/80 bg-[radial-gradient(120%_80%_at_50%_0%,rgba(255,255,255,0.045),transparent_60%)] px-5 pb-6 pt-4">
            <QrProof
              data={proofData || "https://qrit.app"}
              style={style}
              placeholder={isPlaceholder}
              label={created ? `QR code for ${created.title}` : "Preview of your QR code"}
            >
              <p className="mt-2 min-h-[40px] text-center text-[13px] leading-5 text-zinc-500">
                {created
                  ? shortLink
                    ? `Scans open ${shortLink.replace(/^https?:\/\//, "")}`
                    : "The details are printed inside the code."
                  : isPlaceholder
                    ? "Your code appears here as you type."
                    : dynamic
                      ? `Prints a short link on ${host || "your site"}. The last part is added when you create it.`
                      : "Everything is printed inside the code, so it works without internet."}
              </p>
            </QrProof>

            {created ? (
              <div className="mt-4 grid grid-cols-2 gap-2">
                <button
                  type="button"
                  onClick={() => downloadQr(proofData, style, fileName, "png")}
                  className="inline-flex h-11 items-center justify-center gap-2 rounded-xl bg-white text-[15px] font-semibold text-zinc-900 hover:bg-zinc-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-violet-400"
                >
                  <Download size={16} /> PNG
                </button>
                <button
                  type="button"
                  onClick={() => downloadQr(proofData, style, fileName, "svg")}
                  className="inline-flex h-11 items-center justify-center gap-2 rounded-xl border border-zinc-700 text-[15px] font-medium text-zinc-100 hover:bg-zinc-800 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-violet-400"
                >
                  <Download size={16} /> SVG
                </button>
                <p className="col-span-2 mt-1 text-center text-[12px] text-zinc-500">SVG stays sharp at any print size.</p>
              </div>
            ) : (
              <div className="mt-4 hidden flex-col gap-2 lg:flex">
                {createButton("w-full")}
                <p className="text-center text-[12px] text-zinc-500">or press Ctrl + Enter</p>
              </div>
            )}

            {serverError && !serverError.field && (
              <p role="alert" className="mt-4 rounded-xl border border-red-500/30 bg-red-500/10 px-3.5 py-2.5 text-[13px] text-red-300">
                {serverError.message}
                {serverError.upgrade && (
                  <>
                    {" "}
                    <Link href="/pricing" className="font-medium text-red-200 underline underline-offset-2">
                      See plans
                    </Link>
                  </>
                )}
              </p>
            )}
          </div>
        </aside>
      </div>

      {/* Phone: the create button stays in reach. */}
      {!created && (
        <div className="fixed inset-x-0 bottom-0 z-30 flex items-center gap-3 border-t border-zinc-800 bg-zinc-950/90 px-4 py-3 backdrop-blur lg:hidden">
          <button
            type="button"
            aria-label="Show the preview"
            onClick={() => document.getElementById(`${formId}-proof`)?.scrollIntoView({ behavior: "smooth", block: "center" })}
            className="rounded-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-violet-400"
          >
            <QrThumb data={proofData || "https://qrit.app"} style={style} faded={isPlaceholder} />
          </button>
          {createButton("flex-1")}
        </div>
      )}
    </div>
  );
}

function Segmented({
  value, options, onChange,
}: { value: string; options: { id: string; label: string }[]; onChange: (id: string) => void }) {
  return (
    <div role="radiogroup" className="inline-flex h-8 rounded-lg border border-zinc-800 p-0.5">
      {options.map((o) => (
        <button
          key={o.id}
          type="button"
          role="radio"
          aria-checked={value === o.id}
          onClick={() => onChange(o.id)}
          className={`rounded-md px-3 text-[13px] transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-violet-400 ${
            value === o.id ? "bg-zinc-800 text-zinc-100" : "text-zinc-400 hover:text-zinc-200"
          }`}
        >
          {o.label}
        </button>
      ))}
    </div>
  );
}

function ShapeButton({
  label, active, onClick, children,
}: { label: string; active: boolean; onClick: () => void; children: React.ReactNode }) {
  return (
    <button
      type="button"
      aria-pressed={active}
      title={label}
      onClick={onClick}
      className={`flex h-11 w-11 items-center justify-center rounded-xl border transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-violet-400 ${
        active ? "border-violet-500/70 bg-violet-500/10 text-violet-100" : "border-zinc-800 text-zinc-400 hover:border-zinc-700 hover:text-zinc-200"
      }`}
    >
      <span className="sr-only">{label}</span>
      {children}
    </button>
  );
}

/** A 3x3 patch of modules in the given style. */
function DotGlyph({ kind }: { kind: DotStyle }) {
  const cells = [[0, 0], [1, 0], [0, 1], [2, 1], [1, 2], [2, 2]];
  return (
    <svg viewBox="0 0 18 18" width="20" height="20" aria-hidden fill="currentColor">
      {cells.map(([x, y]) => {
        const cx = x * 6 + 3;
        const cy = y * 6 + 3;
        if (kind === "dots") return <circle key={`${x}${y}`} cx={cx} cy={cy} r={2.6} />;
        const r = kind === "rounded" ? 1.8 : kind === "classy-rounded" ? 2.6 : 0;
        return <rect key={`${x}${y}`} x={cx - 2.8} y={cy - 2.8} width={5.6} height={5.6} rx={r} />;
      })}
    </svg>
  );
}

/** A finder pattern (the big corner squares) in the given style. */
function CornerGlyph({ kind }: { kind: CornerStyle }) {
  const outer = kind === "dot" ? 9 : kind === "extra-rounded" ? 5 : 0.5;
  const inner = kind === "dot" ? 4 : kind === "extra-rounded" ? 1.5 : 0;
  return (
    <svg viewBox="0 0 20 20" width="20" height="20" aria-hidden>
      <rect x="1.5" y="1.5" width="17" height="17" rx={outer} fill="none" stroke="currentColor" strokeWidth="3" />
      <rect x="6.5" y="6.5" width="7" height="7" rx={inner} fill="currentColor" />
    </svg>
  );
}
