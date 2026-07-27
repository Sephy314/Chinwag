"use client"

import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import { useState } from "react"
import { useParams, useRouter } from "next/navigation"
import {
  Settings,
  Loader2,
  Flame,
  CalendarClock,
  CheckCircle2,
  Trash2,
  ArrowLeft,
  Users,
  Shield,
  ShieldOff,
  UserMinus,
  MoreVertical,
} from "lucide-react"
import {
  useRoom,
  useUpdateRoom,
  useDeleteRoom,
  useIsAdmin,
  useRoomMembers,
  useUpdateRoomMember,
  useRemoveRoomMember,
} from "@/features/room/hooks/use-rooms"
import { useAuth } from "@/features/auth/hooks/use-auth"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogClose,
} from "@/components/ui/dialog"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Role } from "@/types"
import { getErrorMessage } from "@/lib/api-client"

const roomSettingsSchema = z.object({
  name: z.string().min(1, "Room name is required").max(100),
  description: z.string().max(500).optional(),
  max_members: z
    .string()
    .optional()
    .refine((v) => !v || (Number(v) >= 2 && Number(v) <= 1000), {
      message: "Must be between 2 and 1000",
    }),
})

type RoomSettingsForm = z.infer<typeof roomSettingsSchema>

export default function RoomSettingsPage() {
  const params = useParams<{ roomId: string }>()
  const router = useRouter()
  const roomId = params.roomId
  const { user } = useAuth()

  const { data: roomData, isLoading: roomLoading } = useRoom(roomId)
  const { isAdmin, isLoading: adminLoading } = useIsAdmin(roomId)
  const updateRoom = useUpdateRoom(roomId)
  const deleteRoom = useDeleteRoom(roomId)
  const { members, isLoading: membersLoading } = useRoomMembers(roomId)
  const updateMember = useUpdateRoomMember(roomId)
  const removeMember = useRemoveRoomMember(roomId)

  const [error, setError] = useState("")
  const [success, setSuccess] = useState("")
  const [deleteOpen, setDeleteOpen] = useState(false)
  const [kickTarget, setKickTarget] = useState<{ userId: string; name: string } | null>(null)

  const room = roomData?.data
  const isPopped = !!room?.popped_at
  const ownerId = room?.owner_id

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<RoomSettingsForm>({
    resolver: zodResolver(roomSettingsSchema),
    values: {
      name: room?.name ?? "",
      description: room?.description ?? "",
      max_members: room?.max_members ? String(room.max_members) : "",
    },
  })

  const onSubmit = async (data: RoomSettingsForm) => {
    setError("")
    setSuccess("")
    try {
      await updateRoom.mutateAsync({
        name: data.name,
        description: data.description || undefined,
        max_members: data.max_members ? Number(data.max_members) : undefined,
      })
      setSuccess("Room settings updated")
    } catch (err) {
      setError(getErrorMessage(err))
    }
  }

  const handleDelete = async () => {
    try {
      await deleteRoom.mutateAsync()
      router.push("/home")
    } catch (err) {
      setError(getErrorMessage(err))
      setDeleteOpen(false)
    }
  }

  const handleRoleToggle = async (userId: string, currentRole: Role) => {
    const newRole = currentRole === Role.ADMIN ? Role.MEMBER : Role.ADMIN
    try {
      await updateMember.mutateAsync({ userId, data: { role: newRole } })
    } catch (err) {
      setError(getErrorMessage(err))
    }
  }

  const handleKick = async () => {
    if (!kickTarget) return
    try {
      await removeMember.mutateAsync(kickTarget.userId)
      setKickTarget(null)
    } catch (err) {
      setError(getErrorMessage(err))
      setKickTarget(null)
    }
  }

  if (roomLoading || adminLoading) {
    return (
      <div className="mx-auto max-w-lg px-4 py-8 space-y-4">
        <div className="h-8 w-48 bg-gray-800 rounded animate-pulse" />
        <div className="h-64 bg-gray-800 rounded animate-pulse" />
      </div>
    )
  }

  return (
    <div className="mx-auto max-w-lg px-4 py-8 space-y-6 overflow-y-auto h-full">
      <Button
        variant="ghost"
        size="sm"
        onClick={() => router.push(`/chat/${roomId}`)}
        className="gap-2 -ml-2"
      >
        <ArrowLeft className="h-4 w-4" />
        Back to chat
      </Button>

      <Card>
        <CardHeader>
          <div className="flex items-center gap-3">
            <Settings className="h-6 w-6 text-gray-400" />
            <div>
              <CardTitle>Room Settings</CardTitle>
              <CardDescription>
                {isAdmin ? "Edit your room configuration" : "Room configuration (read-only)"}
              </CardDescription>
            </div>
          </div>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="name">Room name</Label>
              <Input id="name" readOnly={!isAdmin} {...register("name")} />
              {errors.name && (
                <p className="text-xs text-red-400">{errors.name.message}</p>
              )}
            </div>

            <div className="space-y-2">
              <Label htmlFor="description">Description</Label>
              <Textarea
                id="description"
                placeholder="What's this room about?"
                className="min-h-[80px]"
                readOnly={!isAdmin}
                {...register("description")}
              />
              {errors.description && (
                <p className="text-xs text-red-400">{errors.description.message}</p>
              )}
            </div>

            <div className="space-y-2">
              <Label htmlFor="max_members">Max members</Label>
              <Input
                id="max_members"
                type="number"
                placeholder="No limit"
                readOnly={!isAdmin}
                {...register("max_members")}
              />
              {errors.max_members && (
                <p className="text-xs text-red-400">{errors.max_members.message}</p>
              )}
            </div>

            {error && (
              <div className="rounded-lg bg-red-600/10 border border-red-600/20 px-3 py-2">
                <p className="text-sm text-red-400">{error}</p>
              </div>
            )}

            {success && (
              <div className="rounded-lg bg-green-600/10 border border-green-600/20 px-3 py-2">
                <p className="text-sm text-green-400">{success}</p>
              </div>
            )}

            {isAdmin && (
              <Button type="submit" disabled={isSubmitting}>
                {isSubmitting ? (
                  <Loader2 className="h-4 w-4 animate-spin mr-2" />
                ) : null}
                Save Changes
              </Button>
            )}
          </form>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Pop Status</CardTitle>
          <CardDescription>Read-only room lifecycle information</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="flex items-center justify-between">
            <span className="text-sm text-gray-400">Status</span>
            {isPopped ? (
              <span className="flex items-center gap-1.5 text-sm text-orange-400">
                <Flame className="h-4 w-4" />
                Popped
              </span>
            ) : (
              <span className="flex items-center gap-1.5 text-sm text-green-400">
                <CheckCircle2 className="h-4 w-4" />
                Active
              </span>
            )}
          </div>

          {room?.pop_at && (
            <div className="flex items-center justify-between">
              <span className="text-sm text-gray-400">Scheduled pop</span>
              <span className="flex items-center gap-1.5 text-sm text-gray-300">
                <CalendarClock className="h-4 w-4 text-gray-500" />
                {new Date(room.pop_at).toLocaleString()}
              </span>
            </div>
          )}

          {room?.popped_at && (
            <div className="flex items-center justify-between">
              <span className="text-sm text-gray-400">Popped at</span>
              <span className="text-sm text-gray-300">
                {new Date(room.popped_at).toLocaleString()}
              </span>
            </div>
          )}

          <div className="flex items-center justify-between">
            <span className="text-sm text-gray-400">Created</span>
            <span className="text-sm text-gray-300">
              {new Date(room?.created_at ?? "").toLocaleString()}
            </span>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <Users className="h-5 w-5 text-gray-400" />
            <div>
              <CardTitle className="text-base">Members</CardTitle>
              <CardDescription>
                {isAdmin ? "Manage room members" : `${members.length} member${members.length !== 1 ? "s" : ""}`}
              </CardDescription>
            </div>
          </div>
        </CardHeader>
        <CardContent>
          {membersLoading ? (
            <div className="space-y-3">
              {Array.from({ length: 3 }).map((_, i) => (
                <div key={i} className="h-10 bg-gray-800 rounded animate-pulse" />
              ))}
            </div>
          ) : members.length === 0 ? (
            <p className="text-sm text-gray-500 text-center py-4">No members</p>
          ) : (
            <div className="space-y-1">
              {members.map((member) => {
                const isSelf = member.user_id === user?.id
                const isOwner = member.user_id === ownerId
                const canManage = isAdmin && !isSelf && !isOwner

                return (
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
                        {isSelf && (
                          <span className="text-xs text-gray-500 ml-1">(you)</span>
                        )}
                      </p>
                    </div>
                    <div className="flex items-center gap-2">
                      {member.role === Role.ADMIN ? (
                        <Badge variant="secondary" className="text-xs gap-1">
                          <Shield className="h-3 w-3" />
                          Admin
                        </Badge>
                      ) : (
                        <Badge variant="outline" className="text-xs">
                          Member
                        </Badge>
                      )}
                      {canManage && (
                        <DropdownMenu>
                          <DropdownMenuTrigger className="inline-flex items-center justify-center h-7 w-7 rounded-md text-gray-400 hover:bg-gray-800 hover:text-gray-100 outline-none">
                            <MoreVertical className="h-3.5 w-3.5" />
                          </DropdownMenuTrigger>
                          <DropdownMenuContent align="end">
                            <DropdownMenuItem
                              onClick={() =>
                                handleRoleToggle(member.user_id, member.role)
                              }
                              className="gap-2"
                            >
                              {member.role === Role.ADMIN ? (
                                <>
                                  <ShieldOff className="h-3.5 w-3.5" />
                                  Demote to Member
                                </>
                              ) : (
                                <>
                                  <Shield className="h-3.5 w-3.5" />
                                  Promote to Admin
                                </>
                              )}
                            </DropdownMenuItem>
                            <DropdownMenuItem
                              onClick={() =>
                                setKickTarget({
                                  userId: member.user_id,
                                  name: member.user_name || "Unknown",
                                })
                              }
                              className="gap-2 text-red-400 focus:text-red-400"
                            >
                              <UserMinus className="h-3.5 w-3.5" />
                              Remove from room
                            </DropdownMenuItem>
                          </DropdownMenuContent>
                        </DropdownMenu>
                      )}
                    </div>
                  </div>
                )
              })}
            </div>
          )}
        </CardContent>
      </Card>

      {isAdmin && (
        <Card className="border-red-600/20">
          <CardHeader>
            <CardTitle className="text-base text-red-400">Danger Zone</CardTitle>
            <CardDescription>Irreversible actions</CardDescription>
          </CardHeader>
          <CardContent>
            <Button
              variant="destructive"
              onClick={() => setDeleteOpen(true)}
              className="gap-2"
            >
              <Trash2 className="h-4 w-4" />
              Delete Room
            </Button>
          </CardContent>
        </Card>
      )}

      <Dialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete Room</DialogTitle>
            <DialogDescription>
              Are you sure you want to delete &ldquo;{room?.name}&rdquo;? This
              action cannot be undone.
            </DialogDescription>
          </DialogHeader>
          <DialogClose onClick={() => setDeleteOpen(false)} />
          <div className="flex justify-end gap-3 pt-2">
            <Button variant="outline" onClick={() => setDeleteOpen(false)}>
              Cancel
            </Button>
            <Button
              variant="destructive"
              onClick={handleDelete}
              disabled={deleteRoom.isPending}
            >
              {deleteRoom.isPending ? (
                <Loader2 className="h-4 w-4 animate-spin mr-2" />
              ) : null}
              Delete
            </Button>
          </div>
        </DialogContent>
      </Dialog>

      <Dialog open={!!kickTarget} onOpenChange={() => setKickTarget(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Remove Member</DialogTitle>
            <DialogDescription>
              Are you sure you want to remove &ldquo;{kickTarget?.name}&rdquo;
              from this room?
            </DialogDescription>
          </DialogHeader>
          <DialogClose onClick={() => setKickTarget(null)} />
          <div className="flex justify-end gap-3 pt-2">
            <Button variant="outline" onClick={() => setKickTarget(null)}>
              Cancel
            </Button>
            <Button
              variant="destructive"
              onClick={handleKick}
              disabled={removeMember.isPending}
            >
              {removeMember.isPending ? (
                <Loader2 className="h-4 w-4 animate-spin mr-2" />
              ) : null}
              Remove
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  )
}
