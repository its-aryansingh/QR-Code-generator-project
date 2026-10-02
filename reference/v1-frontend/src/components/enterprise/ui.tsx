"use client";

/**
 * Shared dashboard primitives.
 *
 * The pages each rolled their own cards, tables, empty states and spinners,
 * so nothing lined up and no page had a real loading or error state. These
 * are the pieces every enterprise page is built from.
 */
import * as React from "react";
import Link from "next/link";
import { cn } from "@/lib/utils";
import type { FeatureName, Role } from "@/types/enterprise";

/* ------------------------------------------------------------------ layout */

export function PageHeader({
  title,
  description,
  actions,
  breadcrumb,
}: {
  title: string;
  description?: string;
  actions?: React.ReactNode;
  breadcrumb?: React.ReactNode;
}) {
  return (
    <div className="mb-6 flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
      <div className="min-w-0">
        {breadcrumb && <div className="mb-1.5 text-xs text-zinc-500">{breadcrumb}</div>}
        <h1 className="truncate text-2xl font-semibold tracking-tight text-white">{title}</h1>
        {description && <p className="mt-1 max-w-2xl text-sm text-zinc-400">{description}</p>}
      </div>
      {actions && <div className="flex shrink-0 flex-wrap items-center gap-2">{actions}</div>}
    </div>
  );
}

export function Panel({
  title,
  description,
  actions,
  children,
  className,
  bodyClassName,
}: {
  title?: string;
  description?: string;
  actions?: React.ReactNode;
  children: React.ReactNode;
  className?: string;
  bodyClassName?: string;
}) {
  return (
    <section
      className={cn("rounded-xl border border-zinc-800/70 bg-zinc-900/40", className)}
    >
      {(title || actions) && (
        <header className="flex items-start justify-between gap-4 border-b border-zinc-800/70 px-5 py-4">
          <div className="min-w-0">
            {title && <h2 className="text-sm font-semibold text-zinc-100">{title}</h2>}
            {description && <p className="mt-0.5 text-xs text-zinc-500">{description}</p>}
          </div>
          {actions && <div className="flex shrink-0 items-center gap-2">{actions}</div>}
        </header>
      )}
      <div className={cn("p-5", bodyClassName)}>{children}</div>
    </section>
  );
}

export function Grid({
  cols = 4,
  children,
  className,
}: {
  cols?: 2 | 3 | 4;
  children: React.ReactNode;
  className?: string;
}) {
  const map = {
    2: "sm:grid-cols-2",
    3: "sm:grid-cols-2 lg:grid-cols-3",
    4: "sm:grid-cols-2 lg:grid-cols-4",
  } as const;
  return <div className={cn("grid grid-cols-1 gap-4", map[cols], className)}>{children}</div>;
}

/* ------------------------------------------------------------------ stats */

export function StatCard({
  label,
  value,
  delta,
  hint,
  icon,
  href,
}: {
  label: string;
  value: React.ReactNode;
  delta?: number | null;
  hint?: string;
  icon?: React.ReactNode;
  href?: string;
}) {
  const body = (
    <div className="rounded-xl border border-zinc-800/70 bg-zinc-900/40 p-4 transition-colors hover:border-zinc-700">
      <div className="flex items-start justify-between gap-2">
        <p className="text-xs font-medium uppercase tracking-wide text-zinc-500">{label}</p>
        {icon && <span className="text-zinc-600">{icon}</span>}
      </div>
      <p className="mt-2 text-2xl font-semibold tabular-nums text-white">{value}</p>
      <div className="mt-1 flex items-center gap-2">
        {typeof delta === "number" && <DeltaPill value={delta} />}
        {hint && <span className="truncate text-xs text-zinc-500">{hint}</span>}
      </div>
    </div>
  );
  return href ? <Link href={href}>{body}</Link> : body;
}

export function DeltaPill({ value }: { value: number }) {
  const flat = Math.abs(value) < 0.05;
  const up = value > 0;
  return (
    <span
      className={cn(
        "inline-flex items-center gap-0.5 rounded-md px-1.5 py-0.5 text-xs font-medium tabular-nums",
        flat
          ? "bg-zinc-800 text-zinc-400"
          : up
            ? "bg-emerald-500/10 text-emerald-400"
            : "bg-red-500/10 text-red-400",
      )}
    >
      {!flat && (up ? "▲" : "▼")}
      {flat ? "0%" : `${Math.abs(value)}%`}
    </span>
  );
}

