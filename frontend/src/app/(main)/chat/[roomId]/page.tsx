"use client"

import { useCallback, useState } from "react"
import { useParams, useRouter } from "next/navigation"
import { Hash, Link2, Shield, Flame, Users, Settings, AlertTriangle, ArrowLeft } from "lucide-react"
import { useRoom, useIsAdmin, usePopRoom } from "@/features/room/hooks/use-rooms"
import { useMessages } from "@/features/chat/hooks/use-messages"
import { MessageList } from "@/features/chat/components/message-list"
import { MessageInput } from "@/features/chat/components/message-input"
import { InviteLinkDialog } from "@/features/room/components/invite-link-dialog"
import { MembersDialog } from "@/features/room/components/members-dialog"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"

export default function ChatPage() {
  const params = useParams<{ roomId: string }>()
  const router = useRouter()
  const roomId = params.roomId

  const { data: roomData, isLoading: roomLoading, error: roomError } = useRoom(roomId)
  const { isAdmin } = useIsAdmin(roomId)
  const popRoom = usePopRoom(roomId)
  const {
    messages,
    isLoading: msgsLoading,
    error: msgsError,
    createMessage,
    editMessage,
    deleteMessage,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
  } = useMessages(roomId)

  const [inviteOpen, setInviteOpen] = useState(false)
  const [membersOpen, setMembersOpen] = useState(false)

  const room = roomData?.data
  const isPopped = !!room?.popped_at

  const handleSend = useCallback(
    async (content: string) => {
      try {
        await createMessage(content)
      } catch {
        // toast handled by useMessages
      }
    },
    [createMessage],
  )

  const handleEdit = useCallback(
    async (messageId: string, content: string) => {
      try {
        await editMessage({ messageId, data: { content } })
      } catch {
        // toast handled by useMessages
      }
    },
    [editMessage],
  )

  const handleDelete = useCallback(
    async (messageId: string) => {
      try {
        await deleteMessage(messageId)
      } catch {
        // toast handled by useMessages
      }
    },
    [deleteMessage],
  )

  if (roomLoading) {
    return (
      <div className="flex flex-col h-full">
        <div className="border-b border-gray-800 px-6 py-4">
          <Skeleton className="h-6 w-48" />
          <Skeleton className="h-4 w-24 mt-1" />
        </div>
        <div className="flex-1 p-4 space-y-4">
          {Array.from({ length: 6 }).map((_, i) => (
            <div key={i} className="flex gap-3">
              <Skeleton className="h-8 w-8 rounded-full shrink-0" />
              <div className="space-y-2 flex-1">
                <Skeleton className="h-4 w-24" />
                <Skeleton className="h-16 w-full max-w-md" />
              </div>
            </div>
          ))}
        </div>
      </div>
    )
  }

  if (roomError) {
    return (
      <div className="flex flex-col items-center justify-center h-full text-center px-4">
        <AlertTriangle className="h-12 w-12 text-red-400 mb-4" />
        <h2 className="text-lg font-semibold text-gray-100 mb-2">Failed to load room</h2>
        <p className="text-sm text-gray-500 mb-6 max-w-md">
          {roomError instanceof Error ? roomError.message : "This room may not exist or you may not have access."}
        </p>
        <Button variant="outline" onClick={() => router.push("/home")} className="gap-2">
          <ArrowLeft className="h-4 w-4" />
          Back to Home
        </Button>
      </div>
    )
  }

  return (
    <div className="flex flex-col h-full relative">
      <div className="flex items-center gap-2 border-b border-gray-800 px-6 py-3 shrink-0">
        <Hash className="h-5 w-5 text-gray-500" />
        <div className="flex-1 min-w-0">
          <h1 className="text-sm font-semibold text-gray-100">
            {room?.name ?? "Loading..."}
          </h1>
          {room?.description && (
            <p className="text-xs text-gray-500 truncate max-w-md">
              {room.description}
            </p>
          )}
        </div>
        {isPopped && (
          <span className="text-xs text-orange-400 bg-orange-400/10 px-2 py-0.5 rounded-full">
            Popped
          </span>
        )}
        <Button
          variant="ghost"
          size="icon"
          onClick={() => setMembersOpen(true)}
          title="Members"
        >
          <Users className="h-4 w-4" />
        </Button>
        {isAdmin && (
          <div className="flex items-center gap-1">
            <Shield className="h-4 w-4 text-yellow-500" />
            {!isPopped && (
              <Button
                variant="ghost"
                size="icon"
                onClick={() => popRoom.mutate()}
                disabled={popRoom.isPending}
                title="Pop Room"
              >
                <Flame className="h-4 w-4 text-orange-500" />
              </Button>
            )}
            <Button
              variant="ghost"
              size="icon"
              onClick={() => setInviteOpen(true)}
              title="Invite Link"
            >
              <Link2 className="h-4 w-4" />
            </Button>
          </div>
        )}
        <Button
          variant="ghost"
          size="icon"
          onClick={() => router.push(`/chat/${roomId}/settings`)}
          title="Room Settings"
        >
          <Settings className="h-4 w-4" />
        </Button>
      </div>

      <MessageList
        messages={messages}
        isLoading={msgsLoading}
        error={msgsError}
        hasNextPage={!!hasNextPage}
        isFetchingNextPage={isFetchingNextPage}
        fetchNextPage={fetchNextPage}
        onEdit={handleEdit}
        onDelete={handleDelete}
      />

      <div className="shrink-0">
        <MessageInput onSend={handleSend} disabled={isPopped} />
      </div>

      {isAdmin && (
        <InviteLinkDialog
          open={inviteOpen}
          onOpenChange={setInviteOpen}
          roomId={roomId}
        />
      )}

      <MembersDialog
        open={membersOpen}
        onOpenChange={setMembersOpen}
        roomId={roomId}
      />
    </div>
  )
}
