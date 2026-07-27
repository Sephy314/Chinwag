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

const TTL_OPTIONS = [
  { value: "1", label: "1 hour" },
  { value: "6", label: "6 hours" },
  { value: "12", label: "12 hours" },
  { value: "24", label: "24 hours" },
  { value: "72", label: "3 days" },
  { value: "168", label: "7 days" },
]

export function InviteLinkDialog({ open, onOpenChange, roomId }: InviteLinkDialogProps) {
  const createInviteLink = useCreateInviteLink(roomId)
  const [inviteUrl, setInviteUrl] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)
  const [ttlHours, setTtlHours] = useState("24")
  const [singleUse, setSingleUse] = useState(false)

  const handleGenerate = async () => {
    setCopied(false)
    try {
      const res = await createInviteLink.mutateAsync({
        single_use: singleUse,
        ttl_hours: parseInt(ttlHours, 10),
      })
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
            <div className="space-y-4">
              <div className="space-y-2">
                <label className="text-sm font-medium text-gray-300">
                  Expiration
                </label>
                <select
                  value={ttlHours}
                  onChange={(e) => setTtlHours(e.target.value)}
                  className="w-full rounded-md border border-gray-700 bg-gray-800 px-3 py-2 text-sm text-gray-300 focus:outline-none focus:ring-2 focus:ring-purple-500"
                >
                  {TTL_OPTIONS.map((option) => (
                    <option key={option.value} value={option.value}>
                      {option.label}
                    </option>
                  ))}
                </select>
              </div>

              <div className="space-y-2">
                <label className="text-sm font-medium text-gray-300">
                  Usage Limit
                </label>
                <div className="flex items-center gap-3">
                  <button
                    type="button"
                    role="switch"
                    aria-checked={singleUse}
                    onClick={() => setSingleUse(!singleUse)}
                    className={`relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-purple-500 focus:ring-offset-2 focus:ring-offset-gray-900 ${
                      singleUse ? "bg-purple-600" : "bg-gray-600"
                    }`}
                  >
                    <span
                      className={`pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out ${
                        singleUse ? "translate-x-5" : "translate-x-0"
                      }`}
                    />
                  </button>
                  <span className="text-sm text-gray-300">
                    {singleUse ? "Single use (one person)" : "Multiple uses"}
                  </span>
                </div>
              </div>

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
            </div>
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
