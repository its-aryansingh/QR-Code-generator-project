import Link from "next/link";
import { ArrowLeft, Home, QrCode } from "lucide-react";

export default function NotFound() {
  return (
    <div className="min-h-screen bg-zinc-950 flex items-center justify-center p-4 text-zinc-100">
      <div className="max-w-md w-full text-center space-y-6">
        <div className="inline-flex items-center justify-center w-16 h-16 rounded-2xl bg-violet-600/10 border border-violet-500/20 text-violet-400 mb-2">
          <QrCode className="w-8 h-8" />
        </div>

        <div className="space-y-2">
          <h1 className="text-6xl font-bold tracking-tight text-zinc-100">404</h1>
          <h2 className="text-xl font-semibold text-zinc-200">Page Not Found</h2>
          <p className="text-sm text-zinc-400">
            The page you are looking for doesn&apos;t exist, has been removed, or the link may be broken.
          </p>
        </div>

        <div className="flex flex-col sm:flex-row gap-3 justify-center pt-2">
          <Link
            href="/"
            className="inline-flex items-center justify-center gap-2 px-5 py-2.5 rounded-lg bg-violet-600 hover:bg-violet-500 text-white font-medium text-sm transition-colors"
          >
            <Home className="w-4 h-4" />
            Back to Home
          </Link>
          <Link
            href="/dashboard"
            className="inline-flex items-center justify-center gap-2 px-5 py-2.5 rounded-lg bg-zinc-900 hover:bg-zinc-800 border border-zinc-800 text-zinc-300 font-medium text-sm transition-colors"
          >
            <ArrowLeft className="w-4 h-4" />
            Dashboard
          </Link>
        </div>
      </div>
    </div>
  );
}
