"use client"

import { useEffect } from "react"
import { useRouter } from "next/navigation"
import { useAuth } from "@/features/auth/hooks/use-auth"
import { Loader2 } from "lucide-react"

export default function RootPage() {
  const { isAuthenticated, isLoading } = useAuth()
  const router = useRouter()

  useEffect(() => {
    if (!isLoading) {
      router.replace(isAuthenticated ? "/home" : "/login")
    }
  }, [isLoading, isAuthenticated, router])

  return (
    <div className="flex min-h-screen items-center justify-center bg-black">
      <Loader2 className="h-8 w-8 animate-spin text-blue-500" />
    </div>
  )
}
