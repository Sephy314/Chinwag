"use client"

import { useMemo } from "react"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import { useRouter } from "next/navigation"
import { toast } from "sonner"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogClose,
} from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import { useCreateRoom } from "@/features/room/hooks/use-rooms"
import { getErrorMessage } from "@/lib/api-client"
import { Loader2 } from "lucide-react"

const createRoomSchema = z.object({
  name: z.string().min(1, "Room name is required").max(100),
  description: z.string().max(500).optional(),
  pop_at: z.string().optional(),
}).refine(
  (data) => {
    if (!data.pop_at) return true
    return new Date(data.pop_at) > new Date()
  },
  { message: "Pop time must be in the future", path: ["pop_at"] }
)

type CreateRoomForm = z.infer<typeof createRoomSchema>

interface CreateRoomDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function CreateRoomDialog({ open, onOpenChange }: CreateRoomDialogProps) {
  const createRoom = useCreateRoom()
  const router = useRouter()

  const defaultPopAt = useMemo(() => {
    const d = new Date()
    d.setDate(d.getDate() + 1)
    return d.toISOString().slice(0, 16)
  }, [])

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors, isSubmitting },
  } = useForm<CreateRoomForm>({
    resolver: zodResolver(createRoomSchema),
  })

  const onSubmit = async (data: CreateRoomForm) => {
    try {
      const res = await createRoom.mutateAsync({
        name: data.name,
        description: data.description || undefined,
        pop_at: data.pop_at ? new Date(data.pop_at).toISOString() : undefined,
      })
      if (res.data?.id) {
        router.push(`/chat/${res.data.id}`)
      }
      reset()
      onOpenChange(false)
    } catch (err) {
      toast.error(`Failed to create room: ${getErrorMessage(err)}`)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Create Room</DialogTitle>
          <DialogDescription>Create a new chat room to start talking</DialogDescription>
        </DialogHeader>
        <DialogClose onClick={() => { reset(); onOpenChange(false) }} />

        <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="name">Room name</Label>
            <Input
              id="name"
              placeholder="e.g. gaming, book-club..."
              {...register("name")}
            />
            {errors.name && (
              <p className="text-xs text-red-400">{errors.name.message}</p>
            )}
          </div>

          <div className="space-y-2">
            <Label htmlFor="description">Description (optional)</Label>
            <Textarea
              id="description"
              placeholder="What's this room about?"
              className="min-h-[80px]"
              {...register("description")}
            />
            {errors.description && (
              <p className="text-xs text-red-400">{errors.description.message}</p>
            )}
          </div>

          <div className="space-y-2">
            <Label htmlFor="pop_at">Auto-pop time (optional)</Label>
            <Input
              id="pop_at"
              type="datetime-local"
              defaultValue={defaultPopAt}
              {...register("pop_at")}
            />
            <p className="text-xs text-gray-500">
              Room becomes read-only after this time
            </p>
          </div>

          <div className="flex justify-end gap-3 pt-2">
            <Button
              type="button"
              variant="outline"
              onClick={() => { reset(); onOpenChange(false) }}
            >
              Cancel
            </Button>
            <Button type="submit" disabled={isSubmitting}>
              {isSubmitting ? (
                <Loader2 className="h-4 w-4 animate-spin mr-2" />
              ) : null}
              Create Room
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  )
}
