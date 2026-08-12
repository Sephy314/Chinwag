import Link from "next/link"
import { AdminGuard } from "@/components/admin-guard"
import { Logo } from "@/components/logo"
import { cn } from "@/lib/utils"
import {
  LayoutDashboard,
  Users,
  KeyRound,
  MessagesSquare,
  ScrollText,
  ExternalLink,
} from "lucide-react"

const GRAFANA_URL =
  process.env.NEXT_PUBLIC_GRAFANA_URL || "https://chinwag.duckdns.org/grafana"

const NAV_ITEMS = [
  { href: "/admin", label: "Overview", icon: LayoutDashboard, exact: true },
  { href: "/admin/users", label: "Users", icon: Users },
  { href: "/admin/sessions", label: "Sessions", icon: KeyRound },
  { href: "/admin/rooms", label: "Rooms", icon: MessagesSquare },
  { href: "/admin/messages", label: "Messages", icon: ScrollText },
  { href: "/admin/audit", label: "Audit", icon: ScrollText },
]

function AdminNav() {
  return (
    <nav className="flex flex-wrap items-center gap-1 border-b border-gray-800 px-4 py-2">
      {NAV_ITEMS.map((item) => {
        const Icon = item.icon
        return (
          <Link
            key={item.href}
            href={item.href}
            className={cn(
              "inline-flex items-center gap-2 rounded-lg px-3 py-1.5 text-sm font-medium transition-colors",
              "text-gray-400 hover:bg-gray-900 hover:text-gray-200",
            )}
          >
            <Icon className="h-4 w-4" />
            {item.label}
          </Link>
        )
      })}
      <Link
        href="/home"
        className="ml-auto inline-flex items-center gap-2 rounded-lg px-3 py-1.5 text-sm font-medium text-gray-400 transition-colors hover:bg-gray-900 hover:text-gray-200"
      >
        ← Back to app
      </Link>
    </nav>
  )
}

export default function AdminLayout({ children }: { children: React.ReactNode }) {
  return (
    <AdminGuard>
      <div className="flex min-h-screen flex-col bg-black">
        <header className="flex h-14 items-center justify-between border-b border-gray-800 px-4">
          <div className="flex items-center gap-2">
            <Logo className="h-7 w-7" />
            <span className="font-semibold text-gray-100">Chinwag Admin</span>
          </div>
          <a
            href={GRAFANA_URL}
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-2 rounded-lg border border-gray-800 px-3 py-1.5 text-sm font-medium text-gray-300 transition-colors hover:border-gray-700 hover:bg-gray-900 hover:text-gray-100"
          >
            <ExternalLink className="h-4 w-4" />
            Grafana
          </a>
        </header>
        <AdminNav />
        <main className="flex-1 overflow-auto p-4">{children}</main>
      </div>
    </AdminGuard>
  )
}
