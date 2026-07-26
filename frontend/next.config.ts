import type { NextConfig } from "next"

const nextConfig: NextConfig = {
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
  async rewrites() {
    return [
      {
        source: "/auth/:path*",
        destination: "http://localhost:8000/auth/:path*",
      },
      {
        source: "/chat/rooms/:path*",
        destination: "http://localhost:8000/chat/rooms/:path*",
      },
      {
        source: "/rooms/:path*",
        destination: "http://localhost:8000/rooms/:path*",
      },
      {
        source: "/users/:path*",
        destination: "http://localhost:8000/users/:path*",
      },
    ]
  },
}

export default nextConfig
