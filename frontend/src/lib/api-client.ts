import type { ApiResponse } from "@/types"

const API_BASE = ""

let accessToken: string | null = null
let refreshPromise: Promise<string | null> | null = null
let refreshInProgress = false

export function setAccessToken(token: string | null) {
  accessToken = token
}

export function getAccessToken(): string | null {
  return accessToken
}

async function refreshAccessToken(): Promise<string | null> {
  if (refreshInProgress && refreshPromise) {
    return refreshPromise
  }

  refreshInProgress = true

  try {
    const res = await fetch(`${API_BASE}/auth/refresh`, {
      method: "POST",
      credentials: "include",
    })

    if (!res.ok) {
      accessToken = null
      return null
    }

    const body: ApiResponse<{ token: string }> = await res.json()

    if (body.success && body.data?.token) {
      accessToken = body.data.token
      return accessToken
    }

    accessToken = null
    return null
  } catch {
    accessToken = null
    return null
  } finally {
    refreshInProgress = false
    refreshPromise = null
  }
}

function isTokenExpired(token: string): boolean {
  try {
    const payload = JSON.parse(atob(token.split(".")[1]))
    return payload.exp * 1000 <= Date.now()
  } catch {
    return true
  }
}

async function getValidToken(): Promise<string | null> {
  if (accessToken && !isTokenExpired(accessToken)) {
    return accessToken
  }

  if (!refreshPromise) {
    refreshPromise = refreshAccessToken()
  }

  return refreshPromise
}

export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
    public data?: unknown,
  ) {
    super(message)
    this.name = "ApiError"
  }
}

interface RequestOptions {
  method?: string
  body?: unknown
  params?: Record<string, string | number | undefined>
  auth?: boolean
}

export async function apiRequest<T>(
  path: string,
  options: RequestOptions = {},
): Promise<ApiResponse<T>> {
  const { method = "GET", body, params, auth = true } = options

  let urlStr = `${API_BASE}${path}`
  if (params) {
    const searchParams = new URLSearchParams()
    Object.entries(params).forEach(([key, value]) => {
      if (value !== undefined && value !== "") {
        searchParams.set(key, String(value))
      }
    })
    const qs = searchParams.toString()
    if (qs) urlStr += `?${qs}`
  }

  const headers: Record<string, string> = {}
  if (body) {
    headers["Content-Type"] = "application/json"
  }

  if (auth) {
    const token = await getValidToken()
    if (token) {
      headers["Authorization"] = `Bearer ${token}`
    }
  }

  let res = await fetch(urlStr, {
    method,
    headers,
    body: body ? JSON.stringify(body) : undefined,
    credentials: "include",
  })

  if (res.status === 401 && auth) {
    const newToken = await refreshAccessToken()
    if (newToken) {
      headers["Authorization"] = `Bearer ${newToken}`
      res = await fetch(urlStr, {
        method,
        headers,
        body: body ? JSON.stringify(body) : undefined,
        credentials: "include",
      })
    }
  }

  const json: ApiResponse<T> = await res.json()

  if (!res.ok) {
    throw new ApiError(
      res.status,
      json.code ?? "UNKNOWN",
      json.message ?? "An unexpected error occurred",
      json.data,
    )
  }

  return json
}

export function apiGet<T>(path: string, params?: Record<string, string | number | undefined>) {
  return apiRequest<T>(path, { method: "GET", params })
}

export function apiPost<T>(path: string, body?: unknown, auth?: boolean) {
  return apiRequest<T>(path, { method: "POST", body, auth })
}



export function apiPut<T>(path: string, body?: unknown) {
  return apiRequest<T>(path, { method: "PUT", body })
}

export function apiDelete<T>(path: string) {
  return apiRequest<T>(path, { method: "DELETE" })
}

export function getErrorMessage(err: unknown): string {
  if (err instanceof ApiError) return err.message
  if (err instanceof Error) return err.message
  return "An unexpected error occurred"
}