/* ------------------------------------------------------------------ badges */

const ROLE_STYLES: Record<Role, string> = {
  owner: "bg-amber-500/10 text-amber-400 ring-amber-500/20",
  admin: "bg-violet-500/10 text-violet-300 ring-violet-500/20",
  editor: "bg-sky-500/10 text-sky-300 ring-sky-500/20",
  viewer: "bg-zinc-700/40 text-zinc-300 ring-zinc-600/30",
};

export function RoleBadge({ role }: { role: Role }) {
  return (
    <span
      className={cn(
        "inline-flex items-center rounded-md px-2 py-0.5 text-xs font-medium capitalize ring-1 ring-inset",
        ROLE_STYLES[role] ?? ROLE_STYLES.viewer,
      )}
    >
      {role}
    </span>
  );
}

type Tone = "neutral" | "success" | "warning" | "danger" | "info" | "violet";

const TONE_STYLES: Record<Tone, string> = {
  neutral: "bg-zinc-800 text-zinc-300 ring-zinc-700",
  success: "bg-emerald-500/10 text-emerald-400 ring-emerald-500/20",
  warning: "bg-amber-500/10 text-amber-400 ring-amber-500/20",
  danger: "bg-red-500/10 text-red-400 ring-red-500/20",
  info: "bg-sky-500/10 text-sky-300 ring-sky-500/20",
  violet: "bg-violet-500/10 text-violet-300 ring-violet-500/20",
};

export function Badge({
  tone = "neutral",
  children,
  className,
}: {
  tone?: Tone;
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1 rounded-md px-2 py-0.5 text-xs font-medium ring-1 ring-inset",
        TONE_STYLES[tone],
        className,
      )}
    >
      {children}
    </span>
  );
}

export function StatusDot({ active, label }: { active: boolean; label?: string }) {
  return (
    <span className="inline-flex items-center gap-1.5 text-xs text-zinc-400">
      <span
        className={cn(
          "size-1.5 rounded-full",
          active ? "bg-emerald-400 shadow-[0_0_6px] shadow-emerald-400/60" : "bg-zinc-600",
        )}
      />
      {label ?? (active ? "Active" : "Paused")}
    </span>
  );
}

/* ------------------------------------------------------------------ buttons */

type ButtonVariant = "primary" | "secondary" | "ghost" | "danger" | "outline";

const BUTTON_STYLES: Record<ButtonVariant, string> = {
  primary: "bg-white text-zinc-950 hover:bg-zinc-200",
  secondary: "bg-zinc-800 text-zinc-100 hover:bg-zinc-700",
  ghost: "text-zinc-400 hover:bg-zinc-800 hover:text-zinc-100",
  danger: "bg-red-600 text-white hover:bg-red-500",
  outline: "border border-zinc-700 text-zinc-200 hover:border-zinc-600 hover:bg-zinc-800/60",
};

export function Btn({
  variant = "secondary",
  size = "md",
  loading,
  className,
  children,
  ...props
}: React.ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: ButtonVariant;
  size?: "sm" | "md";
  loading?: boolean;
}) {
  return (
    <button
      {...props}
      disabled={props.disabled || loading}
      className={cn(
        "inline-flex items-center justify-center gap-1.5 rounded-lg font-medium transition-colors",
        "disabled:cursor-not-allowed disabled:opacity-50",
        size === "sm" ? "px-2.5 py-1.5 text-xs" : "px-3.5 py-2 text-sm",
        BUTTON_STYLES[variant],
        className,
      )}
    >
      {loading && <Spinner className="size-3.5" />}
      {children}
    </button>
  );
}

export function Spinner({ className }: { className?: string }) {
  return (
    <svg className={cn("animate-spin", className ?? "size-4")} viewBox="0 0 24 24" fill="none">
      <circle className="opacity-20" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
      <path
        className="opacity-90"
        fill="currentColor"
        d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z"
      />
    </svg>
  );
}

/* ------------------------------------------------------------------ inputs */

