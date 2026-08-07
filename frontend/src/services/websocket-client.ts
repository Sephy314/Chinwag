import { apiPost } from "@/lib/api-client"
import type { ApiResponse, ServerEvent, WsMessageHandler } from "@/types"

type WsStatus = "disconnected" | "connecting" | "connected"

interface WsClientOptions {
  roomId: string
  onMessage: WsMessageHandler
  onStatusChange?: (status: WsStatus) => void
}

/**
 * Resolves the WebSocket base URL to the same backend address the API uses:
 * the page's own origin (http→ws, https→wss). src/proxy.ts rewrites the
 * /chat/rooms/:id/ws path to the gateway, so the browser talks to the same
 * host for both REST and WebSocket. The SSR fallback is never actually used
 * to open a socket.
 */
function resolveWsBase(): string {
  if (typeof window === "undefined") {
    return "ws://localhost:8000"
  }
  const protocol = window.location.protocol === "https:" ? "wss:" : "ws:"
  return `${protocol}//${window.location.host}`
}

const WS_BASE = resolveWsBase()

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

      const url = `${WS_BASE}/chat/rooms/${this.roomId}/ws?ticket=${encodeURIComponent(ticket)}`
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
