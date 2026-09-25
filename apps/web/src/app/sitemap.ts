import { MetadataRoute } from 'next';

export default function sitemap(): MetadataRoute.Sitemap {
  const baseUrl = process.env.NEXT_PUBLIC_APP_URL || 'https://qrit.io';

  const routes = [
    { path: '', changeFrequency: 'daily', priority: 1.0 },
    { path: '/pricing', changeFrequency: 'weekly', priority: 0.9 },
    { path: '/wifi-qr-code', changeFrequency: 'monthly', priority: 0.8 },
    { path: '/upi-qr-code', changeFrequency: 'monthly', priority: 0.8 },
    { path: '/vcard-qr-code', changeFrequency: 'monthly', priority: 0.8 },
    { path: '/docs/api', changeFrequency: 'weekly', priority: 0.7 },
    { path: '/abuse', changeFrequency: 'monthly', priority: 0.5 },
    { path: '/terms', changeFrequency: 'yearly', priority: 0.3 },
    { path: '/privacy', changeFrequency: 'yearly', priority: 0.3 },
  ];

  return routes.map((r) => ({
    url: `${baseUrl}${r.path}`,
    lastModified: new Date(),
    changeFrequency: r.changeFrequency as any,
    priority: r.priority,
  }));
}
