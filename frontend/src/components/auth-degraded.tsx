"use client"

import { useState } from "react"
import { Loader2, WifiOff } from "lucide-react"
import { useAuth } from "@/features/auth/hooks/use-auth"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Avatar, AvatarFallback } from "@/components/ui/avatar"

export function AuthDegraded() {
  const { user, refreshSession, logout } = useAuth()
  const [retrying, setRetrying] = useState(false)

  const initials =
    user?.name
      ?.split(" ")
      .map((n) => n[0])
      .join("")
      .toUpperCase()
      .slice(0, 2) ?? "U"

  const handleRetry = async () => {
    setRetrying(true)
    try {
      await refreshSession()
    } finally {
      setRetrying(false)
    }
  }

  return (
    <div className="flex h-screen items-center justify-center bg-black px-4">
      <Card className="w-full max-w-md">
        <CardHeader className="text-center">
          <div className="flex justify-center mb-4">
            <div className="rounded-full bg-amber-500/10 p-3">
              <WifiOff className="h-8 w-8 text-amber-500" />
            </div>
          </div>
          <CardTitle>Service temporarily unavailable</CardTitle>
          <CardDescription>
            We couldn&apos;t reach the authentication service. You are viewing a read-only
            snapshot of your account — rooms and chat are unavailable until service is
            restored.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="mb-4 flex items-center gap-3 rounded-lg border border-gray-800 bg-gray-900/50 p-3">
            <Avatar className="h-9 w-9">
              <AvatarFallback>{initials}</AvatarFallback>
            </Avatar>
            <div className="min-w-0 flex-1">
              <p className="truncate text-sm font-medium text-gray-200">{user?.name}</p>
              <p className="truncate text-xs text-gray-500">{user?.email}</p>
            </div>
            <span className="shrink-0 text-xs uppercase tracking-wide text-amber-500">
              Read-only
            </span>
          </div>
          <div className="flex gap-2">
            <Button className="flex-1" onClick={handleRetry} disabled={retrying}>
              {retrying ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : null}
              Retry
            </Button>
            <Button variant="ghost" onClick={logout}>
              Sign out
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
