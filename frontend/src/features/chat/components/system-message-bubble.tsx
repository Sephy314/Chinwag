"use client"

import { LogIn, LogOut } from "lucide-react"
import type { SystemMessage } from "@/types"

interface SystemMessageBubbleProps {
  message: SystemMessage
}

export function SystemMessageBubble({ message }: SystemMessageBubbleProps) {
  const time = new Date(message.timestamp).toLocaleTimeString([], {
    hour: "2-digit",
    minute: "2-digit",
  })

  if (message.type === "user_joined") {
    return (
      <div className="flex items-center justify-center gap-2 py-2">
        <div className="flex items-center gap-1.5 text-xs text-gray-500">
          <LogIn className="h-3 w-3 text-green-500" />
          <span>
            <span className="font-medium text-gray-400">{message.user_name}</span>
            {" "}joined the chat
          </span>
          <span className="text-gray-600">{time}</span>
        </div>
      </div>
    )
  }

  return (
    <div className="flex items-center justify-center gap-2 py-2">
      <div className="flex items-center gap-1.5 text-xs text-gray-500">
        <LogOut className="h-3 w-3 text-red-500" />
        <span>
          <span className="font-medium text-gray-400">{message.user_name}</span>
          {" "}left the chat
        </span>
        <span className="text-gray-600">{time}</span>
      </div>
    </div>
  )
}
