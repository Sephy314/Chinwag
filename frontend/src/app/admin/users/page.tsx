"use client"

import { useState } from "react"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import { Loader2, Plus, Search } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Badge } from "@/components/ui/badge"
import {
  useAdminUsers,
  useCreateAdminUser,
  useUpdateAdminUserRole,
  useDisableAdminUser,
  useRestoreAdminUser,
} from "@/features/admin/hooks/use-admin"
import { PageHeader, Table, Th, Td, EmptyState } from "@/features/admin/components/ui"
import type { AdminUser } from "@/types"

const createSchema = z.object({
  name: z.string().min(1, "Name is required"),
  email: z.string().email("Invalid email"),
  password: z.string().min(6, "Password must be at least 6 characters"),
  role: z.enum(["USER", "MANAGER", "ADMIN"]),
})
type CreateForm = z.infer<typeof createSchema>

function RoleBadge({ role, deleted }: { role: string; deleted?: string }) {
  if (deleted) return <Badge variant="destructive">Disabled</Badge>
  if (role === "ADMIN") return <Badge className="bg-blue-600 text-white">ADMIN</Badge>
  if (role === "MANAGER") return <Badge className="bg-purple-600 text-white">MANAGER</Badge>
  return <Badge className="bg-gray-700 text-gray-200">USER</Badge>
}

export default function AdminUsersPage() {
  const [q, setQ] = useState("")
  const [search, setSearch] = useState("")
  const [role, setRole] = useState("")
  const [deleted, setDeleted] = useState("")

  const { data, isLoading } = useAdminUsers({ q: search, role, deleted, limit: 100 })
  const users: AdminUser[] = data?.data ?? []

  return (
    <div>
      <PageHeader title="Users" description="List, create, and manage user accounts">
        <CreateUserDialog />
      </PageHeader>

      <div className="mb-3 flex flex-wrap items-center gap-2">
        <form
          className="flex items-center gap-2"
          onSubmit={(e) => {
            e.preventDefault()
            setSearch(q)
          }}
        >
          <Input
            className="w-64"
            placeholder="Search by name or email…"
            value={q}
            onChange={(e) => setQ(e.target.value)}
          />
          <Button type="submit" variant="secondary" size="sm" className="gap-1.5">
            <Search className="h-3.5 w-3.5" />
            Search
          </Button>
        </form>
        <select
          className="h-9 rounded-md border border-gray-700 bg-gray-900 px-2 text-sm text-gray-200"
          value={role}
          onChange={(e) => setRole(e.target.value)}
        >
          <option value="">All roles</option>
          <option value="ADMIN">ADMIN</option>
          <option value="MANAGER">MANAGER</option>
          <option value="USER">USER</option>
        </select>
        <select
          className="h-9 rounded-md border border-gray-700 bg-gray-900 px-2 text-sm text-gray-200"
          value={deleted}
          onChange={(e) => setDeleted(e.target.value)}
        >
          <option value="">Active only</option>
          <option value="include">Include disabled</option>
          <option value="only">Disabled only</option>
        </select>
      </div>

      {isLoading ? (
        <div className="flex justify-center py-12">
          <Loader2 className="h-6 w-6 animate-spin text-gray-500" />
        </div>
      ) : users.length === 0 ? (
        <EmptyState message="No users found." />
      ) : (
        <Table>
          <thead>
            <tr className="border-b border-gray-800">
              <Th>Name</Th>
              <Th>Email</Th>
              <Th>Role</Th>
              <Th>Status</Th>
              <Th>Created</Th>
              <Th>Actions</Th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-800">
            {users.map((u) => (
              <UserRow key={u.id} user={u} />
            ))}
          </tbody>
        </Table>
      )}
    </div>
  )
}

function UserRow({ user }: { user: AdminUser }) {
  const updateRole = useUpdateAdminUserRole(user.id)
  const disable = useDisableAdminUser(user.id)
  const restore = useRestoreAdminUser(user.id)

  return (
    <tr className="hover:bg-gray-900/50">
      <Td className="font-medium text-gray-100">{user.name}</Td>
      <Td>{user.email}</Td>
      <Td>
        <RoleBadge role={user.role} deleted={user.deleted_at} />
      </Td>
      <Td>{user.provider ? <Badge className="bg-gray-700 text-gray-200">{user.provider}</Badge> : <span className="text-gray-500">local</span>}</Td>
      <Td className="text-gray-500">{new Date(user.created_at).toLocaleString()}</Td>
      <Td>
        <div className="flex items-center gap-1.5">
          <select
            className="h-7 rounded border border-gray-700 bg-gray-900 px-1 text-xs text-gray-200"
            value={user.role}
            disabled={!!user.deleted_at || updateRole.isPending}
            onChange={(e) => updateRole.mutate(e.target.value)}
          >
            <option value="USER">USER</option>
            <option value="MANAGER">MANAGER</option>
            <option value="ADMIN">ADMIN</option>
          </select>
          {user.deleted_at ? (
            <Button
              size="sm"
              variant="secondary"
              onClick={() => restore.mutate()}
              disabled={restore.isPending}
            >
              Restore
            </Button>
          ) : (
            <Button
              size="sm"
              variant="destructive"
              onClick={() => disable.mutate()}
              disabled={disable.isPending}
            >
              Disable
            </Button>
          )}
        </div>
      </Td>
    </tr>
  )
}

function CreateUserDialog() {
  const [open, setOpen] = useState(false)
  const create = useCreateAdminUser()
  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<CreateForm>({
    resolver: zodResolver(createSchema),
    defaultValues: { name: "", email: "", password: "", role: "USER" },
  })

  const onSubmit = async (data: CreateForm) => {
    await create.mutateAsync(data)
    setOpen(false)
    reset()
  }

  return (
    <>
      <Button size="sm" className="gap-1.5" onClick={() => setOpen(true)}>
        <Plus className="h-4 w-4" />
        Create user
      </Button>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Create user</DialogTitle>
          </DialogHeader>
        <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="name">Name</Label>
            <Input id="name" {...register("name")} />
            {errors.name && <p className="text-xs text-red-400">{errors.name.message}</p>}
          </div>
          <div className="space-y-2">
            <Label htmlFor="email">Email</Label>
            <Input id="email" type="email" {...register("email")} />
            {errors.email && <p className="text-xs text-red-400">{errors.email.message}</p>}
          </div>
          <div className="space-y-2">
            <Label htmlFor="password">Password</Label>
            <Input id="password" type="password" {...register("password")} />
            {errors.password && <p className="text-xs text-red-400">{errors.password.message}</p>}
          </div>
          <div className="space-y-2">
            <Label htmlFor="role">Role</Label>
            <select id="role" className="h-9 w-full rounded-md border border-gray-700 bg-gray-900 px-2 text-sm text-gray-200" {...register("role")}>
              <option value="USER">USER</option>
              <option value="MANAGER">MANAGER</option>
              <option value="ADMIN">ADMIN</option>
            </select>
          </div>
          <div className="flex justify-end gap-2 pt-2">
            <Button type="button" variant="ghost" onClick={() => setOpen(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={create.isPending}>
              {create.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              Create
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
    </>
  )
}
