import { cn } from "@/lib/utils"

/**
 * Chinwag logo mark — a blue chat bubble with a "w" (for "wag").
 * Inline SVG so it renders at any size without next/image config.
 */
export function Logo({ className }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 64 64"
      fill="none"
      role="img"
      aria-label="Chinwag logo"
      className={cn("h-8 w-8", className)}
    >
      <defs>
        <linearGradient id="cw-bg" x1="0" y1="0" x2="1" y2="1">
          <stop offset="0%" stopColor="#60a5fa" />
          <stop offset="100%" stopColor="#2563eb" />
        </linearGradient>
      </defs>
      <rect x="1" y="1" width="62" height="62" rx="16" fill="url(#cw-bg)" />
      <g fill="#ffffff">
        <rect x="14" y="16" width="36" height="30" rx="10" />
        <path d="M19 45 L12.5 55 L26 47 Z" />
      </g>
      <path
        d="M22 25.5 L26 38.5 L29.5 28.5 L33 38.5 L37 25.5"
        stroke="#2563eb"
        strokeWidth="4.5"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  )
}
