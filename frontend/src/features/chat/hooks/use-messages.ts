"use client"

import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import {
  useInfiniteQuery,
  useMutation,
  useQueryClient,
} from "@tanstack/react-query"
import { toast } from "sonner"
import {
  fetchMessages,
  createMessage,
  updateMessage,
  deleteMessage,
} from "@/features/chat/api/chat-api"
import type { Message, UpdateMessageRequest, ServerEvent } from "@/types"
import { MessageType } from "@/types"
import { useAuth } from "@/features/auth/hooks/use-auth"
import { WsClient } from "@/services/websocket-client"
import { getErrorMessage } from "@/lib/api-client"

export function useMessages(roomId: string) {
  const { user, readOnly } = useAuth()
  const queryClient = useQueryClient()
  const wsRef = useRef<WsClient | null>(null)
  const [onlineCount, setOnlineCount] = useState(0)
  const [wsConnected, setWsConnected] = useState(false)

  const queryKey = useMemo(() => ["messages", roomId] as const, [roomId])

  const messagesQuery = useInfiniteQuery({
    queryKey,
    queryFn: ({ pageParam }) => fetchMessages(roomId, pageParam as string | undefined),
    getNextPageParam: (lastPage) => {
      const meta = lastPage.meta as { next_cursor?: string; has_more?: boolean } | undefined
      if (meta?.has_more && meta?.next_cursor) {
        return meta.next_cursor
      }
      return undefined
    },
    initialPageParam: undefined as string | undefined,
    enabled: !!roomId && !readOnly,
  })

  const handleWsMessage = useCallback(
    (event: ServerEvent) => {
      queryClient.setQueryData<typeof messagesQuery.data>(queryKey, (old) => {
        if (!old || old.pages.length === 0) return old

        const pages = [...old.pages]

        switch (event.type) {
          case "new_message": {
            const msg = event.data as Message
            if (msg && msg.id) {
              const data = pages[0].data.filter((m) => m.id !== msg.id)
              pages[0] = {
                ...pages[0],
                data: [msg, ...data],
              }
            }
            break
          }
          case "updated_message": {
            const updated = event.data as Message
            if (updated && updated.id) {
              pages[0] = {
                ...pages[0],
                data: pages[0].data.map((m) =>
                  m.id === updated.id ? updated : m,
                ),
              }
            }
            break
          }
          case "deleted_message": {
            const deleted = event.data as { id: string }
            if (deleted && deleted.id) {
              pages[0] = {
                ...pages[0],
                data: pages[0].data.filter((m) => m.id !== deleted.id),
              }
            }
            break
          }
        }

        return { ...old, pages }
      })
    },
    [queryClient, queryKey],
  )

  useEffect(() => {
    if (!roomId || !user?.id || readOnly) return

    wsRef.current = new WsClient({
      roomId,
      onMessage: handleWsMessage,
      onStatusChange: (status) => {
        setWsConnected(status === "connected")
        if (status === "disconnected" && wsRef.current) {
          toast.error("Connection lost. Reconnecting...", { id: "ws-reconnect" })
        }
        if (status === "connected") {
          toast.dismiss("ws-reconnect")
        }
      },
    })
    wsRef.current.connect()

    return () => {
      wsRef.current?.disconnect()
      wsRef.current = null
    }
  }, [roomId, user?.id, readOnly, handleWsMessage])

  const allMessages = messagesQuery.data?.pages.flatMap((page) => page.data).filter((m): m is Message => m != null) ?? []

  const createMsg = useMutation({
    mutationFn: ({ id, content }: { id: string; content: string }) =>
      createMessage(roomId, {
        id,
        content,
        message_type: MessageType.TEXT,
      }),
    onMutate: async ({ id, content }) => {
      await queryClient.cancelQueries({ queryKey })
      const previous = queryClient.getQueryData<typeof messagesQuery.data>(queryKey)

      const optimistic: Message = {
        id,
        room_id: roomId,
        author_id: user!.id,
        author_name: user!.name,
        message_type: MessageType.TEXT,
        content,
        created_at: new Date().toISOString(),
      }

      queryClient.setQueryData<typeof messagesQuery.data>(queryKey, (old) => {
        if (!old || old.pages.length === 0 || !old.pages[0]?.data) return old
        const pages = [...old.pages]
        pages[0] = {
          ...pages[0],
          data: [optimistic, ...pages[0].data],
        }
        return { ...old, pages }
      })

      return { previous }
    },
    onError: (_err, _message, context) => {
      if (context?.previous) {
        queryClient.setQueryData(queryKey, context.previous)
      }
      toast.error("Failed to send message")
    },
  })

  const editMsg = useMutation({
    mutationFn: ({
      messageId,
      data,
    }: {
      messageId: string
      data: UpdateMessageRequest
    }) => updateMessage(roomId, messageId, data),
    onError: (err) => {
      toast.error(`Failed to edit message: ${getErrorMessage(err)}`)
    },
  })

  const deleteMsg = useMutation({
    mutationFn: (messageId: string) => deleteMessage(roomId, messageId),
    onError: (err) => {
      toast.error(`Failed to delete message: ${getErrorMessage(err)}`)
    },
  })

  const fetchNextPage = messagesQuery.fetchNextPage
  const hasNextPage = messagesQuery.hasNextPage
  const isFetchingNextPage = messagesQuery.isFetchingNextPage

  return {
    messages: allMessages,
    isLoading: messagesQuery.isLoading,
    error: messagesQuery.error,
    createMessage: (content: string) =>
      createMsg.mutateAsync({ id: crypto.randomUUID(), content }),
    editMessage: editMsg.mutateAsync,
    deleteMessage: deleteMsg.mutateAsync,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
    onlineCount,
    wsConnected,
  }
}
