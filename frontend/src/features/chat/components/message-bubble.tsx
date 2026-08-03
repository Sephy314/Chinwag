"use client"

import { useState } from "react"
import { MoreHorizontal, Pencil, Trash2, Check, X } from "lucide-react"
import { toast } from "sonner"
import type { Message } from "@/types"
import { MessageType } from "@/types"
import { cn } from "@/lib/utils"
import { getErrorMessage } from "@/lib/api-client"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Button } from "@/components/ui/button"
import { Textarea } from "@/components/ui/textarea"

interface MessageBubbleProps {
  message: Message
  isOwn: boolean
  readOnly?: boolean
  onEdit: (messageId: string, content: string) => Promise<void>
  onDelete: (messageId: string) => Promise<void>
}

export function MessageBubble({ message, isOwn, readOnly, onEdit, onDelete }: MessageBubbleProps) {
  const [isEditing, setIsEditing] = useState(false)
  const [editContent, setEditContent] = useState(message.content)
  const [isSaving, setIsSaving] = useState(false)
  const isOptimistic = message.id.startsWith("optimistic-")

  const isDeleted = message.content === "" && message.message_type === MessageType.TEXT

  const handleSaveEdit = async () => {
    if (!editContent.trim() || editContent === message.content) {
      setIsEditing(false)
      return
    }
    setIsSaving(true)
    try {
      await onEdit(message.id, editContent)
      setIsEditing(false)
    } catch (err) {
      toast.error(`Failed to edit: ${getErrorMessage(err)}`)
    } finally {
      setIsSaving(false)
    }
  }

  const handleCancelEdit = () => {
    setEditContent(message.content)
    setIsEditing(false)
  }

  const handleDelete = async () => {
    try {
      await onDelete(message.id)
    } catch (err) {
      toast.error(`Failed to delete: ${getErrorMessage(err)}`)
    }
  }

  const time = new Date(message.created_at).toLocaleTimeString([], {
    hour: "2-digit",
    minute: "2-digit",
  })

  if (isDeleted) return null

  return (
    <div
      className={cn(
        "group flex gap-2 px-2 py-1 rounded-lg transition-colors hover:bg-gray-900/50",
        isOwn ? "flex-row-reverse" : "flex-row",
        isOptimistic && "opacity-60",
      )}
      role="listitem"
    >
      <div className={cn("flex flex-col max-w-[75%]", isOwn ? "items-end" : "items-start")}>
        {!isOwn && (
          <span className="text-xs text-gray-500 mb-0.5 px-1">
            {message.author_name}
          </span>
        )}

        <div
          className={cn(
            "rounded-2xl px-4 py-2 text-sm",
            isOwn
              ? "bg-blue-600 text-white rounded-br-md"
              : "bg-gray-800 text-gray-200 rounded-bl-md",
          )}
        >
          {isEditing ? (
            <div className="space-y-2 min-w-[200px]">
              <Textarea
                value={editContent}
                onChange={(e) => setEditContent(e.target.value)}
                className="min-h-[60px] text-sm"
                autoFocus
                onKeyDown={(e) => {
                  if (e.key === "Enter" && !e.shiftKey) {
                    e.preventDefault()
                    handleSaveEdit()
                  }
                  if (e.key === "Escape") {
                    handleCancelEdit()
                  }
                }}
              />
              <div className="flex justify-end gap-2">
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={handleCancelEdit}
                  disabled={isSaving}
                  aria-label="Cancel edit"
                >
                  <X className="h-3 w-3" />
                </Button>
                <Button
                  variant="default"
                  size="sm"
                  onClick={handleSaveEdit}
                  disabled={isSaving || !editContent.trim()}
                  aria-label="Save edit"
                >
                  <Check className="h-3 w-3" />
                </Button>
              </div>
            </div>
          ) : (
            <p className="whitespace-pre-wrap break-words">{message.content}</p>
          )}
        </div>

        <div
          className={cn(
            "flex items-center gap-2 mt-0.5",
            isOwn ? "flex-row-reverse" : "flex-row",
          )}
        >
          <span className="text-[10px] text-gray-600">{time}</span>
          {message.updated_at && message.updated_at !== message.created_at && (
            <span className="text-[10px] text-gray-600">(edited)</span>
          )}
        </div>
      </div>

      {isOwn && !isOptimistic && !isEditing && !readOnly && (
        <div className="opacity-0 group-hover:opacity-100 transition-opacity self-start pt-1">
          <DropdownMenu>
            <DropdownMenuTrigger
              className="inline-flex items-center justify-center h-7 w-7 rounded-lg text-gray-400 hover:text-gray-200 hover:bg-gray-800 transition-colors"
              aria-label="Message actions"
            >
              <MoreHorizontal className="h-3.5 w-3.5" />
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem onClick={() => setIsEditing(true)}>
                <Pencil className="h-3.5 w-3.5 mr-2" />
                Edit
              </DropdownMenuItem>
              <DropdownMenuItem onClick={handleDelete} className="text-red-400">
                <Trash2 className="h-3.5 w-3.5 mr-2" />
                Delete
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      )}
    </div>
  )
}
