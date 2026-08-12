// Admin panel types — mirror the backend admin response shapes.
export type AccountRole = "USER" | "MANAGER" | "ADMIN"

export interface AdminUser {
  id: string
  name: string
  email: string
  role: string
  provider?: string
  created_at: string
  updated_at: string
  deleted_at?: string
}

export interface AdminCreateUserInput {
  name: string
  email: string
  password: string
  role?: string
}

export interface AdminUpdateUserInput {
  name?: string
  email?: string
  password?: string
}

export interface AdminSession {
  lineage_id: string
  user_id: string
  created_at: number
  used: boolean
  revoked: boolean
  jkt?: string
  tokens: number
}

export interface AuditEvent {
  id: string
  admin_id: string
  action: string
  target_type: string
  target_id: string
  metadata?: Record<string, unknown>
  created_at: string
}

export interface AdminStats {
  count: number
}

export interface AdminRoomStats {
  total_rooms: number
}

export interface AdminMessageStats {
  total_messages: number
}

export interface ListResponse<T> {
  items: T[]
  nextCursor?: string
  hasMore: boolean
}
