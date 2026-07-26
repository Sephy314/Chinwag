import { getAccessToken } from "@/lib/api-client"
import type { ServerEvent, WsMessageHandler } from "@/types"

type WsStatus = "disconnected" | "connecting" | "connected"

interface WsClientOptions {
  roomId: string
  onMessage: WsMessageHandler
  onStatusChange?: (status: WsStatus) => void
}

const WS_BASE = process.env.NEXT_PUBLIC_WS_URL ?? "ws://localhost:8000"

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
  private status: WsStatus = "disconnected"

  constructor(options: WsClientOptions) {
    this.roomId = options.roomId
    this.onMessage = options.onMessage
    this.onStatusChange = options.onStatusChange
  }

  connect() {
    if (this.ws?.readyState === WebSocket.OPEN || this.ws?.readyState === WebSocket.CONNECTING) {
      return
    }

    this.intentionalClose = false
    this.setStatus("connecting")

    const token = getAccessToken()
    if (!token) {
      this.scheduleReconnect()
      return
    }

    const url = `${WS_BASE}/chat/rooms/${this.roomId}/ws?token=${encodeURIComponent(token)}`
    this.ws = new WebSocket(url)

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
