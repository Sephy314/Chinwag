import { ProtectedRoute } from "@/components/protected-route"
import { SidebarWrapper } from "./sidebar-wrapper"

export default function MainLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <ProtectedRoute>
      <div className="flex h-screen overflow-hidden bg-black">
        <SidebarWrapper />
        <main className="flex-1 overflow-hidden">{children}</main>
      </div>
    </ProtectedRoute>
  )
}
