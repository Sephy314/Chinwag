"use client"

import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import {
  useInfiniteQuery,
  useMutation,
  useQueryClient,
  type InfiniteData,
} from "@tanstack/react-query"
import { toast } from "sonner"
import {
  fetchMessages,
  createMessage,
  updateMessage,
  deleteMessage,
} from "@/features/chat/api/chat-api"
import type { ApiResponse, Message, UpdateMessageRequest, ServerEvent } from "@/types"
import { MessageType } from "@/types"
import { useAuth } from "@/features/auth/hooks/use-auth"
import { WsClient } from "@/services/websocket-client"
import { getErrorMessage } from "@/lib/api-client"

interface Cursor {
  created_at: string
  id: string
}

function seedPage(data: Message[]): ApiResponse<Message[]> {
  return { success: true, code: "OK", message: "", data }
}

type MessagePage = ApiResponse<Message[]>
type MessageQueryData = InfiniteData<MessagePage, string | undefined>

export function useMessages(roomId: string) {
  const { user, readOnly } = useAuth()
  const queryClient = useQueryClient()
  const wsRef = useRef<WsClient | null>(null)
  const lastMessageCursorRef = useRef<Cursor | null>(null)
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
      queryClient.setQueryData<MessageQueryData>(queryKey, (old): MessageQueryData | undefined => {
        // The very first message of a new room often arrives over WS before the
        // initial query has resolved (old is still undefined). Seed a fresh page
        // so it is not dropped.
        const msg = event.type === "new_message" ? (event.data as Message | null) : null
        if (!old || old.pages.length === 0 || !Array.isArray(old.pages[0]?.data)) {
          if (!msg || !msg.id) return old
          lastMessageCursorRef.current = {
            created_at: msg.created_at,
            id: msg.id,
          }
          return { pages: [seedPage([msg])], pageParams: [undefined] as (string | undefined)[] }
        }

        const pages = [...old.pages]

        switch (event.type) {
          case "new_message": {
            if (msg && msg.id) {
              const data = pages[0].data.filter((m) => m.id !== msg.id)
              pages[0] = {
                ...pages[0],
                data: [msg, ...data],
              }
              // Track the newest message's cursor for reconnect gap fill
              lastMessageCursorRef.current = {
                created_at: msg.created_at,
                id: msg.id,
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
      onStatusChange: async (status) => {
        setWsConnected(status === "connected")
        if (status === "disconnected" && wsRef.current) {
          toast.error("Connection lost. Reconnecting...", { id: "ws-reconnect" })
        }
        if (status === "connected") {
          toast.dismiss("ws-reconnect")
          // Refetch messages after the last known message to fill any reconnection gap
          if (lastMessageCursorRef.current && messagesQuery.data?.pages[0]?.data.length) {
            try {
              const lastMsg = lastMessageCursorRef.current
              const afterCursor = btoa(JSON.stringify({ created_at: lastMsg.created_at, id: lastMsg.id }))
              const resyncedMessages = await fetchMessages(roomId, undefined, afterCursor)

              if (resyncedMessages.data && resyncedMessages.data.length > 0) {
                // Merge resynced messages with existing data, deduplicating by id
queryClient.setQueryData<MessageQueryData>(queryKey, (old): MessageQueryData | undefined => {
                  if (!old || old.pages.length === 0 || !Array.isArray(old.pages[0]?.data)) return old
                  const pages = [...old.pages]
                  const existingIds = new Set(pages[0].data.map((m) => m.id))
                  const newMessages = resyncedMessages.data.filter((m) => !existingIds.has(m.id))
                  if (newMessages.length > 0) {
                    pages[0] = {
                      ...pages[0],
                      data: [...newMessages, ...pages[0].data],
                    }
                  }
                  return { ...old, pages }
                })
              }
            } catch (err) {
              console.error("Failed to refetch messages after reconnect:", err)
            }
          }
        }
      },
    })
    wsRef.current.connect()

    return () => {
      wsRef.current?.disconnect()
      wsRef.current = null
    }
  }, [roomId, user?.id, readOnly, handleWsMessage, messagesQuery.data, queryClient, queryKey])

  // Track the newest message's cursor for reconnect gap fill
  const allMessages = useMemo(
    () => messagesQuery.data?.pages.flatMap((page) => page.data ?? []) ?? [],
    [messagesQuery.data],
  )

  // Track the newest message's cursor for reconnect gap fill
  useEffect(() => {
    if (allMessages.length > 0) {
      const newestMsg = allMessages[0]
      lastMessageCursorRef.current = {
        created_at: newestMsg.created_at,
        id: newestMsg.id,
      }
    }
  }, [allMessages])

  const createMsg = useMutation({
    mutationFn: ({ id, content }: { id: string; content: string }) =>
      createMessage(roomId, {
        id,
        content,
        message_type: MessageType.TEXT,
      }),
    onMutate: async ({ id, content }) => {
      await queryClient.cancelQueries({ queryKey })
      const previous = queryClient.getQueryData<MessageQueryData | undefined>(queryKey)

      const optimistic: Message = {
        id,
        room_id: roomId,
        author_id: user!.id,
        author_name: user!.name,
        message_type: MessageType.TEXT,
        content,
        created_at: new Date().toISOString(),
      }

      queryClient.setQueryData<MessageQueryData>(queryKey, (old): MessageQueryData | undefined => {
        if (!old || old.pages.length === 0 || !old.pages[0] || !Array.isArray(old.pages[0].data)) {
          return { pages: [seedPage([optimistic])], pageParams: [undefined] as (string | undefined)[] }
        }
        const pages = [...old.pages]
        pages[0] = {
          ...pages[0],
          data: [optimistic, ...pages[0].data],
        }
        return { ...old, pages }
      })

      return { previous }
    },
    onSuccess: () => {
      // Reconcile with the projection so the optimistic message is never left
      // out if the initial query was canceled before it resolved.
      queryClient.invalidateQueries({ queryKey })
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
    wsConnected,
  }
}
