"use client"

import { useState } from "react"
import { Loader2, Search, Trash2, Users } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Badge } from "@/components/ui/badge"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {
  useAdminRooms,
  useDeleteAdminRoom,
  useAdminRoomMembers,
  useRemoveAdminRoomMember,
} from "@/features/admin/hooks/use-admin"
import { PageHeader, Table, Th, Td, EmptyState } from "@/features/admin/components/ui"
import type { Room, RoomMember } from "@/types"

export default function AdminRoomsPage() {
  const [q, setQ] = useState("")
  const [search, setSearch] = useState("")

  const { data, isLoading } = useAdminRooms({ q: search, limit: 100 })
  const rooms: Room[] = data?.data ?? []

  return (
    <div>
      <PageHeader title="Rooms" description="List, inspect, and manage rooms across the platform" />

      <div className="mb-3 flex items-center gap-2">
        <form
          className="flex items-center gap-2"
          onSubmit={(e) => {
            e.preventDefault()
            setSearch(q)
          }}
        >
          <Input className="w-64" placeholder="Search rooms…" value={q} onChange={(e) => setQ(e.target.value)} />
          <Button type="submit" variant="secondary" size="sm" className="gap-1.5">
            <Search className="h-3.5 w-3.5" />
            Search
          </Button>
        </form>
      </div>

      {isLoading ? (
        <div className="flex justify-center py-12">
          <Loader2 className="h-6 w-6 animate-spin text-gray-500" />
        </div>
      ) : rooms.length === 0 ? (
        <EmptyState message="No rooms found." />
      ) : (
        <Table>
          <thead>
            <tr className="border-b border-gray-800">
              <Th>Name</Th>
              <Th>Owner</Th>
              <Th>Max</Th>
              <Th>Created</Th>
              <Th>State</Th>
              <Th>Actions</Th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-800">
            {rooms.map((room) => (
              <RoomRow key={room.id} room={room} />
            ))}
          </tbody>
        </Table>
      )}
    </div>
  )
}

function RoomRow({ room }: { room: Room }) {
  const del = useDeleteAdminRoom(room.id)
  return (
    <tr className="hover:bg-gray-900/50">
      <Td className="font-medium text-gray-100">{room.name}</Td>
      <Td className="break-all font-mono text-xs">{room.owner_id}</Td>
      <Td>{room.max_members}</Td>
      <Td className="text-gray-400">{new Date(room.created_at).toLocaleString()}</Td>
      <Td>
        {room.deleted_at ? (
          <Badge variant="destructive">Deleted</Badge>
        ) : room.popped_at ? (
          <Badge className="bg-amber-600 text-white">Popped</Badge>
        ) : (
          <Badge className="bg-emerald-600 text-white">Active</Badge>
        )}
      </Td>
      <Td>
        <div className="flex items-center gap-1.5">
          <MembersDialog room={room} />
          <Button
            size="sm"
            variant="destructive"
            onClick={() => del.mutate()}
            disabled={del.isPending || !!room.deleted_at}
          >
            <Trash2 className="mr-1 h-3.5 w-3.5" />
            Delete
          </Button>
        </div>
      </Td>
    </tr>
  )
}

function MembersDialog({ room }: { room: Room }) {
  const [open, setOpen] = useState(false)
  const { data, isLoading } = useAdminRoomMembers(room.id)
  const remove = useRemoveAdminRoomMember(room.id)
  const members: RoomMember[] = data?.data ?? []

  return (
    <>
      <Button size="sm" variant="secondary" onClick={() => setOpen(true)}>
        <Users className="mr-1 h-3.5 w-3.5" />
        Members
      </Button>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Members — {room.name}</DialogTitle>
          </DialogHeader>
        {isLoading ? (
          <div className="flex justify-center py-8">
            <Loader2 className="h-5 w-5 animate-spin text-gray-500" />
          </div>
        ) : members.length === 0 ? (
          <p className="py-6 text-center text-sm text-gray-500">No members.</p>
        ) : (
          <div className="divide-y divide-gray-800">
            {members.map((m) => (
              <div key={m.user_id} className="flex items-center justify-between py-2">
                <div>
                  <p className="text-sm font-medium text-gray-200">{m.user_name}</p>
                  <p className="break-all font-mono text-xs text-gray-500">{m.user_id}</p>
                </div>
                <div className="flex items-center gap-2">
                  <Badge className={m.role === 1 ? "bg-blue-600 text-white" : "bg-gray-700 text-gray-200"}>
                    {m.role === 1 ? "ADMIN" : "MEMBER"}
                  </Badge>
                  {!m.left_at && (
                    <Button
                      size="sm"
                      variant="destructive"
                      onClick={() => remove.mutate(m.user_id)}
                      disabled={remove.isPending}
                    >
                      Remove
                    </Button>
                  )}
                </div>
              </div>
            ))}
          </div>
        )}
      </DialogContent>
    </Dialog>
    </>
  )
}
