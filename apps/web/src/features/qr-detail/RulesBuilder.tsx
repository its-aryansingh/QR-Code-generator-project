'use client';

import React, { useState } from 'react';
import { Plus, Trash2, Split, Clock, Globe, Smartphone, Check } from 'lucide-react';

export interface RuleCondition {
  field: string;
  op: string;
  value: any;
}

export interface SplitVariant {
  variant: string;
  weight: number;
  destination_url: string;
}

export interface RoutingRule {
  id: string;
  name: string;
  enabled: boolean;
  when?: {
    all?: RuleCondition[];
    any?: RuleCondition[];
  } | null;
  destination_url?: string;
  split?: SplitVariant[];
}

interface RulesBuilderProps {
  rules: RoutingRule[];
  onChange: (rules: RoutingRule[]) => void;
}

export function RulesBuilder({ rules, onChange }: RulesBuilderProps) {
  const [activeTab, setActiveTab] = useState<number>(0);

  const handleAddRule = () => {
    const newRule: RoutingRule = {
      id: `r_${Date.now()}`,
      name: `Rule ${rules.length + 1}`,
      enabled: true,
      when: {
        all: [{ field: 'os', op: 'in', value: ['iOS'] }],
      },
      destination_url: 'https://',
    };
    onChange([...rules, newRule]);
    setActiveTab(rules.length);
  };

  const handleDeleteRule = (idx: number) => {
    const next = rules.filter((_, i) => i !== idx);
    onChange(next);
    if (activeTab >= next.length) {
      setActiveTab(Math.max(0, next.length - 1));
    }
  };

  const handleUpdateRule = (idx: number, updated: RoutingRule) => {
    const next = [...rules];
    next[idx] = updated;
    onChange(next);
  };

  return (
    <div className="bg-[var(--surface)] p-6 rounded-2xl border border-[var(--border)] shadow-xs space-y-6">
      <div className="flex items-center justify-between pb-4 border-b border-[var(--border)]">
        <div>
          <h3 className="text-sm font-bold text-[var(--text)]">Smart Routing & A/B Rules</h3>
          <p className="text-xs text-[var(--text-muted)]">
            Evaluate requests top to bottom; the first matching rule dictates destination.
          </p>
        </div>
        <button
          type="button"
          onClick={handleAddRule}
          className="px-3 py-1.5 bg-[var(--accent)] text-[var(--accent-fg)] rounded-lg text-xs font-semibold flex items-center gap-1.5 shadow-xs hover:opacity-95"
        >
          <Plus className="w-3.5 h-3.5" />
          Add Rule
        </button>
      </div>

      {rules.length === 0 ? (
        <div className="py-12 text-center text-xs text-[var(--text-muted)] space-y-3">
          <p>No routing rules defined yet. Traffic resolves directly to the default destination.</p>
          <button
            type="button"
            onClick={handleAddRule}
            className="px-3.5 py-2 bg-[var(--bg-subtle)] border border-[var(--border)] rounded-lg text-xs font-semibold text-[var(--text)] hover:border-[var(--accent)]"
          >
            Create first rule
          </button>
        </div>
      ) : (
        <div className="space-y-4">
          {/* Rules List / Tabs */}
          <div className="flex items-center gap-2 overflow-x-auto pb-1">
            {rules.map((rule, idx) => (
              <button
                key={rule.id}
                type="button"
                onClick={() => setActiveTab(idx)}
                className={`px-3 py-1.5 rounded-lg text-xs font-semibold border transition-all flex items-center gap-2 shrink-0 ${
                  activeTab === idx
                    ? 'border-[var(--accent)] bg-[var(--accent)]/10 text-[var(--accent)]'
                    : 'border-[var(--border)] bg-[var(--bg-subtle)] text-[var(--text-muted)]'
                }`}
              >
                <span>{rule.name || `Rule ${idx + 1}`}</span>
                {rule.enabled ? (
                  <span className="w-1.5 h-1.5 rounded-full bg-green-500" />
                ) : (
                  <span className="w-1.5 h-1.5 rounded-full bg-gray-400" />
                )}
              </button>
            ))}
          </div>

          {/* Active Rule Editor */}
          {rules[activeTab] && (
            <div className="p-4 bg-[var(--bg-subtle)] rounded-xl border border-[var(--border)] space-y-4">
              <div className="flex items-center justify-between">
                <input
                  type="text"
                  value={rules[activeTab].name}
                  onChange={(e) =>
                    handleUpdateRule(activeTab, { ...rules[activeTab], name: e.target.value })
                  }
                  className="font-bold text-xs bg-[var(--surface)] border border-[var(--border)] px-3 py-1.5 rounded-lg text-[var(--text)]"
                />

                <div className="flex items-center gap-2">
                  <label className="text-xs font-semibold text-[var(--text-muted)] flex items-center gap-1.5">
                    <input
                      type="checkbox"
                      checked={rules[activeTab].enabled}
                      onChange={(e) =>
                        handleUpdateRule(activeTab, { ...rules[activeTab], enabled: e.target.checked })
                      }
                      className="rounded border-[var(--border)]"
                    />
                    Enabled
                  </label>
                  <button
                    type="button"
                    onClick={() => handleDeleteRule(activeTab)}
                    className="p-1 text-[var(--danger)] hover:bg-[var(--danger)]/10 rounded"
                    title="Delete Rule"
                  >
                    <Trash2 className="w-4 h-4" />
                  </button>
                </div>
              </div>

              {/* Destination URL */}
              <div>
                <label className="block text-xs font-semibold text-[var(--text-muted)] mb-1">Target Destination URL</label>
                <input
                  type="url"
                  value={rules[activeTab].destination_url || ''}
                  onChange={(e) =>
                    handleUpdateRule(activeTab, { ...rules[activeTab], destination_url: e.target.value })
                  }
                  placeholder="https://apps.apple.com/app"
                  className="w-full px-3 py-2 bg-[var(--surface)] border border-[var(--border)] rounded-lg text-xs font-mono text-[var(--text)]"
                />
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
