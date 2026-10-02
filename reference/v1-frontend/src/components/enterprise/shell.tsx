"use client";

/**
 * Page scaffolding.
 *
 * Every enterprise page needs the same four things before it can render:
 * a loaded workspace list, an active workspace, a role high enough, and a
 * plan that includes the feature. This centralises that so pages only
 * contain their own content.
 */
import * as React from "react";
import Link from "next/link";

import { ApiError } from "@/lib/enterprise";
import { useWorkspace, useWorkspaceStore } from "@/lib/workspace";
import type { FeatureName, Role } from "@/types/enterprise";
import {
  Btn, EmptyState, ErrorState, LoadingPanel, RoleGate, UpgradeGate,
} from "./ui";

/** Fetch-with-state for a workspace-scoped resource. */
export function useResource<T>(
  loader: (workspaceId: string) => Promise<T>,
  deps: React.DependencyList = [],
) {
  const { workspaceId, loaded } = useWorkspace();
  const [data, setData] = React.useState<T | null>(null);
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState<ApiError | Error | null>(null);
  const [tick, setTick] = React.useState(0);

  const loaderRef = React.useRef(loader);
  loaderRef.current = loader;

  React.useEffect(() => {
    if (!loaded) return;
    if (!workspaceId) {
      setLoading(false);
      setData(null);
      return;
    }
    let cancelled = false;
    setLoading(true);
    setError(null);
    loaderRef.current(workspaceId)
      .then((result) => {
        if (!cancelled) setData(result);
      })
      .catch((cause) => {
        if (!cancelled) setError(cause instanceof Error ? cause : new Error(String(cause)));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [workspaceId, loaded, tick, ...deps]);

  return {
    data,
    setData,
    loading,
    error,
    reload: React.useCallback(() => setTick((n) => n + 1), []),
  };
}

/** Runs an async mutation with busy + error state and an optional reload. */
export function useMutation<Args extends unknown[], Result>(
  action: (...args: Args) => Promise<Result>,
  options: { onSuccess?: (result: Result) => void } = {},
) {
  const [busy, setBusy] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);
  const optionsRef = React.useRef(options);
  optionsRef.current = options;

  const run = React.useCallback(
    async (...args: Args): Promise<Result | null> => {
      setBusy(true);
      setError(null);
      try {
        const result = await action(...args);
        optionsRef.current.onSuccess?.(result);
        return result;
      } catch (cause) {
        setError(cause instanceof Error ? cause.message : "Something went wrong");
        return null;
      } finally {
        setBusy(false);
      }
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [],
  );

  return { run, busy, error, clearError: () => setError(null) };
}

export function WorkspaceGate({
  requiredRole,
  requiredFeature,
  children,
}: {
  requiredRole?: Role;
  requiredFeature?: FeatureName;
  children: React.ReactNode;
}) {
  const { workspaceId, loading, loaded, error, atLeast, can, minPlanFor } = useWorkspace();
  const load = useWorkspaceStore((s) => s.load);

  React.useEffect(() => {
    void load();
  }, [load]);

  if (!loaded || loading) return <LoadingPanel rows={6} />;

  if (error) {
    return <ErrorState message={error} onRetry={() => void load({ force: true })} />;
  }

  if (!workspaceId) {
    return (
      <EmptyState
        icon={
          <svg className="size-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" />
          </svg>
        }
        title="Create your first workspace"
        description="A workspace holds your QR codes, campaigns, team and analytics. Everything in the dashboard lives inside one."
        action={
          <Link href="/dashboard/settings">
            <Btn variant="primary">Create workspace</Btn>
          </Link>
        }
      />
    );
  }

  if (requiredRole && !atLeast(requiredRole)) return <RoleGate required={requiredRole} />;

  if (requiredFeature && !can(requiredFeature)) {
    return <UpgradeGate feature={requiredFeature} requiredPlan={minPlanFor(requiredFeature)} />;
  }

  return <>{children}</>;
}

/** Renders `children` only when the plan allows it, otherwise a lock hint. */
export function FeatureGuard({
  feature,
  children,
  fallback,
}: {
  feature: FeatureName;
  children: React.ReactNode;
  fallback?: React.ReactNode;
}) {
  const { can, minPlanFor } = useWorkspace();
  if (can(feature)) return <>{children}</>;
  return <>{fallback ?? <UpgradeGate feature={feature} requiredPlan={minPlanFor(feature)} />}</>;
}

/** Disables an action for roles below `minimum`, with an explanatory title. */
export function useCan(minimum: Role) {
  const { atLeast, role } = useWorkspace();
  const allowed = atLeast(minimum);
  return {
    allowed,
    disabledProps: allowed
      ? {}
      : {
          disabled: true,
          title: `Requires the ${minimum} role — you are ${role ?? "not a member"}`,
        },
  };
}
