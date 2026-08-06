import type { NextConfig } from "next"

const nextConfig: NextConfig = {
  // Emits a self-contained server in .next/standalone so the app can run in a
  // minimal container image (used by Dockerfile).
  output: "standalone",
  turbopack: {
    root: process.cwd(),
  },
  images: {
    remotePatterns: [
      {
        protocol: "http",
        hostname: "localhost",
        port: "8000",
      },
    ],
  },
}

export default nextConfig
