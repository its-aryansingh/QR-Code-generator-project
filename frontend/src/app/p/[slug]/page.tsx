"use client";

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { CheckCircle, AlertCircle } from "lucide-react";

interface FormField {
  name: string;
  type: string;
  label: string;
  required?: boolean;
  placeholder?: string;
  options?: string[]; // for select fields
}

interface PageData {
  id: string;
  name: string;
  slug: string;
  headline: string;
  subheadline: string;
  hero_image: string;
  button_text: string;
  button_color: string;
  background_color: string;
  text_color: string;
  thank_you_message: string;
  redirect_url: string;
  form_fields: string; // JSON string
  requires_opt_in: boolean;
  is_active: boolean;
}

export default function PublicLeadPage() {
  const params = useParams();
  const slug = params?.slug as string;
  const api = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8081/api/v1";

  const [page, setPage] = useState<PageData | null>(null);
  const [fields, setFields] = useState<FormField[]>([]);
  const [formValues, setFormValues] = useState<Record<string, string>>({});
  const [optIn, setOptIn] = useState(false);
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [submitted, setSubmitted] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!slug) return;
    fetch(`${api}/pages/${slug}`)
      .then((r) => r.json())
      .then((d) => {
        if (d.success) {
          setPage(d.data);
          try {
            const f: FormField[] = JSON.parse(d.data.form_fields || "[]");
            setFields(f);
            const initial: Record<string, string> = {};
            f.forEach((field) => { initial[field.name] = ""; });
            setFormValues(initial);
          } catch {}
        } else {
          setError("This page could not be found.");
        }
      })
      .catch(() => setError("Failed to load page."))
      .finally(() => setLoading(false));
  }, [slug, api]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!page) return;

    // Validate required fields
    for (const field of fields) {
      if (field.required && !formValues[field.name]?.trim()) {
        setError(`${field.label} is required.`);
        return;
      }
    }
    if (page.requires_opt_in && !optIn) {
      setError("Please agree to receive communications to continue.");
      return;
    }

    setError("");
    setSubmitting(true);

    try {
      const body: Record<string, string> = { ...formValues };
      if (page.requires_opt_in) body._opt_in = optIn ? "true" : "false";

      const res = await fetch(`${api}/pages/${slug}/submit`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });
      const data = await res.json();
      if (res.ok && data.success) {
        setSubmitted(true);
        if (page.redirect_url) {
          setTimeout(() => { window.location.href = page.redirect_url; }, 2500);
        }
      } else {
        setError(data.error ?? "Submission failed. Please try again.");
      }
    } catch {
      setError("Network error. Please try again.");
    } finally {
      setSubmitting(false);
    }
  };

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-zinc-950">
        <div className="w-8 h-8 border-2 border-violet-500 border-t-transparent rounded-full animate-spin" />
      </div>
    );
  }

  if (error && !page) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-zinc-950 text-center px-4">
        <div>
          <AlertCircle className="w-12 h-12 text-zinc-600 mx-auto mb-4" />
          <p className="text-zinc-300 text-lg font-medium">{error}</p>
          <p className="text-zinc-600 text-sm mt-2">The page you're looking for doesn't exist or has been removed.</p>
        </div>
      </div>
    );
  }

  if (!page) return null;

  const bg = page.background_color || "#09090b";
  const fg = page.text_color || "#fafafa";
  const btnColor = page.button_color || "#7c3aed";

  return (
    <div className="min-h-screen flex flex-col" style={{ backgroundColor: bg, color: fg }}>
      {/* Hero / header */}
      {page.hero_image && (
        <div className="w-full h-48 sm:h-64 overflow-hidden">
          <img src={page.hero_image} alt="" className="w-full h-full object-cover" />
        </div>
      )}

      <div className="flex-1 flex items-center justify-center px-4 py-12">
        <div className="w-full max-w-md">
          {/* Headline */}
          {page.headline && (
            <h1 className="text-2xl sm:text-3xl font-bold mb-2 text-center" style={{ color: fg }}>
              {page.headline}
            </h1>
          )}
          {page.subheadline && (
            <p className="text-sm sm:text-base text-center mb-8 opacity-70" style={{ color: fg }}>
              {page.subheadline}
            </p>
          )}

          {submitted ? (
            <div className="text-center space-y-4">
              <CheckCircle className="w-16 h-16 mx-auto text-emerald-400" />
              <p className="text-lg font-semibold" style={{ color: fg }}>
                {page.thank_you_message || "Thank you! We'll be in touch."}
              </p>
              {page.redirect_url && (
                <p className="text-sm opacity-60" style={{ color: fg }}>Redirecting you shortly…</p>
              )}
            </div>
          ) : (
            <form onSubmit={handleSubmit} className="space-y-4">
              {fields.map((field) => (
                <div key={field.name}>
                  <label className="block text-sm font-medium mb-1.5 opacity-80" style={{ color: fg }}>
                    {field.label}
                    {field.required && <span className="text-red-400 ml-1">*</span>}
                  </label>

                  {field.type === "textarea" ? (
                    <textarea
                      value={formValues[field.name] ?? ""}
                      onChange={(e) => setFormValues((p) => ({ ...p, [field.name]: e.target.value }))}
                      placeholder={field.placeholder}
                      required={field.required}
                      rows={3}
                      className="w-full px-4 py-2.5 rounded-lg text-sm border border-white/10 bg-white/5 focus:outline-none focus:border-white/30 resize-none"
                      style={{ color: fg }}
                    />
                  ) : field.type === "select" && field.options ? (
                    <select
                      value={formValues[field.name] ?? ""}
                      onChange={(e) => setFormValues((p) => ({ ...p, [field.name]: e.target.value }))}
                      required={field.required}
                      className="w-full px-4 py-2.5 rounded-lg text-sm border border-white/10 bg-white/5 focus:outline-none focus:border-white/30"
                      style={{ color: fg, backgroundColor: `${bg}cc` }}
                    >
                      <option value="">Select…</option>
                      {field.options.map((opt) => <option key={opt} value={opt}>{opt}</option>)}
                    </select>
                  ) : (
                    <input
                      type={field.type || "text"}
                      value={formValues[field.name] ?? ""}
                      onChange={(e) => setFormValues((p) => ({ ...p, [field.name]: e.target.value }))}
                      placeholder={field.placeholder}
                      required={field.required}
                      className="w-full px-4 py-2.5 rounded-lg text-sm border border-white/10 bg-white/5 focus:outline-none focus:border-white/30"
                      style={{ color: fg }}
                    />
                  )}
                </div>
              ))}

              {/* Opt-in checkbox */}
              {page.requires_opt_in && (
                <label className="flex items-start gap-3 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={optIn}
                    onChange={(e) => setOptIn(e.target.checked)}
                    className="mt-0.5 w-4 h-4 rounded accent-violet-500"
                  />
                  <span className="text-xs opacity-70" style={{ color: fg }}>
                    I agree to receive marketing communications. You can unsubscribe at any time.
                  </span>
                </label>
              )}

              {/* Error */}
              {error && (
                <div className="flex items-center gap-2 p-3 rounded-lg bg-red-500/10 border border-red-500/30 text-red-300 text-sm">
                  <AlertCircle size={14} /> {error}
                </div>
              )}

              <button
                type="submit"
                disabled={submitting}
                className="w-full py-3 rounded-lg font-semibold text-sm text-white disabled:opacity-60 transition-opacity flex items-center justify-center gap-2"
                style={{ backgroundColor: btnColor }}
              >
                {submitting && <div className="w-4 h-4 border-2 border-white border-t-transparent rounded-full animate-spin" />}
                {submitting ? "Submitting…" : page.button_text || "Submit"}
              </button>
            </form>
          )}
        </div>
      </div>

      {/* Powered by footer */}
      <div className="text-center py-4">
        <a
          href="/"
          className="text-xs opacity-30 hover:opacity-50 transition-opacity"
          style={{ color: fg }}
        >
          Powered by QRit
        </a>
      </div>
    </div>
  );
}
