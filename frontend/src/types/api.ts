export interface ApiResponse<T> {
  code: string
  data: T
  message: string
  meta?: Record<string, unknown>
  request_id?: string
  success: boolean
}

export interface CursorMeta {
  next_cursor?: string
  has_more: boolean
}

export interface TokenResponse {
  token: string
}
