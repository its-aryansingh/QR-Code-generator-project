'use client';

import React, { useState, useEffect } from 'react';
import { api } from '@/lib/api/client';

interface LiveBadgeProps {
  workspace: string;
}

export function LiveBadge({ workspace }: LiveBadgeProps) {
  const [recentCount, setRecentCount] = useState<number>(0);

  useEffect(() => {
    let timer: NodeJS.Timeout;
    let mounted = true;

    async function fetchRealtime() {
      if (typeof document !== 'undefined' && document.visibilityState !== 'visible') {
        return;
      }
      try {
        const res: any = await api.get(`/v1/workspaces/${workspace}/realtime?minutes=15`);
        if (mounted && res?.total !== undefined) {
          setRecentCount(res.total);
        }
      } catch {
        // quiet fallback
      }
    }

    fetchRealtime();
    timer = setInterval(fetchRealtime, 10000);

    return () => {
      mounted = false;
      clearInterval(timer);
    };
  }, [workspace]);

  return (
    <div className="inline-flex items-center gap-2 px-2.5 py-1 rounded-full bg-green-500/10 border border-green-500/20 text-xs font-semibold text-green-600">
      <span className="relative flex h-2 w-2">
        <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-green-400 opacity-75" />
        <span className="relative inline-flex rounded-full h-2 w-2 bg-green-500" />
      </span>
      <span>
        Live: <strong className="tabular-nums">{recentCount}</strong> {recentCount === 1 ? 'scan' : 'scans'} in last 15m
      </span>
    </div>
  );
}
