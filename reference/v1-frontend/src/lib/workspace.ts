/**
 * Shared workspace context.
 *
 * Every dashboard page used to fetch `/workspaces` on mount and silently pick
 * `data[0]`, so the sidebar switcher changed nothing, the selection reset on
 * navigation, and two pages could be showing two different workspaces at once.
 * The active workspace now lives here, persisted, and every page reads it.
 */
import { create } from "zustand";
import { persist } from "zustand/middleware";

import { ApiError, enterprise } from "./enterprise";
import type { Entitlements, FeatureName, Role, Workspace } from "@/types/enterprise";
import { roleAtLeast } from "@/types/enterprise";

interface WorkspaceState {
  workspaces: Workspace[];
  activeId: string | null;
  entitlements: Entitlements | null;
  loading: boolean;
  loaded: boolean;
  error: string | null;

  load: (options?: { force?: boolean }) => Promise<void>;
  setActive: (id: string) => void;
  refreshEntitlements: () => Promise<void>;
  upsert: (workspace: Workspace) => void;
  remove: (id: string) => void;
  reset: () => void;
}

export const useWorkspaceStore = create<WorkspaceState>()(
  persist(
    (set, get) => ({
      workspaces: [],
      activeId: null,
      entitlements: null,
      loading: false,
      loaded: false,
      error: null,

      load: async ({ force = false } = {}) => {
        if (get().loading) return;
        if (get().loaded && !force) return;
        set({ loading: true, error: null });
        try {
          const workspaces = await enterprise.listWorkspaces();
          const previous = get().activeId;
          const stillValid = previous && workspaces.some((w) => w.id === previous);
          const activeId = stillValid ? previous : workspaces[0]?.id ?? null;
          set({ workspaces, activeId, loading: false, loaded: true });
          if (activeId) await get().refreshEntitlements();
        } catch (error) {
          set({
            loading: false,
            loaded: true,
            error: error instanceof ApiError ? error.message : "Could not load workspaces",
          });
        }
      },

      setActive: (id) => {
        if (get().activeId === id) return;
        set({ activeId: id, entitlements: null });
        localStorage.setItem("qrit_active_workspace", id);
        void get().refreshEntitlements();
      },

      refreshEntitlements: async () => {
        const id = get().activeId;
        if (!id) return;
        try {
          set({ entitlements: await enterprise.entitlements(id) });
        } catch {
          set({ entitlements: null });
        }
      },

      upsert: (workspace) =>
        set((state) => {
          const index = state.workspaces.findIndex((w) => w.id === workspace.id);
          const workspaces =
            index >= 0
              ? state.workspaces.map((w) => (w.id === workspace.id ? { ...w, ...workspace } : w))
              : [workspace, ...state.workspaces];
          return { workspaces, activeId: state.activeId ?? workspace.id };
        }),

      remove: (id) =>
        set((state) => {
          const workspaces = state.workspaces.filter((w) => w.id !== id);
          return {
            workspaces,
            activeId: state.activeId === id ? workspaces[0]?.id ?? null : state.activeId,
            entitlements: state.activeId === id ? null : state.entitlements,
          };
        }),

      reset: () =>
        set({
          workspaces: [], activeId: null, entitlements: null,
          loading: false, loaded: false, error: null,
        }),
    }),
    {
      name: "qrit-workspace",
      partialize: (state) => ({ activeId: state.activeId }),
    },
  ),
);

/** Convenience selector bundle used by every dashboard page. */
export function useWorkspace() {
  const workspaces = useWorkspaceStore((s) => s.workspaces);
  const activeId = useWorkspaceStore((s) => s.activeId);
  const entitlements = useWorkspaceStore((s) => s.entitlements);
  const loading = useWorkspaceStore((s) => s.loading);
  const loaded = useWorkspaceStore((s) => s.loaded);
  const error = useWorkspaceStore((s) => s.error);

  const workspace = workspaces.find((w) => w.id === activeId) ?? null;
  const role = (workspace?.role ?? entitlements?.role) as Role | undefined;

  return {
    workspaces,
    workspace,
    workspaceId: activeId,
    entitlements,
    role,
    plan: entitlements?.plan ?? workspace?.plan ?? "free",
    loading,
    loaded,
    error,
    /** Does the plan include this capability? */
    can: (feature: FeatureName) => entitlements?.features?.[feature]?.enabled ?? false,
    /** Is the caller's role at least `minimum`? */
    atLeast: (minimum: Role) => roleAtLeast(role, minimum),
    limit: (key: keyof Entitlements["limits"]) => entitlements?.limits?.[key] ?? 0,
    usage: (key: string) => entitlements?.usage?.[key] ?? 0,
    minPlanFor: (feature: FeatureName) => entitlements?.features?.[feature]?.min_plan ?? "enterprise",
  };
}
