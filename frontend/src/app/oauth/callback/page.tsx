"use client"

import { Suspense, useEffect } from "react"
import { useRouter, useSearchParams } from "next/navigation"
import { setAccessToken } from "@/lib/api-client"
import { useAuth } from "@/features/auth/hooks/use-auth"
import { Loader2 } from "lucide-react"

function CallbackHandler() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const { checkSession } = useAuth()

  useEffect(() => {
    const token = searchParams.get("token")
    const error = searchParams.get("error")

    if (error) {
      router.replace(`/login?error=${error}`)
      return
    }

    if (token) {
      setAccessToken(token)
      checkSession().then((ok) => {
        if (ok) {
          router.replace("/home")
        } else {
          router.replace("/login?error=oauth_session_failed")
        }
      })
      return
    }

    router.replace("/login?error=no_token")
  }, [searchParams, router, checkSession])

  return (
    <div className="flex flex-col items-center gap-4">
      <Loader2 className="h-8 w-8 animate-spin text-blue-500" />
      <p className="text-sm text-muted-foreground">Signing you in...</p>
    </div>
  )
}

export default function OAuthCallbackPage() {
  return (
    <div className="flex min-h-screen items-center justify-center">
      <Suspense
        fallback={
          <div className="flex flex-col items-center gap-4">
            <Loader2 className="h-8 w-8 animate-spin text-blue-500" />
            <p className="text-sm text-muted-foreground">Signing you in...</p>
          </div>
        }
      >
        <CallbackHandler />
      </Suspense>
    </div>
  )
}