export function Field({
  label,
  hint,
  error,
  required,
  children,
  className,
}: {
  label?: string;
  hint?: string;
  error?: string;
  required?: boolean;
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("space-y-1.5", className)}>
      {label && (
        <label className="block text-xs font-medium text-zinc-300">
          {label}
          {required && <span className="ml-0.5 text-red-400">*</span>}
        </label>
      )}
      {children}
      {error ? (
        <p className="text-xs text-red-400">{error}</p>
      ) : (
        hint && <p className="text-xs text-zinc-500">{hint}</p>
      )}
    </div>
  );
}

const CONTROL =
  "w-full rounded-lg border border-zinc-800 bg-zinc-950 px-3 py-2 text-sm text-zinc-100 " +
  "placeholder:text-zinc-600 outline-none transition-colors focus:border-violet-500/60 " +
  "focus:ring-1 focus:ring-violet-500/30 disabled:opacity-50";

export function TextInput({
  className,
  ...props
}: React.InputHTMLAttributes<HTMLInputElement>) {
  return <input {...props} className={cn(CONTROL, className)} />;
}

export function TextArea({
  className,
  ...props
}: React.TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return <textarea {...props} className={cn(CONTROL, "min-h-24 resize-y", className)} />;
}

export function Select({
  className,
  children,
  ...props
}: React.SelectHTMLAttributes<HTMLSelectElement>) {
  return (
    <select {...props} className={cn(CONTROL, "appearance-none pr-8", className)}>
      {children}
    </select>
  );
}

export function Toggle({
  checked,
  onChange,
  disabled,
  label,
  description,
}: {
  checked: boolean;
  onChange: (next: boolean) => void;
  disabled?: boolean;
  label?: string;
  description?: string;
}) {
  return (
    <label
      className={cn(
        "flex items-start gap-3",
        disabled ? "cursor-not-allowed opacity-60" : "cursor-pointer",
      )}
    >
      <button
        type="button"
        role="switch"
        aria-checked={checked}
        disabled={disabled}
        onClick={() => !disabled && onChange(!checked)}
        className={cn(
          "relative mt-0.5 h-5 w-9 shrink-0 rounded-full transition-colors",
          checked ? "bg-violet-600" : "bg-zinc-700",
        )}
      >
        <span
          className={cn(
            "absolute top-0.5 size-4 rounded-full bg-white transition-transform",
            checked ? "translate-x-4" : "translate-x-0.5",
          )}
        />
      </button>
      {(label || description) && (
        <span className="min-w-0">
          {label && <span className="block text-sm text-zinc-200">{label}</span>}
          {description && <span className="block text-xs text-zinc-500">{description}</span>}
        </span>
      )}
    </label>
  );
}

