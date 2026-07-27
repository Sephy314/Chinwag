"use client"

import { useEffect } from "react"
import { AlertTriangle, RefreshCw, ArrowLeft } from "lucide-react"
import { useRouter } from "next/navigation"
import { Button } from "@/components/ui/button"

export default function ChatRoomError({
  error,
  reset,
}: {
  error: Error & { digest?: string }
  reset: () => void
}) {
  const router = useRouter()

  useEffect(() => {
    console.error("Chat room error:", error)
  }, [error])

  return (
    <div className="flex flex-col items-center justify-center h-full text-center px-4">
      <AlertTriangle className="h-12 w-12 text-red-400 mb-4" />
      <h2 className="text-lg font-semibold text-gray-100 mb-2">Failed to load chat</h2>
      <p className="text-sm text-gray-500 mb-6 max-w-md">
        {error.message || "An error occurred while loading this chat room."}
      </p>
      <div className="flex gap-3">
        <Button variant="outline" onClick={() => router.push("/home")} className="gap-2">
          <ArrowLeft className="h-4 w-4" />
          Go home
        </Button>
        <Button onClick={reset} className="gap-2">
          <RefreshCw className="h-4 w-4" />
          Try again
        </Button>
      </div>
    </div>
  )
}
