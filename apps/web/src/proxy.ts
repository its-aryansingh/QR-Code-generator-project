export interface ProxyOptions extends RequestInit {
  cookie?: string;
}

export async function apiProxy(path: string, options: ProxyOptions = {}): Promise<Response> {
  const baseUrl = process.env.API_INTERNAL_URL || 'http://localhost:8080';
  const url = `${baseUrl.replace(/\/$/, '')}${path.startsWith('/') ? path : `/${path}`}`;

  const headers = new Headers(options.headers || {});
  if (options.cookie) {
    headers.set('Cookie', options.cookie);
  }

  return fetch(url, {
    ...options,
    headers,
  });
}
