"use client"

import { useState } from "react"
import { Loader2 } from "lucide-react"
import { Input } from "@/components/ui/input"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { useAdminAudit } from "@/features/admin/hooks/use-admin"
import { PageHeader, Table, Th, Td, EmptyState } from "@/features/admin/components/ui"
import type { AuditEvent } from "@/types"

export default function AdminAuditPage() {
  const [adminId, setAdminId] = useState("")
  const [action, setAction] = useState("")
  const [targetType, setTargetType] = useState("")

  const { data, isLoading, refetch, isFetching } = useAdminAudit({
    limit: 100,
    admin_id: adminId || undefined,
    action: action || undefined,
    target_type: targetType || undefined,
  })
  const events: AuditEvent[] = data?.data ?? []

  return (
    <div>
      <PageHeader title="Audit log" description="Admin actions recorded over the internal mTLS channel" />

      <div className="mb-3 flex flex-wrap items-center gap-2">
        <Input
          className="w-56"
          placeholder="Admin id…"
          value={adminId}
          onChange={(e) => setAdminId(e.target.value)}
        />
        <Input
          className="w-56"
          placeholder="Action (e.g. user.create)…"
          value={action}
          onChange={(e) => setAction(e.target.value)}
        />
        <Input
          className="w-40"
          placeholder="Target type…"
          value={targetType}
          onChange={(e) => setTargetType(e.target.value)}
        />
        <Button size="sm" variant="secondary" onClick={() => refetch()} disabled={isFetching}>
          Apply
        </Button>
      </div>

      {isLoading ? (
        <div className="flex justify-center py-12">
          <Loader2 className="h-6 w-6 animate-spin text-gray-500" />
        </div>
      ) : events.length === 0 ? (
        <EmptyState message="No audit events found." />
      ) : (
        <Table>
          <thead>
            <tr className="border-b border-gray-800">
              <Th>When</Th>
              <Th>Admin</Th>
              <Th>Action</Th>
              <Th>Target</Th>
              <Th>Target id</Th>
              <Th>Metadata</Th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-800">
            {events.map((e) => (
              <tr key={e.id} className="hover:bg-gray-900/50">
                <Td className="whitespace-nowrap text-gray-400">{new Date(e.created_at).toLocaleString()}</Td>
                <Td className="font-mono text-xs">{e.admin_id.slice(0, 8)}…</Td>
                <Td>
                  <Badge className="bg-gray-800 text-gray-200">{e.action}</Badge>
                </Td>
                <Td className="text-gray-400">{e.target_type}</Td>
                <Td className="font-mono text-xs text-gray-400">{e.target_id.slice(0, 12)}…</Td>
                <Td className="max-w-[220px] truncate font-mono text-xs text-gray-500">
                  {e.metadata ? JSON.stringify(e.metadata) : "—"}
                </Td>
              </tr>
            ))}
          </tbody>
        </Table>
      )}
    </div>
  )
}
