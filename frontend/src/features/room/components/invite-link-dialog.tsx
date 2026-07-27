"use client"

import { useState } from "react"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogClose,
} from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"
import { useCreateInviteLink } from "@/features/room/hooks/use-rooms"
import { Loader2, Link2, Copy, Check } from "lucide-react"

interface InviteLinkDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  roomId: string
}

export function InviteLinkDialog({ open, onOpenChange, roomId }: InviteLinkDialogProps) {
  const createInviteLink = useCreateInviteLink(roomId)
  const [inviteUrl, setInviteUrl] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)

  const handleGenerate = async () => {
    setCopied(false)
    try {
      const res = await createInviteLink.mutateAsync({ single_use: false, ttl_hours: 24 })
      if (res.data?.token) {
        const url = `${window.location.origin}/invite/${res.data.token}`
        setInviteUrl(url)
      }
    } catch {
      // toast handled by useCreateInviteLink
    }
  }

  const handleCopy = async () => {
    if (!inviteUrl) return
    await navigator.clipboard.writeText(inviteUrl)
    setCopied(true)
  }

  const handleClose = () => {
    setInviteUrl(null)
    setCopied(false)
    createInviteLink.reset()
    onOpenChange(false)
  }

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Invite Link</DialogTitle>
          <DialogDescription>
            Generate a link to invite others to this room
          </DialogDescription>
        </DialogHeader>
        <DialogClose onClick={handleClose} />

        <div className="space-y-4">
          {!inviteUrl ? (
            <Button
              onClick={handleGenerate}
              disabled={createInviteLink.isPending}
              className="w-full"
            >
              {createInviteLink.isPending ? (
                <Loader2 className="h-4 w-4 animate-spin mr-2" />
              ) : (
                <Link2 className="h-4 w-4 mr-2" />
              )}
              Generate Invite Link
            </Button>
          ) : (
            <div className="space-y-3">
              <div className="flex items-center gap-2 rounded-lg border border-gray-700 bg-gray-800 px-3 py-2">
                <span className="text-sm text-gray-300 truncate flex-1 select-all">
                  {inviteUrl}
                </span>
                <Button
                  variant="ghost"
                  size="icon"
                  onClick={handleCopy}
                  className="shrink-0 h-8 w-8"
                >
                  {copied ? (
                    <Check className="h-4 w-4 text-green-400" />
                  ) : (
                    <Copy className="h-4 w-4" />
                  )}
                </Button>
              </div>
              <Button
                variant="outline"
                onClick={handleGenerate}
                disabled={createInviteLink.isPending}
                className="w-full"
              >
                {createInviteLink.isPending ? (
                  <Loader2 className="h-4 w-4 animate-spin mr-2" />
                ) : null}
                Generate New Link
              </Button>
            </div>
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}
