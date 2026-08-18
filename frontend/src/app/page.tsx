"use client"

import { useEffect } from "react"
import Link from "next/link"
import { useRouter } from "next/navigation"
import { useAuth } from "@/features/auth/hooks/use-auth"
import { buttonVariants } from "@/components/ui/button"
import { cn } from "@/lib/utils"
import { Logo } from "@/components/logo"
import { ArrowRight, Boxes, Layers, Loader2, ShieldCheck, Zap } from "lucide-react"

const features = [
  {
    icon: Zap,
    title: "Real-time streaming",
    description:
      "WebSocket messaging with single-use upgrade tickets and exponential-backoff reconnection for a snappy, live chat experience.",
  },
  {
    icon: ShieldCheck,
    title: "DPoP-secured (RFC 9449)",
    description:
      "Sender-constrained access tokens with nonce/jti replay protection. A stolen token can't be replayed from another device.",
  },
  {
    icon: Layers,
    title: "CQRS architecture",
    description:
      "Command/query services separated by an outbox pattern and NATS JetStream, keeping reads fast and writes consistent.",
  },
  {
    icon: Boxes,
    title: "Go microservices",
    description:
      "Gateway-routed Go services over PostgreSQL, Redis, and NATS JetStream — built to scale and survive partial failures.",
  },
]

const techStack = ["Go 1.26+", "Echo v5", "NATS JetStream", "Redis 7", "PostgreSQL 16", "Next.js 16"]

function GithubIcon({ className }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 16 16"
      fill="currentColor"
      role="img"
      aria-label="GitHub"
      className={className}
    >
      <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27s1.36.09 2 .27c1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.01 8.01 0 0 0 16 8c0-4.42-3.58-8-8-8z" />
    </svg>
  )
}

export default function RootPage() {
  const { isAuthenticated, isLoading } = useAuth()
  const router = useRouter()

  useEffect(() => {
    if (!isLoading && isAuthenticated) {
      router.replace("/home")
    }
  }, [isLoading, isAuthenticated, router])

  if (isLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-black">
        <Loader2 className="h-8 w-8 animate-spin text-blue-500" />
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-black text-gray-100">
      {/* Nav */}
      <header className="sticky top-0 z-10 border-b border-gray-800/60 bg-black/70 backdrop-blur">
        <div className="mx-auto flex h-16 max-w-5xl items-center justify-between px-6">
          <Link href="/" className="flex items-center gap-2">
            <Logo className="h-7 w-7" />
            <span className="text-lg font-semibold tracking-tight">Chinwag</span>
          </Link>
          <nav className="flex items-center gap-3">
            <Link
              href="/login"
              className={cn(buttonVariants({ variant: "ghost", size: "sm" }))}
            >
              Sign in
            </Link>
            <Link
              href="/register"
              className={cn(buttonVariants({ variant: "default", size: "sm" }))}
            >
              Get started
            </Link>
          </nav>
        </div>
      </header>

      {/* Hero */}
      <section className="mx-auto max-w-5xl px-6 pt-20 pb-16 text-center sm:pt-28">
        <div className="mb-6 flex justify-center">
          <Logo className="h-20 w-20" />
        </div>
        <span className="mb-5 inline-flex items-center gap-1.5 rounded-full border border-gray-800 bg-gray-900/60 px-3 py-1 text-xs text-gray-400">
          <ShieldCheck className="h-3.5 w-3.5 text-blue-400" />
          DPoP-secured · Go microservices · CQRS
        </span>
        <h1 className="mx-auto max-w-2xl text-4xl font-bold tracking-tight sm:text-5xl">
          Real-time streaming chat,
          <span className="text-blue-500"> built to last.</span>
        </h1>
        <p className="mx-auto mt-5 max-w-xl text-base leading-relaxed text-gray-400 sm:text-lg">
          Chinwag is a real-time streaming chat service built on Go microservices —
          DPoP authentication, CQRS event consistency, and WebSocket streaming under the hood.
        </p>
        <div className="mt-8 flex flex-wrap items-center justify-center gap-3">
          <Link
            href="/register"
            className={cn(buttonVariants({ variant: "default", size: "lg" }))}
          >
            Get started
            <ArrowRight className="ml-2 h-4 w-4" />
          </Link>
          <Link
            href="/login"
            className={cn(buttonVariants({ variant: "outline", size: "lg" }))}
          >
            Sign in
          </Link>
        </div>
      </section>

      {/* Features */}
      <section className="mx-auto max-w-5xl px-6 pb-20">
        <div className="grid gap-4 sm:grid-cols-2">
          {features.map((feature) => (
            <div
              key={feature.title}
              className="rounded-xl border border-gray-800 bg-gray-900/40 p-6"
            >
              <feature.icon className="mb-4 h-6 w-6 text-blue-500" />
              <h3 className="mb-2 font-semibold text-gray-100">{feature.title}</h3>
              <p className="text-sm leading-relaxed text-gray-500">{feature.description}</p>
            </div>
          ))}
        </div>
      </section>

      {/* Tech strip */}
      <section className="mx-auto max-w-5xl px-6 pb-20">
        <div className="flex flex-wrap items-center justify-center gap-x-3 gap-y-2 text-sm text-gray-500">
          {techStack.map((tech, i) => (
            <span key={tech} className="flex items-center gap-3">
              {i > 0 && <span className="text-gray-700">·</span>}
              {tech}
            </span>
          ))}
        </div>
      </section>

      {/* Footer */}
      <footer className="border-t border-gray-800/60 py-6">
        <div className="mx-auto flex max-w-5xl flex-col items-center justify-between gap-4 px-6 text-sm text-gray-500 sm:flex-row">
          <div className="flex items-center gap-2">
            <Logo className="h-5 w-5" />
            <span>Chinwag — real-time streaming chat</span>
          </div>
          <div className="flex items-center gap-5">
            <Link
              href="https://github.com/Sephy314"
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-center gap-1.5 transition-colors hover:text-gray-200"
            >
              <GithubIcon className="h-4 w-4" />
              Sephy314
            </Link>
            <Link
              href="https://github.com/Sephy314/Chinwag"
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-center gap-1.5 transition-colors hover:text-gray-200"
            >
              <GithubIcon className="h-4 w-4" />
              Chinwag repo
            </Link>
          </div>
          <span>Apache License 2.0</span>
        </div>
      </footer>
    </div>
  )
}
