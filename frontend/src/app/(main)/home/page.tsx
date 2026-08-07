"use client"

import { Logo } from "@/components/logo"

export default function HomePage() {
  return (
    <div className="flex h-full items-center justify-center">
      <div className="text-center max-w-sm">
        <div className="flex justify-center mb-6">
          <Logo className="h-20 w-20" />
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
