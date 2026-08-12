"use client"

import { Loader2, ShieldCheck, ShieldX } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { useAdminSessions, useRevokeAdminSession } from "@/features/admin/hooks/use-admin"
import { PageHeader, Table, Th, Td, EmptyState } from "@/features/admin/components/ui"
import type { AdminSession } from "@/types"

export default function AdminSessionsPage() {
  const { data, isLoading } = useAdminSessions({ limit: 100 })
  const sessions: AdminSession[] = data?.data ?? []

  return (
    <div>
      <PageHeader
        title="Sessions"
        description="Active refresh-token lineages. Revoking a lineage signs out every device in it."
      />

      {isLoading ? (
        <div className="flex justify-center py-12">
          <Loader2 className="h-6 w-6 animate-spin text-gray-500" />
        </div>
      ) : sessions.length === 0 ? (
        <EmptyState message="No sessions found." />
      ) : (
        <Table>
          <thead>
            <tr className="border-b border-gray-800">
              <Th>Lineage</Th>
              <Th>User</Th>
              <Th>Created</Th>
              <Th>Tokens</Th>
              <Th>State</Th>
              <Th>Actions</Th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-800">
            {sessions.map((s) => (
              <SessionRow key={s.lineage_id} session={s} />
            ))}
          </tbody>
        </Table>
      )}
    </div>
  )
}

function SessionRow({ session }: { session: AdminSession }) {
  const revoke = useRevokeAdminSession(session.lineage_id)
  const active = !session.revoked

  return (
    <tr className="hover:bg-gray-900/50">
      <Td className="font-mono text-xs text-gray-300">{session.lineage_id.slice(0, 8)}…</Td>
      <Td className="font-mono text-xs">{session.user_id.slice(0, 8)}…</Td>
      <Td className="text-gray-400">{new Date(session.created_at).toLocaleString()}</Td>
      <Td>{session.tokens}</Td>
      <Td>
        {active ? (
          <Badge className="bg-emerald-600 text-white">
            <ShieldCheck className="mr-1 h-3 w-3" /> Active
          </Badge>
        ) : (
          <Badge variant="destructive">
            <ShieldX className="mr-1 h-3 w-3" /> Revoked
          </Badge>
        )}
      </Td>
      <Td>
        {active && (
          <Button
            size="sm"
            variant="destructive"
            onClick={() => revoke.mutate()}
            disabled={revoke.isPending}
          >
            Revoke
          </Button>
        )}
      </Td>
    </tr>
  )
}
