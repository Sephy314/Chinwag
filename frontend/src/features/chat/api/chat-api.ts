import { apiGet, apiPost, apiPut, apiDelete } from "@/lib/api-client"
import { API_PATHS } from "@/lib/api-paths"
import type { Message } from "@/types"
import type { CreateMessageRequest, UpdateMessageRequest } from "@/types"

export async function fetchMessages(
  roomId: string,
  cursor?: string,
  after?: string,
) {
  return apiGet<Message[]>(API_PATHS.chat.messages(roomId), {
    cursor,
    after,
  })
}

export async function fetchMessage(roomId: string, messageId: string) {
  return apiGet<Message>(API_PATHS.chat.message(roomId, messageId))
}

export async function createMessage(
  roomId: string,
  data: CreateMessageRequest,
) {
  return apiPost<Message>(API_PATHS.chat.messages(roomId), data)
}

export async function updateMessage(
  roomId: string,
  messageId: string,
  data: UpdateMessageRequest,
) {
  return apiPut<Message>(API_PATHS.chat.message(roomId, messageId), data)
}

export async function deleteMessage(
  roomId: string,
  messageId: string,
) {
  return apiDelete<void>(API_PATHS.chat.message(roomId, messageId))
}
