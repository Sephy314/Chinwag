"use client"

import { useEffect } from "react"
import { AlertTriangle, RefreshCw, ArrowLeft } from "lucide-react"
import { useRouter } from "next/navigation"
import { Button } from "@/components/ui/button"

export default function MainLayoutError({
  error,
  reset,
}: {
  error: Error & { digest?: string }
  reset: () => void
}) {
  const router = useRouter()

  useEffect(() => {
    console.error("Main layout error:", error)
  }, [error])

  return (
    <div className="flex min-h-screen flex-col items-center justify-center px-4 text-center">
      <AlertTriangle className="h-12 w-12 text-red-400 mb-4" />
      <h2 className="text-lg font-semibold text-gray-100 mb-2">Something went wrong</h2>
      <p className="text-sm text-gray-500 mb-6 max-w-md">
        An error occurred while loading this page.
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
