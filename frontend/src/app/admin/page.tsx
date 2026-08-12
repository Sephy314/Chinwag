"use client"

import { useAdminStats } from "@/features/admin/hooks/use-admin"
import { PageHeader, StatCard } from "@/features/admin/components/ui"
import { Card, CardContent } from "@/components/ui/card"

export default function AdminOverviewPage() {
  const { users, sessions, rooms, messages } = useAdminStats()

  const cards = [
    { label: "Users", value: users.data?.data?.count, loading: users.isLoading },
    { label: "Sessions", value: sessions.data?.data?.count, loading: sessions.isLoading },
    { label: "Rooms", value: rooms.data?.data?.total_rooms, loading: rooms.isLoading },
    { label: "Messages", value: messages.data?.data?.total_messages, loading: messages.isLoading },
  ]

  return (
    <div>
      <PageHeader title="Overview" description="Platform-wide admin statistics" />
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        {cards.map((c) => (
          <StatCard key={c.label} label={c.label} value={c.value} loading={c.loading} />
        ))}
      </div>

      <Card className="mt-6">
        <CardContent className="p-4 text-sm text-gray-400">
          <p className="mb-2 font-medium text-gray-200">Admin capabilities</p>
          <ul className="list-inside list-disc space-y-1">
            <li>Users — list/search, create, change roles, disable &amp; restore accounts</li>
            <li>Sessions — list refresh-token lineages and revoke sessions</li>
            <li>Rooms — list/search, inspect members, manage memberships, delete rooms</li>
            <li>Messages — search messages across rooms and delete (CQRS-propagated)</li>
            <li>Audit — review admin actions recorded over the internal mTLS channel</li>
          </ul>
        </CardContent>
      </Card>
    </div>
  )
}
