import type { ApiResponse } from "@/types"
import { createDPoPProof } from "./dpop"

const API_BASE = "/api"

let accessToken: string | null = null
let refreshPromise: Promise<string | null> | null = null
let dpopNonce: string | null = null
/** Serialize DPoP requests so single-use nonces are not raced. */
let dpopGate: Promise<unknown> = Promise.resolve()

export function setAccessToken(token: string | null) {
  accessToken = token
}

export function getAccessToken(): string | null {
  return accessToken
}

function setDPoPNonce(nonce: string | null) {
  dpopNonce = nonce
}

function captureDPoPNonce(res: Response) {
  const nonce = res.headers.get("DPoP-Nonce")
  if (nonce) setDPoPNonce(nonce)
  return nonce
}

async function fetchWithProof(
  url: string,
  method: string,
  headers: Record<string, string>,
  body: string | undefined,
  nonce: string | undefined,
): Promise<Response> {
  const { proof } = await createDPoPProof({ method, url, nonce })
  return fetch(url, {
    method,
    headers: { ...headers, DPoP: proof },
    body,
    credentials: "include",
  })
}

async function isUseNonceResponse(res: Response): Promise<boolean> {
  if (res.status !== 400) return false
  try {
    const data: { code?: string } = await res.clone().json()
    return data?.code === "use_dpop_nonce"
  } catch {
    return false
  }
}

function withDPoPGate<T>(fn: () => Promise<T>): Promise<T> {
  const next = dpopGate.then(fn, fn)
  dpopGate = next.then(
    () => undefined,
    () => undefined,
  )
  return next
}

/**
 * Fetches with a DPoP proof. Always stores DPoP-Nonce from the response
 * (including successes). On `use_dpop_nonce`, retries with that challenge
 * nonce (RFC 9449 §8.3). Serialized so concurrent callers do not burn the
 * same single-use nonce.
 */
async function doFetch(
  url: string,
  method: string,
  headers: Record<string, string>,
  body: string | undefined,
): Promise<Response> {
  return withDPoPGate(async () => {
    let res = await fetchWithProof(url, method, headers, body, dpopNonce ?? undefined)
    let challengeNonce = captureDPoPNonce(res)

    for (let attempt = 0; attempt < 2 && (await isUseNonceResponse(res)); attempt++) {
      if (!challengeNonce) break
      res = await fetchWithProof(url, method, headers, body, challengeNonce)
      challengeNonce = captureDPoPNonce(res)
    }

    return res
  })
}

async function refreshAccessToken(): Promise<string | null> {
  if (refreshPromise) {
    return refreshPromise
  }

  refreshPromise = (async () => {
    try {
      const res = await doFetch(`${API_BASE}/auth/refresh`, "POST", {}, undefined)

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
      refreshPromise = null
    }
  })()

  return refreshPromise
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

  return refreshAccessToken()
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
      headers["Authorization"] = `DPoP ${token}`
    }
  }

  let res = await doFetch(urlStr, method, headers, body ? JSON.stringify(body) : undefined)

  if (res.status === 401 && auth) {
    const newToken = await refreshAccessToken()
    if (newToken) {
      headers["Authorization"] = `DPoP ${newToken}`
      res = await doFetch(urlStr, method, headers, body ? JSON.stringify(body) : undefined)
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
