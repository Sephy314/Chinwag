"use client"

import { useEffect, useState } from "react"
import { useParams, useRouter } from "next/navigation"
import { useAuth } from "@/features/auth/hooks/use-auth"
import { useJoinRoomViaInvite } from "@/features/room/hooks/use-rooms"
import { ApiError } from "@/lib/api-client"
import { Loader2 } from "lucide-react"

export default function InvitePage() {
  const params = useParams<{ token: string }>()
  const router = useRouter()
  const { isAuthenticated, isLoading: authLoading, readOnly } = useAuth()
  const joinInvite = useJoinRoomViaInvite()
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (authLoading || !isAuthenticated || readOnly) return

    joinInvite.mutate(params.token, {
      onSuccess: (res) => {
        const roomId = res.data?.room_id
        if (roomId) {
          router.replace(`/chat/${roomId}`)
        } else {
          router.replace("/home")
        }
      },
      onError: (err) => {
        if (err instanceof ApiError) {
          switch (err.status) {
            case 404:
              setError("Invite link has expired or is invalid.")
              break
            case 409:
              setError("You are already a member of this room.")
              break
            case 410:
              setError("This room has been closed.")
              break
            default:
              setError(err.message)
          }
        } else {
          setError("Something went wrong. Please try again.")
        }
      },
    })
  }, [authLoading, isAuthenticated, readOnly, params.token, joinInvite, router])

  if (authLoading) {
    return (
      <div className="flex h-screen items-center justify-center bg-black">
        <Loader2 className="h-8 w-8 animate-spin text-blue-500" />
      </div>
    )
  }

  if (!isAuthenticated) {
    router.replace(`/login?redirect=/invite/${params.token}`)
    return null
  }

  if (readOnly) {
    return (
      <div className="flex h-screen items-center justify-center bg-black">
        <div className="text-center space-y-4">
          <p className="text-gray-400 text-sm">
            Authentication service is temporarily unavailable. Please try again later.
          </p>
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="flex h-screen items-center justify-center bg-black">
        <div className="text-center space-y-4">
          <p className="text-gray-400 text-sm">{error}</p>
          <button
            onClick={() => router.replace("/home")}
            className="text-blue-500 text-sm hover:underline"
          >
            Go to Home
          </button>
        </div>
      </div>
    )
  }

  return (
    <div className="flex h-screen items-center justify-center bg-black">
      <div className="flex items-center gap-2 text-gray-400 text-sm">
        <Loader2 className="h-5 w-5 animate-spin" />
        Joining room...
      </div>
    </div>
  )
}
