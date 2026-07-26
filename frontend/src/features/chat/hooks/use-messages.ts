"use client"

import { useCallback, useEffect, useRef, useState } from "react"
import {
  useInfiniteQuery,
  useMutation,
  useQueryClient,
} from "@tanstack/react-query"
import {
  fetchMessages,
  createMessage,
  updateMessage,
  deleteMessage,
} from "@/features/chat/api/chat-api"
import type {
  Message,
  CreateMessageRequest,
  UpdateMessageRequest,
  ServerEvent,
} from "@/types"
import { MessageType } from "@/types"
import { useAuth } from "@/features/auth/hooks/use-auth"
import { WsClient } from "@/services/websocket-client"

export function useMessages(roomId: string) {
  const { user } = useAuth()
  const queryClient = useQueryClient()
  const wsRef = useRef<WsClient | null>(null)
  const [onlineCount, setOnlineCount] = useState(0)

  const queryKey = ["messages", roomId]

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
    enabled: !!roomId,
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
              const data = pages[0].data.filter(
                (m) =>
                  !(m.id.startsWith("optimistic-") &&
                    m.author_id === msg.author_id &&
                    m.content === msg.content),
              )
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
    if (!roomId || !user?.id) return

    wsRef.current = new WsClient({
      roomId,
      onMessage: handleWsMessage,
    })
    wsRef.current.connect()

    return () => {
      wsRef.current?.disconnect()
      wsRef.current = null
    }
  }, [roomId, user?.id, handleWsMessage])

  const allMessages = messagesQuery.data?.pages.flatMap((page) => page.data).filter((m): m is Message => m != null) ?? []

  const createMsg = useMutation({
    mutationFn: (content: string) =>
      createMessage(roomId, {
        content,
        message_type: MessageType.TEXT,
      }),
    onMutate: async (content) => {
      await queryClient.cancelQueries({ queryKey })
      const previous = queryClient.getQueryData<typeof messagesQuery.data>(queryKey)

      const optimistic: Message = {
        id: `optimistic-${Date.now()}`,
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
    onError: (_err, _content, context) => {
      if (context?.previous) {
        queryClient.setQueryData(queryKey, context.previous)
      }
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
  })

  const deleteMsg = useMutation({
    mutationFn: (messageId: string) => deleteMessage(roomId, messageId),
  })

  const fetchNextPage = messagesQuery.fetchNextPage
  const hasNextPage = messagesQuery.hasNextPage
  const isFetchingNextPage = messagesQuery.isFetchingNextPage

  return {
    messages: allMessages,
    isLoading: messagesQuery.isLoading,
    error: messagesQuery.error,
    createMessage: createMsg.mutateAsync,
    editMessage: editMsg.mutateAsync,
    deleteMessage: deleteMsg.mutateAsync,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
    onlineCount,
  }
}
