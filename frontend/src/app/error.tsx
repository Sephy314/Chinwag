"use client"

import { useEffect } from "react"
import { AlertTriangle, RefreshCw } from "lucide-react"
import { Button } from "@/components/ui/button"

export default function GlobalError({
  error,
  reset,
}: {
  error: Error & { digest?: string }
  reset: () => void
}) {
  useEffect(() => {
    console.error("Application error:", error)
  }, [error])

  return (
    <div className="flex min-h-screen flex-col items-center justify-center px-4 text-center">
      <AlertTriangle className="h-16 w-16 text-red-400 mb-6" />
      <h1 className="text-2xl font-bold text-gray-100 mb-2">Something went wrong</h1>
      <p className="text-gray-500 mb-8 max-w-md">
        An unexpected error occurred. Please try again.
      </p>
      <Button onClick={reset} className="gap-2">
        <RefreshCw className="h-4 w-4" />
        Try again
      </Button>
    </div>
  )
}