export function SearchInput({
  value,
  onChange,
  placeholder = "Search…",
  className,
}: {
  value: string;
  onChange: (next: string) => void;
  placeholder?: string;
  className?: string;
}) {
  return (
    <div className={cn("relative", className)}>
      <svg
        className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-zinc-600"
        fill="none"
        stroke="currentColor"
        viewBox="0 0 24 24"
      >
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-4.35-4.35M17 11a6 6 0 11-12 0 6 6 0 0112 0z" />
      </svg>
      <input
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        className={cn(CONTROL, "pl-9")}
      />
      {value && (
        <button
          onClick={() => onChange("")}
          className="absolute right-2 top-1/2 -translate-y-1/2 rounded p-1 text-zinc-600 hover:text-zinc-300"
          aria-label="Clear search"
        >
          <svg className="size-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      )}
    </div>
  );
}

/* ------------------------------------------------------------------ states */

export function EmptyState({
  icon,
  title,
  description,
  action,
}: {
  icon?: React.ReactNode;
  title: string;
  description?: string;
  action?: React.ReactNode;
}) {
  return (
    <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-zinc-800 px-6 py-14 text-center">
      {icon && (
        <div className="mb-3 flex size-11 items-center justify-center rounded-xl bg-zinc-800/60 text-zinc-500">
          {icon}
        </div>
      )}
      <p className="text-sm font-medium text-zinc-200">{title}</p>
      {description && <p className="mt-1 max-w-sm text-sm text-zinc-500">{description}</p>}
      {action && <div className="mt-4">{action}</div>}
    </div>
  );
}

export function Skeleton({ className }: { className?: string }) {
  return <div className={cn("animate-pulse rounded-md bg-zinc-800/70", className)} />;
}

export function LoadingPanel({ rows = 5 }: { rows?: number }) {
  return (
    <div className="space-y-2.5">
      {Array.from({ length: rows }).map((_, index) => (
        <Skeleton key={index} className="h-11 w-full" />
      ))}
    </div>
  );
}

export function ErrorState({
  message,
  onRetry,
}: {
  message: string;
  onRetry?: () => void;
}) {
  return (
    <div className="rounded-xl border border-red-900/50 bg-red-950/20 px-5 py-4">
      <div className="flex items-start gap-3">
        <svg className="mt-0.5 size-4 shrink-0 text-red-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v4m0 4h.01M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z" />
        </svg>
        <div className="min-w-0 flex-1">
          <p className="text-sm text-red-200">{message}</p>
          {onRetry && (
            <button onClick={onRetry} className="mt-2 text-xs font-medium text-red-300 underline underline-offset-2 hover:text-red-200">
              Try again
            </button>
          )}
        </div>
      </div>
    </div>
  );
}

/** Shown in place of a feature the current plan does not include. */
export function UpgradeGate({
  feature,
  requiredPlan,
  description,
}: {
  feature: FeatureName | string;
  requiredPlan?: string;
  description?: string;
}) {
  const label = String(feature).replace(/_/g, " ");
  return (
    <div className="rounded-xl border border-violet-900/40 bg-violet-950/10 px-6 py-10 text-center">
      <div className="mx-auto mb-3 flex size-11 items-center justify-center rounded-xl bg-violet-500/10 text-violet-300">
        <svg className="size-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
        </svg>
      </div>
      <p className="text-sm font-medium capitalize text-zinc-100">{label} is not on your plan</p>
      <p className="mx-auto mt-1 max-w-sm text-sm text-zinc-500">
        {description ?? `Upgrade to ${requiredPlan ?? "a higher plan"} to unlock this.`}
      </p>
      <Link
        href="/pricing"
        className="mt-4 inline-flex items-center gap-1.5 rounded-lg bg-violet-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-violet-500"
      >
        View plans
      </Link>
    </div>
  );
}

/** Shown when the caller's role is too low for the page. */
export function RoleGate({ required }: { required: Role }) {
  return (
    <EmptyState
      icon={
        <svg className="size-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M18.364 18.364A9 9 0 005.636 5.636m12.728 12.728A9 9 0 015.636 5.636m12.728 12.728L5.636 5.636" />
        </svg>
      }
      title={`You need the ${required} role`}
      description="Ask a workspace owner or admin to change your role if you need access to this."
    />
  );
}

/* ------------------------------------------------------------------ modal */

export function Modal({
  open,
  onClose,
  title,
  description,
  children,
  footer,
  width = "md",
}: {
  open: boolean;
  onClose: () => void;
  title: string;
  description?: string;
  children: React.ReactNode;
  footer?: React.ReactNode;
  width?: "sm" | "md" | "lg";
}) {
  React.useEffect(() => {
    if (!open) return;
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") onClose();
    };
    document.addEventListener("keydown", onKey);
    const previous = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.removeEventListener("keydown", onKey);
      document.body.style.overflow = previous;
    };
  }, [open, onClose]);

  if (!open) return null;
  const widths = { sm: "max-w-sm", md: "max-w-lg", lg: "max-w-2xl" };

  return (
    <div className="fixed inset-0 z-[100] flex items-end justify-center overflow-y-auto p-0 sm:items-center sm:p-4">
      <div className="fixed inset-0 bg-black/70 backdrop-blur-sm" onClick={onClose} />
      <div
        role="dialog"
        aria-modal="true"
        className={cn(
          "relative w-full rounded-t-2xl border border-zinc-800 bg-zinc-900 shadow-2xl sm:rounded-2xl",
          widths[width],
        )}
      >
        <header className="flex items-start justify-between gap-4 border-b border-zinc-800 px-5 py-4">
          <div className="min-w-0">
            <h2 className="text-base font-semibold text-white">{title}</h2>
            {description && <p className="mt-0.5 text-sm text-zinc-500">{description}</p>}
          </div>
          <button
            onClick={onClose}
            className="rounded-lg p-1.5 text-zinc-500 transition-colors hover:bg-zinc-800 hover:text-zinc-200"
            aria-label="Close"
          >
            <svg className="size-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </header>
        <div className="max-h-[70vh] overflow-y-auto px-5 py-4">{children}</div>
        {footer && (
          <footer className="flex items-center justify-end gap-2 border-t border-zinc-800 px-5 py-3.5">
            {footer}
          </footer>
        )}
      </div>
    </div>
  );
}

