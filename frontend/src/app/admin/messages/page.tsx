"use client"

import { useState } from "react"
import { Loader2, Search, Trash2 } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Badge } from "@/components/ui/badge"
import { useAdminMessages, useDeleteAdminMessage } from "@/features/admin/hooks/use-admin"
import { PageHeader, Table, Th, Td, EmptyState } from "@/features/admin/components/ui"
import type { Message } from "@/types"

export default function AdminMessagesPage() {
  const [q, setQ] = useState("")
  const [search, setSearch] = useState("")
  const [roomId, setRoomId] = useState("")
  const [authorId, setAuthorId] = useState("")

  const { data, isLoading, refetch, isFetching } = useAdminMessages({
    q: search || undefined,
    room_id: roomId || undefined,
    author_id: authorId || undefined,
    limit: 100,
  })
  const messages: Message[] = data?.data ?? []

  return (
    <div>
      <PageHeader title="Messages" description="Search messages across all rooms and delete them (CQRS-propagated)" />

      <div className="mb-3 flex flex-wrap items-center gap-2">
        <form
          className="flex items-center gap-2"
          onSubmit={(e) => {
            e.preventDefault()
            setSearch(q)
          }}
        >
          <Input className="w-64" placeholder="Search content…" value={q} onChange={(e) => setQ(e.target.value)} />
          <Button type="submit" variant="secondary" size="sm" className="gap-1.5">
            <Search className="h-3.5 w-3.5" />
            Search
          </Button>
        </form>
        <Input className="w-56" placeholder="Room id (optional)…" value={roomId} onChange={(e) => setRoomId(e.target.value)} />
        <Input className="w-56" placeholder="Author id (optional)…" value={authorId} onChange={(e) => setAuthorId(e.target.value)} />
        <Button size="sm" variant="secondary" onClick={() => refetch()} disabled={isFetching}>
          Apply
        </Button>
      </div>

      {isLoading ? (
        <div className="flex justify-center py-12">
          <Loader2 className="h-6 w-6 animate-spin text-gray-500" />
        </div>
      ) : messages.length === 0 ? (
        <EmptyState message="No messages found." />
      ) : (
        <Table>
          <thead>
            <tr className="border-b border-gray-800">
              <Th>Author</Th>
              <Th>Content</Th>
              <Th>Room</Th>
              <Th>Type</Th>
              <Th>Sent</Th>
              <Th>Actions</Th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-800">
            {messages.map((m) => (
              <MessageRow key={m.id} message={m} />
            ))}
          </tbody>
        </Table>
      )}
    </div>
  )
}

function MessageRow({ message }: { message: Message }) {
  const del = useDeleteAdminMessage(message.id)
  return (
    <tr className="hover:bg-gray-900/50">
      <Td className="font-medium text-gray-100">{message.author_name}</Td>
      <Td className="max-w-[320px] truncate text-gray-300">{message.content}</Td>
      <Td className="break-all font-mono text-xs text-gray-400">{message.room_id}</Td>
      <Td>
        <Badge className="bg-gray-800 text-gray-200">{message.message_type}</Badge>
      </Td>
      <Td className="whitespace-nowrap text-gray-400">{new Date(message.created_at).toLocaleString()}</Td>
      <Td>
        <Button size="sm" variant="destructive" onClick={() => del.mutate()} disabled={del.isPending}>
          <Trash2 className="mr-1 h-3.5 w-3.5" />
          Delete
        </Button>
      </Td>
    </tr>
  )
}
