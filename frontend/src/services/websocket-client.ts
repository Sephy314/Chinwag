import { apiPost } from "@/lib/api-client"
import type { ApiResponse, ServerEvent, WsMessageHandler } from "@/types"

type WsStatus = "disconnected" | "connecting" | "connected"

interface WsClientOptions {
  roomId: string
  onMessage: WsMessageHandler
  onStatusChange?: (status: WsStatus) => void
}

/**
 * WebSocket origin.
 *
 * In production the browser connects to the same origin it loaded the page
 * from: the Ingress routes `/chat` (including the WebSocket upgrade) to the
 * gateway, so `ws://` + `location.host` needs no configuration.
 *
 * In local dev the frontend (Next.js, :3000) and the gateway (:8000) are
 * separate origins, so the gateway port is used instead.
 *
 * An explicit NEXT_PUBLIC_WS_URL still overrides this when the WebSocket
 * server lives on a different origin than the frontend.
 */
const WS_BASE = (() => {
  const explicit = process.env.NEXT_PUBLIC_WS_URL
  if (explicit) return explicit

  if (typeof window === "undefined") {
    // SSR guard; never used to open a real socket.
    return "ws://localhost:8000"
  }

  const scheme = window.location.protocol === "https:" ? "wss" : "ws"
  const port =
    window.location.port === "3000" ? "8000" : window.location.port
  const host = port
    ? `${window.location.hostname}:${port}`
    : window.location.hostname
  return `${scheme}://${host}`
})()

export class WsClient {
  private ws: WebSocket | null = null
  private roomId: string
  private onMessage: WsMessageHandler
  private onStatusChange?: (status: WsStatus) => void
  private reconnectAttempt = 0
  private maxReconnectAttempts = 10
  private reconnectTimeout: ReturnType<typeof setTimeout> | null = null
  private heartbeatInterval: ReturnType<typeof setInterval> | null = null
  private intentionalClose = false
  private connectInFlight = false
  private status: WsStatus = "disconnected"

  constructor(options: WsClientOptions) {
    this.roomId = options.roomId
    this.onMessage = options.onMessage
    this.onStatusChange = options.onStatusChange
  }

  connect() {
    if (
      this.connectInFlight ||
      this.ws?.readyState === WebSocket.OPEN ||
      this.ws?.readyState === WebSocket.CONNECTING
    ) {
      return
    }

    this.intentionalClose = false
    this.setStatus("connecting")
    this.connectInFlight = true
    this.openSocket()
  }

  /**
   * Fetches a short-lived, single-use WebSocket ticket from the API (the
   * request is authenticated with a DPoP proof) and opens the socket with the
   * ticket instead of the durable access token, which is never placed in the
   * URL.
   */
  private async openSocket() {
    try {
      const res = await apiPost<{ ticket: string }>(
        `/chat/rooms/${this.roomId}/ws-ticket`,
        undefined,
        true,
      )
      const ticket = res.data?.ticket

      if (this.intentionalClose) {
        this.connectInFlight = false
        return
      }

      if (!ticket) {
        this.connectInFlight = false
        this.scheduleReconnect()
        return
      }

      const url = `${WS_BASE}/api/chat/rooms/${this.roomId}/ws?ticket=${encodeURIComponent(ticket)}`
      this.ws = new WebSocket(url)
      this.connectInFlight = false

      this.ws.onopen = () => {
        this.reconnectAttempt = 0
        this.setStatus("connected")
        this.startHeartbeat()
      }

      this.ws.onmessage = (event: MessageEvent) => {
        try {
          const data = JSON.parse(event.data) as ServerEvent
          this.onMessage(data)
        } catch {
          // ignore malformed messages
        }
      }

      this.ws.onclose = () => {
        this.stopHeartbeat()
        this.setStatus("disconnected")
        if (!this.intentionalClose) {
          this.scheduleReconnect()
        }
      }

      this.ws.onerror = () => {
        // onclose will fire after this
      }
    } catch {
      this.connectInFlight = false
      if (!this.intentionalClose) {
        this.scheduleReconnect()
      }
    }
  }

  disconnect() {
    this.intentionalClose = true
    this.stopHeartbeat()
    if (this.reconnectTimeout) {
      clearTimeout(this.reconnectTimeout)
      this.reconnectTimeout = null
    }
    this.ws?.close()
    this.ws = null
    this.setStatus("disconnected")
  }

  send(data: Record<string, unknown>) {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(data))
    }
  }

  private startHeartbeat() {
    this.stopHeartbeat()
    this.heartbeatInterval = setInterval(() => {
      this.send({ type: "ping" })
    }, 25000)
  }

  private stopHeartbeat() {
    if (this.heartbeatInterval) {
      clearInterval(this.heartbeatInterval)
      this.heartbeatInterval = null
    }
  }

  private scheduleReconnect() {
    if (this.reconnectAttempt >= this.maxReconnectAttempts) return

    const delay = Math.min(1000 * 2 ** this.reconnectAttempt, 30000)
    this.reconnectAttempt++

    this.reconnectTimeout = setTimeout(() => {
      this.connect()
    }, delay)
  }

  private setStatus(status: WsStatus) {
    this.status = status
    this.onStatusChange?.(status)
  }

  getStatus(): WsStatus {
    return this.status
  }
}
