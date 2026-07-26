export interface User {
  id: string
  name: string
  email: string
}

export interface Room {
  id: string
  name: string
  description: string
  max_members: number
  owner_id: string
  role?: Role
  created_at: string
  updated_at: string
  deleted_at?: string
  pop_at?: string
  popped_at?: string
}

export enum Role {
  MEMBER = 0,
  ADMIN = 1,
}

export interface RoomMember {
  room_id: string
  user_id: string
  user_name: string
  role: Role
  joined_at: string
  left_at?: string
}

export enum MessageType {
  TEXT = 0,
  SYSTEM = 1,
  IMAGE = 2,
  FILE = 3,
  NOTICE = 4,
}

export interface Message {
  id: string
  room_id: string
  author_id: string
  author_name: string
  message_type: MessageType
  content: string
  created_at: string
  updated_at?: string
}

export interface InviteLink {
  token: string
  room_id: string
  expires_at: string
}
