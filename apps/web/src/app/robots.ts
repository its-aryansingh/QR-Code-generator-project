import { MetadataRoute } from 'next';

export default function robots(): MetadataRoute.Robots {
  const baseUrl = process.env.NEXT_PUBLIC_APP_URL || 'https://qrit.io';

  return {
    rules: [
      {
        userAgent: '*',
        allow: '/',
        disallow: ['/w/', '/api/', '/v1/'],
      },
    ],
    sitemap: `${baseUrl}/sitemap.xml`,
  };
}
