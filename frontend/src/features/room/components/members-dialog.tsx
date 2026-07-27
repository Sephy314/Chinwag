"use client"

import { Users, AlertTriangle } from "lucide-react"
import { useRoomMembers } from "@/features/room/hooks/use-rooms"
import { Role } from "@/types"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogClose,
} from "@/components/ui/dialog"
import { Badge } from "@/components/ui/badge"
import { Skeleton } from "@/components/ui/skeleton"

interface MembersDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  roomId: string
}

export function MembersDialog({ open, onOpenChange, roomId }: MembersDialogProps) {
  const { members, isLoading, error } = useRoomMembers(roomId)

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>Members</DialogTitle>
        </DialogHeader>
        <DialogClose onClick={() => onOpenChange(false)} />

        <div className="space-y-1 max-h-96 overflow-y-auto">
          {isLoading ? (
            <div className="space-y-3">
              {Array.from({ length: 4 }).map((_, i) => (
                <div key={i} className="flex items-center gap-3 py-2">
                  <Skeleton className="h-8 w-8 rounded-full" />
                  <Skeleton className="h-4 w-24" />
                </div>
              ))}
            </div>
          ) : error ? (
            <div className="flex flex-col items-center justify-center py-8 text-center">
              <AlertTriangle className="h-6 w-6 text-red-400 mb-2" />
              <p className="text-sm text-gray-400">Failed to load members</p>
              <p className="text-xs text-gray-600 mt-1">{error.message}</p>
            </div>
          ) : members.length === 0 ? (
            <p className="text-sm text-gray-500 py-4 text-center">No members found</p>
          ) : (
            members.map((member) => (
              <div
                key={member.user_id}
                className="flex items-center gap-3 py-2 px-2 rounded-lg hover:bg-gray-800/50"
              >
                <div className="h-8 w-8 rounded-full bg-gray-700 flex items-center justify-center shrink-0">
                  <Users className="h-4 w-4 text-gray-400" />
                </div>
                <div className="flex-1 min-w-0">
                  <p className="text-sm text-gray-100 truncate">
                    {member.user_name || "Unknown"}
                  </p>
                </div>
                {member.role === Role.ADMIN && (
                  <Badge variant="secondary" className="text-xs">
                    Admin
                  </Badge>
                )}
              </div>
            ))
          )}
        </div>

        {!isLoading && !error && members.length > 0 && (
          <p className="text-xs text-gray-500 text-center pt-2">
            {members.length} {members.length === 1 ? "member" : "members"}
          </p>
        )}
      </DialogContent>
    </Dialog>
  )
}
