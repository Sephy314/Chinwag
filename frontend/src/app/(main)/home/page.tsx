"use client"

import { MessageSquare } from "lucide-react"

export default function HomePage() {
  return (
    <div className="flex h-full items-center justify-center">
      <div className="text-center max-w-sm">
        <div className="flex justify-center mb-6">
          <div className="rounded-full bg-blue-600/10 p-4">
            <MessageSquare className="h-12 w-12 text-blue-500" />
          </div>
        </div>
        <h1 className="text-2xl font-semibold text-gray-100 mb-2">
          Welcome to Chinwag
        </h1>
        <p className="text-gray-500 text-sm leading-relaxed">
          Select a room from the sidebar to start chatting, or create a new room
          to begin a conversation.
        </p>
      </div>
    </div>
  )
}
