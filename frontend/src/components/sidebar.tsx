"use client"

import { useState } from "react"
import Link from "next/link"
import { usePathname, useRouter } from "next/navigation"
import {
  MessageSquare,
  Plus,
  LogOut,
  User,
  Settings,
  Menu,
  X,
  Hash,
  Loader2,
} from "lucide-react"
import { cn } from "@/lib/utils"
import { useAuth } from "@/features/auth/hooks/use-auth"
import { useRooms } from "@/features/room/hooks/use-rooms"
import { Button } from "@/components/ui/button"
import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import { CreateRoomDialog } from "@/features/room/components/create-room-dialog"
import { ScrollArea } from "@/components/ui/scroll-area"

export function Sidebar() {
  const { user, logout } = useAuth()
  const { rooms, isLoading } = useRooms()
  const pathname = usePathname()
  const router = useRouter()
  const [sidebarOpen, setSidebarOpen] = useState(false)
  const [createOpen, setCreateOpen] = useState(false)

  const currentRoomId = pathname.startsWith("/chat/") ? pathname.split("/")[2] : null

  const initials = user?.name
    ?.split(" ")
    .map((n) => n[0])
    .join("")
    .toUpperCase()
    .slice(0, 2) ?? "U"

  return (
    <>
      <button
        className="fixed top-4 left-4 z-40 flex items-center justify-center rounded-lg bg-gray-900 p-2.5 border border-gray-800 lg:hidden"
        onClick={() => setSidebarOpen(!sidebarOpen)}
        aria-label={sidebarOpen ? "Close sidebar" : "Open sidebar"}
      >
        {sidebarOpen ? <X className="h-5 w-5" /> : <Menu className="h-5 w-5" />}
      </button>

      {sidebarOpen && (
        <div
          className="fixed inset-0 z-30 bg-black/50 lg:hidden"
          onClick={() => setSidebarOpen(false)}
        />
      )}

      <aside
        className={cn(
          "fixed inset-y-0 left-0 z-30 flex w-64 flex-col border-r border-gray-800 bg-black transition-transform duration-200 lg:static lg:translate-x-0",
          sidebarOpen ? "translate-x-0" : "-translate-x-full",
        )}
      >
        <div className="flex h-14 items-center justify-between border-b border-gray-800 px-4">
          <Link href="/home" className="flex items-center gap-2">
            <MessageSquare className="h-5 w-5 text-blue-500" />
            <span className="font-semibold text-gray-100">Chinwag</span>
          </Link>
        </div>

        <div className="flex items-center justify-between px-4 py-3 border-b border-gray-800">
          <span className="text-xs font-medium uppercase tracking-wider text-gray-500">
            Rooms
          </span>
          <Button
            variant="ghost"
            size="icon"
            className="h-6 w-6"
            onClick={() => setCreateOpen(true)}
            aria-label="Create room"
          >
            <Plus className="h-4 w-4" />
          </Button>
        </div>

        <ScrollArea className="flex-1 px-2 py-2">
          {isLoading ? (
            <div className="flex items-center justify-center py-8">
              <Loader2 className="h-5 w-5 animate-spin text-gray-500" />
            </div>
          ) : rooms.length === 0 ? (
            <div className="px-2 py-8 text-center">
              <p className="text-sm text-gray-500">No rooms yet</p>
              <p className="text-xs text-gray-600 mt-1">
                Create one to get started
              </p>
            </div>
          ) : (
            rooms.map((room) => {
              const isActive = currentRoomId === room.id
              return (
                <Link
                  key={room.id}
                  href={`/chat/${room.id}`}
                  onClick={() => setSidebarOpen(false)}
                  className={cn(
                    "flex items-center gap-3 rounded-lg px-3 py-2 text-sm transition-colors",
                    isActive
                      ? "bg-gray-800 text-gray-100"
                      : "text-gray-400 hover:bg-gray-900 hover:text-gray-200",
                  )}
                >
                  <Hash className="h-4 w-4 shrink-0" />
                  <span className="truncate">{room.name}</span>
                </Link>
              )
            })
          )}
        </ScrollArea>

        <div className="border-t border-gray-800 p-3">
          <div className="flex items-center gap-3 rounded-lg px-2 py-2">
            <Avatar className="h-8 w-8">
              <AvatarFallback>{initials}</AvatarFallback>
            </Avatar>
            <div className="flex-1 min-w-0">
              <p className="text-sm font-medium text-gray-200 truncate">
                {user?.name}
              </p>
            </div>
            <div className="flex gap-1">
              <Button
                variant="ghost"
                size="icon"
                className="h-8 w-8"
                onClick={() => router.push("/profile")}
                aria-label="Profile"
              >
                <User className="h-4 w-4" />
              </Button>
              <Button
                variant="ghost"
                size="icon"
                className="h-8 w-8"
                onClick={() => router.push("/settings")}
                aria-label="Settings"
              >
                <Settings className="h-4 w-4" />
              </Button>
              <Button
                variant="ghost"
                size="icon"
                className="h-8 w-8 text-red-400 hover:text-red-300"
                onClick={logout}
                aria-label="Logout"
              >
                <LogOut className="h-4 w-4" />
              </Button>
            </div>
          </div>
        </div>
      </aside>

      <CreateRoomDialog open={createOpen} onOpenChange={setCreateOpen} />
    </>
  )
}