export function ConfirmModal({
  open,
  onClose,
  onConfirm,
  title,
  message,
  confirmLabel = "Delete",
  destructive = true,
  busy,
}: {
  open: boolean;
  onClose: () => void;
  onConfirm: () => void;
  title: string;
  message: string;
  confirmLabel?: string;
  destructive?: boolean;
  busy?: boolean;
}) {
  return (
    <Modal
      open={open}
      onClose={onClose}
      title={title}
      width="sm"
      footer={
        <>
          <Btn variant="ghost" onClick={onClose}>Cancel</Btn>
          <Btn variant={destructive ? "danger" : "primary"} onClick={onConfirm} loading={busy}>
            {confirmLabel}
          </Btn>
        </>
      }
    >
      <p className="text-sm text-zinc-400">{message}</p>
    </Modal>
  );
}

/* ------------------------------------------------------------------ table */

export interface Column<T> {
  key: string;
  header: React.ReactNode;
  cell: (row: T) => React.ReactNode;
  className?: string;
  headerClassName?: string;
}

export function DataTable<T extends { id: string }>({
  columns,
  rows,
  selectable,
  selected,
  onSelectedChange,
  empty,
  loading,
  rowHref,
}: {
  columns: Column<T>[];
  rows: T[];
  selectable?: boolean;
  selected?: string[];
  onSelectedChange?: (ids: string[]) => void;
  empty?: React.ReactNode;
  loading?: boolean;
  rowHref?: (row: T) => string;
}) {
  const selectedSet = React.useMemo(() => new Set(selected ?? []), [selected]);
  const allSelected = rows.length > 0 && rows.every((row) => selectedSet.has(row.id));

  const toggleAll = () =>
    onSelectedChange?.(allSelected ? [] : rows.map((row) => row.id));

  const toggleOne = (id: string) => {
    const next = new Set(selectedSet);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    onSelectedChange?.([...next]);
  };

  if (loading) return <LoadingPanel rows={6} />;
  if (!rows.length && empty) return <>{empty}</>;

  return (
    <div className="overflow-x-auto rounded-xl border border-zinc-800/70">
      <table className="w-full min-w-[640px] text-left text-sm">
        <thead>
          <tr className="border-b border-zinc-800/70 bg-zinc-900/60">
            {selectable && (
              <th className="w-10 px-3 py-2.5">
                <input
                  type="checkbox"
                  checked={allSelected}
                  onChange={toggleAll}
                  aria-label="Select all"
                  className="size-3.5 cursor-pointer accent-violet-500"
                />
              </th>
            )}
            {columns.map((column) => (
              <th
                key={column.key}
                className={cn(
                  "px-3 py-2.5 text-xs font-medium uppercase tracking-wide text-zinc-500",
                  column.headerClassName,
                )}
              >
                {column.header}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => (
            <tr
              key={row.id}
              className={cn(
                "border-b border-zinc-800/40 transition-colors last:border-0 hover:bg-zinc-900/50",
                selectedSet.has(row.id) && "bg-violet-950/20",
              )}
            >
              {selectable && (
                <td className="px-3 py-2.5">
                  <input
                    type="checkbox"
                    checked={selectedSet.has(row.id)}
                    onChange={() => toggleOne(row.id)}
                    aria-label="Select row"
                    className="size-3.5 cursor-pointer accent-violet-500"
                  />
                </td>
              )}
              {columns.map((column, index) => (
                <td key={column.key} className={cn("px-3 py-2.5 text-zinc-300", column.className)}>
                  {index === 0 && rowHref ? (
                    <Link href={rowHref(row)} className="block hover:text-white">
                      {column.cell(row)}
                    </Link>
                  ) : (
                    column.cell(row)
                  )}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

export function Pagination({
  page,
  pages,
  total,
  onPage,
}: {
  page: number;
  pages: number;
  total: number;
  onPage: (next: number) => void;
}) {
  if (pages <= 1) return null;
  return (
    <div className="mt-3 flex items-center justify-between text-xs text-zinc-500">
      <span>
        Page {page} of {pages} · {total.toLocaleString()} total
      </span>
      <div className="flex gap-1.5">
        <Btn size="sm" variant="outline" disabled={page <= 1} onClick={() => onPage(page - 1)}>
          Previous
        </Btn>
        <Btn size="sm" variant="outline" disabled={page >= pages} onClick={() => onPage(page + 1)}>
          Next
        </Btn>
      </div>
    </div>
  );
}

/* ------------------------------------------------------------------ misc */

export function CopyButton({
  value,
  label = "Copy",
  className,
}: {
  value: string;
  label?: string;
  className?: string;
}) {
  const [copied, setCopied] = React.useState(false);

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(value);
    } catch {
      // Clipboard API needs a secure context; fall back to a temp selection.
      const node = document.createElement("textarea");
      node.value = value;
      node.style.position = "fixed";
      node.style.opacity = "0";
      document.body.appendChild(node);
      node.select();
      document.execCommand("copy");
      node.remove();
    }
    setCopied(true);
    setTimeout(() => setCopied(false), 1600);
  };

  return (
    <button
      onClick={copy}
      className={cn(
        "inline-flex items-center gap-1.5 rounded-md px-2 py-1 text-xs font-medium transition-colors",
        copied ? "text-emerald-400" : "text-zinc-400 hover:bg-zinc-800 hover:text-zinc-200",
        className,
      )}
    >
      {copied ? (
        <svg className="size-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
        </svg>
      ) : (
        <svg className="size-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
        </svg>
      )}
      {copied ? "Copied" : label}
    </button>
  );
}

export function Avatar({
  name,
  email,
  url,
  size = 32,
}: {
  name?: string | null;
  email?: string;
  url?: string | null;
  size?: number;
}) {
  const initials = (name || email || "?")
    .split(/[\s@.]+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase())
    .join("");

  if (url) {
    // eslint-disable-next-line @next/next/no-img-element
    return (
      <img
        src={url}
        alt={name || email || "Avatar"}
        width={size}
        height={size}
        className="shrink-0 rounded-full object-cover"
        style={{ width: size, height: size }}
      />
    );
  }
  return (
    <span
      className="flex shrink-0 items-center justify-center rounded-full bg-violet-600/20 font-medium text-violet-300"
      style={{ width: size, height: size, fontSize: size * 0.36 }}
    >
      {initials}
    </span>
  );
}

export function Tabs({
  tabs,
  active,
  onChange,
}: {
  tabs: Array<{ id: string; label: string; count?: number; disabled?: boolean }>;
  active: string;
  onChange: (id: string) => void;
}) {
  return (
    <div className="mb-5 flex gap-1 overflow-x-auto border-b border-zinc-800/70">
      {tabs.map((tab) => (
        <button
          key={tab.id}
          disabled={tab.disabled}
          onClick={() => onChange(tab.id)}
          className={cn(
            "-mb-px shrink-0 border-b-2 px-3.5 py-2.5 text-sm font-medium transition-colors",
            tab.disabled && "cursor-not-allowed opacity-40",
            active === tab.id
              ? "border-violet-500 text-white"
              : "border-transparent text-zinc-500 hover:text-zinc-200",
          )}
        >
          {tab.label}
          {typeof tab.count === "number" && (
            <span className="ml-1.5 rounded bg-zinc-800 px-1.5 py-0.5 text-xs tabular-nums text-zinc-400">
              {tab.count}
            </span>
          )}
        </button>
      ))}
    </div>
  );
}

/* ------------------------------------------------------------------ format */

export function formatNumber(value: number | undefined | null) {
  return (value ?? 0).toLocaleString();
}

export function formatDate(value?: string | null, withTime = false) {
  if (!value) return "—";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "—";
  return date.toLocaleDateString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
    ...(withTime ? { hour: "2-digit", minute: "2-digit" } : {}),
  });
}

export function formatRelative(value?: string | null) {
  if (!value) return "—";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "—";
  const seconds = Math.round((Date.now() - date.getTime()) / 1000);
  const units: Array<[Intl.RelativeTimeFormatUnit, number]> = [
    ["second", 60], ["minute", 60], ["hour", 24],
    ["day", 7], ["week", 4.35], ["month", 12], ["year", Infinity],
  ];
  const formatter = new Intl.RelativeTimeFormat(undefined, { numeric: "auto" });
  let amount = seconds;
  for (const [unit, divisor] of units) {
    if (Math.abs(amount) < divisor) return formatter.format(-Math.round(amount), unit);
    amount /= divisor;
  }
  return formatDate(value);
}
