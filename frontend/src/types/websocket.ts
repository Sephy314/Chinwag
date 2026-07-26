import { Message } from "./models"

export interface WsEvent {
  type: WsEventType
  data: unknown
}

export type WsEventType = "new_message" | "updated_message" | "deleted_message" | "pong"

export interface NewMessageEvent {
  type: "new_message"
  data: Message
}

export interface UpdatedMessageEvent {
  type: "updated_message"
  data: Message
}

export interface DeletedMessageEvent {
  type: "deleted_message"
  data: { id: string; room_id: string }
}

export interface PongEvent {
  type: "pong"
}

export type ServerEvent = NewMessageEvent | UpdatedMessageEvent | DeletedMessageEvent | PongEvent

export type WsMessageHandler = (event: ServerEvent) => void
