import path from 'path';
import type { NextConfig } from 'next';

const nextConfig: NextConfig = {
  reactStrictMode: true,
  outputFileTracingRoot: path.join(__dirname, '../../'),
  transpilePackages: ['@qrit/qr-render'],
  experimental: {
    serverActions: {
      bodySizeLimit: '10mb',
    },
  },
  async rewrites() {
    const apiTarget = process.env.API_INTERNAL_URL || 'http://localhost:8080';
    return [
      {
        source: '/v1/:path*',
        destination: `${apiTarget}/v1/:path*`,
      },
    ];
  },
};

export default nextConfig;
