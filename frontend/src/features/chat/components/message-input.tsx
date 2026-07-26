"use client"

import { useState, useRef, type KeyboardEvent } from "react"
import { Send, Loader2 } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Textarea } from "@/components/ui/textarea"

interface MessageInputProps {
  onSend: (content: string) => Promise<void>
  disabled?: boolean
}

export function MessageInput({ onSend, disabled }: MessageInputProps) {
  const [content, setContent] = useState("")
  const [isSending, setIsSending] = useState(false)
  const textareaRef = useRef<HTMLTextAreaElement>(null)

  const handleSend = async () => {
    const trimmed = content.trim()
    if (!trimmed || isSending) return

    setIsSending(true)
    try {
      await onSend(trimmed)
      setContent("")
      if (textareaRef.current) {
        textareaRef.current.style.height = "auto"
      }
    } finally {
      setIsSending(false)
      textareaRef.current?.focus()
    }
  }

  const handleKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault()
      handleSend()
    }
  }

  const handleInput = () => {
    if (textareaRef.current) {
      textareaRef.current.style.height = "auto"
      textareaRef.current.style.height = `${Math.min(textareaRef.current.scrollHeight, 120)}px`
    }
  }

  return (
    <div className="border-t border-gray-800 px-4 py-3">
      <div className="flex items-end gap-2">
        <Textarea
          ref={textareaRef}
          value={content}
          onChange={(e) => {
            setContent(e.target.value)
            handleInput()
          }}
          onKeyDown={handleKeyDown}
          placeholder={disabled ? "This room has been popped" : "Type a message..."}
          className="min-h-[40px] max-h-[120px] py-2.5"
          rows={1}
          aria-label="Message input"
          disabled={disabled}
        />
        <Button
          onClick={handleSend}
          disabled={!content.trim() || isSending || disabled}
          size="icon"
          className="h-10 w-10 shrink-0"
          aria-label="Send message"
        >
          {isSending ? (
            <Loader2 className="h-4 w-4 animate-spin" />
          ) : (
            <Send className="h-4 w-4" />
          )}
        </Button>
      </div>
    </div>
  )
}
