export interface User {
  id: string;
  email: string;
  name: string;
  avatar_url?: string;
  email_verified: boolean;
}

export interface WorkspaceMember {
  workspace_id: string;
  name: string;
  slug: string;
  role: 'owner' | 'admin' | 'editor' | 'analyst';
  plan: 'free' | 'pro' | 'business' | 'enterprise';
}

export interface AuthSession {
  user: User;
  workspaces: WorkspaceMember[];
  currentWorkspace?: WorkspaceMember;
}

export function getCsrfTokenFromCookie(): string | null {
  if (typeof document === 'undefined') return null;
  const match = document.cookie.match(/(?:^|;\s*)qrit_csrf=([^;]*)/);
  return match ? decodeURIComponent(match[1]) : null;
}
