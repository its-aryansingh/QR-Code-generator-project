"use client";

import { Check, X } from "lucide-react";

interface PasswordStrengthProps {
  password: string;
}

interface Requirement {
  label: string;
  test: (pw: string) => boolean;
}

const requirements: Requirement[] = [
  { label: "At least 8 characters", test: (pw) => pw.length >= 8 },
  { label: "One uppercase letter", test: (pw) => /[A-Z]/.test(pw) },
  { label: "One lowercase letter", test: (pw) => /[a-z]/.test(pw) },
  { label: "One number", test: (pw) => /\d/.test(pw) },
  { label: "One special character", test: (pw) => /[!@#$%^&*()_+\-=\[\]{};':"\\|,.<>/?]/.test(pw) },
];

export function getPasswordScore(password: string): number {
  if (!password) return 0;
  return requirements.filter((r) => r.test(password)).length;
}

function getStrength(password: string): { score: number; label: string; color: string } {
  if (!password) return { score: 0, label: "", color: "bg-zinc-800" };

  const passed = getPasswordScore(password);
  const pct = Math.round((passed / requirements.length) * 100);

  if (passed <= 1) return { score: pct, label: "Weak", color: "bg-red-500" };
  if (passed <= 2) return { score: pct, label: "Fair", color: "bg-orange-500" };
  if (passed <= 3) return { score: pct, label: "Fair", color: "bg-orange-500" };
  if (passed === 4) return { score: pct, label: "Good", color: "bg-yellow-500" };
  return { score: 100, label: "Strong", color: "bg-emerald-500" };
}

export function PasswordStrength({ password }: PasswordStrengthProps) {
  const strength = getStrength(password);

  if (!password) return null;

  return (
    <div className="space-y-3 mt-3 animate-in fade-in slide-in-from-top-2 duration-300">
      {/* Strength bar */}
      <div className="space-y-1.5">
        <div className="flex items-center justify-between">
          <span className="text-xs text-zinc-500">Password strength</span>
          <span className={`text-xs font-medium ${
            strength.score <= 20 ? "text-red-400" :
            strength.score <= 60 ? "text-orange-400" :
            strength.score <= 80 ? "text-yellow-400" :
            "text-emerald-400"
          }`}>
            {strength.label}
          </span>
        </div>
        <div className="h-1.5 bg-zinc-800 rounded-full overflow-hidden">
          <div
            className={`h-full rounded-full transition-all duration-500 ease-out ${strength.color}`}
            style={{ width: `${strength.score}%` }}
          />
        </div>
      </div>

      {/* Requirements checklist */}
      <div className="grid grid-cols-2 gap-1.5">
        {requirements.map((req) => {
          const passed = req.test(password);
          return (
            <div
              key={req.label}
              className={`flex items-center gap-1.5 text-xs transition-colors duration-200 ${
                passed ? "text-emerald-400" : "text-zinc-600"
              }`}
            >
              {passed ? (
                <Check className="w-3 h-3 flex-shrink-0" />
              ) : (
                <X className="w-3 h-3 flex-shrink-0" />
              )}
              <span>{req.label}</span>
            </div>
          );
        })}
      </div>
    </div>
  );
}
