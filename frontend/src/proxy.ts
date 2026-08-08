// Local Next.js: proxy API paths to the gateway on the host.
// In Docker Compose, nginx (edge) handles this instead.

import { NextRequest, NextResponse } from "next/server"

export function proxy(request: NextRequest) {
  const gatewayUrl =
    process.env.GATEWAY_URL ||
    process.env.NEXT_PUBLIC_GATEWAY_URL ||
    "http://localhost:8000"
  const gateway = new URL(gatewayUrl)
  const url = request.nextUrl.clone()
  url.protocol = gateway.protocol
  url.host = gateway.host
  // Strip the /api routing prefix — the gateway/backends don't see it.
  if (url.pathname.startsWith("/api")) {
    url.pathname = url.pathname.slice(4) || "/"
  }
  return NextResponse.rewrite(url)
}

export const config = {
  matcher: ["/api/:path*"],
}
