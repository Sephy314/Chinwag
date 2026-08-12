"use client"

import type { ReactNode } from "react"
import { Loader2 } from "lucide-react"
import { Card, CardContent } from "@/components/ui/card"

export function PageHeader({
  title,
  description,
  children,
}: {
  title: string
  description?: string
  children?: ReactNode
}) {
  return (
    <div className="mb-4 flex flex-wrap items-end justify-between gap-3">
      <div>
        <h1 className="text-xl font-semibold text-gray-100">{title}</h1>
        {description && <p className="text-sm text-gray-500">{description}</p>}
      </div>
      {children && <div className="flex items-center gap-2">{children}</div>}
    </div>
  )
}

export function StatCard({
  label,
  value,
  loading,
}: {
  label: string
  value?: number
  loading?: boolean
}) {
  return (
    <Card>
      <CardContent className="flex flex-col gap-1 p-4">
        <span className="text-xs font-medium uppercase tracking-wider text-gray-500">
          {label}
        </span>
        {loading ? (
          <Loader2 className="h-5 w-5 animate-spin text-gray-500" />
        ) : (
          <span className="text-2xl font-semibold text-gray-100">
            {value?.toLocaleString() ?? "—"}
          </span>
        )}
      </CardContent>
    </Card>
  )
}

export function Th({ children }: { children: ReactNode }) {
  return (
    <th className="px-3 py-2 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
      {children}
    </th>
  )
}

export function Td({ children, className }: { children: ReactNode; className?: string }) {
  return <td className={`px-3 py-2 text-sm text-gray-300 ${className ?? ""}`}>{children}</td>
}

export function Table({ children }: { children: ReactNode }) {
  return (
    <div className="overflow-x-auto rounded-lg border border-gray-800">
      <table className="w-full min-w-max border-collapse">{children}</table>
    </div>
  )
}

export function EmptyState({ message }: { message: string }) {
  return (
    <div className="rounded-lg border border-gray-800 px-4 py-10 text-center text-sm text-gray-500">
      {message}
    </div>
  )
}
