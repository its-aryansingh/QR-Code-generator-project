import type { NextConfig } from "next";
import path from "path";
import { fileURLToPath } from "url";

// ensure .env.local is resolved relative to this directory
const workspaceRoot = path.dirname(fileURLToPath(import.meta.url));

const nextConfig: NextConfig = {
  output: "standalone",
  images: {
    unoptimized: true,
  },
};

export default nextConfig;
