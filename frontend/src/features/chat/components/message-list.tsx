"use client"

import { useEffect, useRef, useState, useCallback } from "react"
import { Loader2, ChevronDown, AlertTriangle } from "lucide-react"
import type { Message } from "@/types"
import { MessageBubble } from "./message-bubble"
import { Button } from "@/components/ui/button"
import { useAuth } from "@/features/auth/hooks/use-auth"

interface MessageListProps {
  messages: Message[]
  isLoading: boolean
  error: Error | null
  hasNextPage: boolean
  isFetchingNextPage: boolean
  fetchNextPage: () => void
  onEdit: (messageId: string, content: string) => Promise<void>
  onDelete: (messageId: string) => Promise<void>
}

export function MessageList({
  messages,
  isLoading,
  error,
  hasNextPage,
  isFetchingNextPage,
  fetchNextPage,
  onEdit,
  onDelete,
}: MessageListProps) {
  const { user } = useAuth()
  const scrollRef = useRef<HTMLDivElement>(null)
  const bottomRef = useRef<HTMLDivElement>(null)
  const [autoScroll, setAutoScroll] = useState(true)
  const prevLengthRef = useRef(messages.length)

  const handleScroll = useCallback(() => {
    if (!scrollRef.current) return
    const { scrollTop, scrollHeight, clientHeight } = scrollRef.current
    const isNearBottom = scrollHeight - scrollTop - clientHeight < 200
    setAutoScroll(isNearBottom)
  }, [])

  useEffect(() => {
    if (autoScroll && bottomRef.current) {
      bottomRef.current.scrollIntoView({ behavior: "smooth" })
    }
  }, [messages.length, autoScroll])

  useEffect(() => {
    if (scrollRef.current && messages.length > prevLengthRef.current) {
      const wasAtTop =
        scrollRef.current.scrollHeight - scrollRef.current.scrollTop -
          scrollRef.current.clientHeight <
        200
      if (wasAtTop && bottomRef.current) {
        bottomRef.current.scrollIntoView({ behavior: "smooth" })
      }
    }
    prevLengthRef.current = messages.length
  }, [messages.length])

  if (isLoading) {
    return (
      <div className="flex flex-1 items-center justify-center">
        <Loader2 className="h-6 w-6 animate-spin text-gray-500" />
      </div>
    )
  }

  if (error) {
    return (
      <div className="flex flex-1 items-center justify-center text-center px-4">
        <div>
          <AlertTriangle className="h-8 w-8 text-red-400 mx-auto mb-2" />
          <p className="text-sm text-gray-400">Failed to load messages</p>
          <p className="text-xs text-gray-600 mt-1">{error.message}</p>
        </div>
      </div>
    )
  }

  return (
    <div
      ref={scrollRef}
      onScroll={handleScroll}
      className="flex-1 overflow-y-auto px-4 py-4 space-y-1"
      role="log"
      aria-label="Messages"
      aria-live="polite"
    >
      {hasNextPage && (
        <div className="flex justify-center py-2">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => fetchNextPage()}
            disabled={isFetchingNextPage}
          >
            {isFetchingNextPage ? (
              <Loader2 className="h-4 w-4 animate-spin mr-2" />
            ) : null}
            Load older messages
          </Button>
        </div>
      )}

      {messages.length === 0 && !isLoading && (
        <div className="flex flex-col items-center justify-center h-full text-center text-gray-500">
          <p className="text-sm">No messages yet</p>
          <p className="text-xs mt-1">Be the first to say something</p>
        </div>
      )}

      {[...messages].reverse().map((message) => (
        <MessageBubble
          key={message.id}
          message={message}
          isOwn={message.author_id === user?.id}
          onEdit={onEdit}
          onDelete={onDelete}
        />
      ))}

      {!autoScroll && messages.length > 0 && (
        <button
          onClick={() => {
            setAutoScroll(true)
            bottomRef.current?.scrollIntoView({ behavior: "smooth" })
          }}
          className="absolute bottom-20 left-1/2 -translate-x-1/2 rounded-full bg-gray-800 border border-gray-700 p-2 shadow-lg hover:bg-gray-700 transition-colors"
          aria-label="Scroll to bottom"
        >
          <ChevronDown className="h-4 w-4" />
        </button>
      )}

      <div ref={bottomRef} />
    </div>
  )
}
