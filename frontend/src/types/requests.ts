import { MessageType, Role } from "./models"

export interface LoginRequest {
  email: string
  password: string
}

export interface RegisterRequest {
  name: string
  email: string
  password: string
}

export interface UpdateUserRequest {
  name?: string
  email?: string
  password?: string
}

export interface CreateRoomRequest {
  name: string
  description?: string
  max_members?: number
  pop_at?: string
}

export interface UpdateRoomRequest {
  name?: string
  description?: string
  max_members?: number
}

export interface CreateMessageRequest {
  content: string
  message_type: MessageType
}

export interface UpdateMessageRequest {
  content: string
}

export interface CreateInviteLinkRequest {
  single_use?: boolean
  ttl_hours?: number
}

export interface UpdateRoomMemberRequest {
  role: Role
}
