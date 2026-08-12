"use client"

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { getErrorMessage } from "@/lib/api-client"
import {
  fetchAdminUsers,
  createAdminUser,
  updateAdminUserRole,
  disableAdminUser,
  restoreAdminUser,
  fetchAdminSessions,
  revokeAdminSession,
  fetchAdminAudit,
  fetchAdminRooms,
  deleteAdminRoom,
  fetchAdminRoomMembers,
  addAdminRoomMember,
  removeAdminRoomMember,
  fetchAdminMessages,
  deleteAdminMessage,
  fetchAdminStatsUsers,
  fetchAdminStatsSessions,
  fetchAdminStatsRooms,
  fetchAdminStatsMessages,
  type AdminQueryParams,
} from "../api/admin-api"
import type { AdminCreateUserInput } from "@/types"

// --- Users ---
export function useAdminUsers(params?: AdminQueryParams) {
  return useQuery({
    queryKey: ["admin", "users", params ?? {}],
    queryFn: () => fetchAdminUsers(params),
  })
}

export function useCreateAdminUser() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: AdminCreateUserInput) => createAdminUser(input),
    onSuccess: () => {
      toast.success("User created")
      qc.invalidateQueries({ queryKey: ["admin", "users"] })
    },
    onError: (err) => toast.error(getErrorMessage(err)),
  })
}

export function useUpdateAdminUserRole(id: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (role: string) => updateAdminUserRole(id, role),
    onSuccess: () => {
      toast.success("Role updated")
      qc.invalidateQueries({ queryKey: ["admin", "users"] })
    },
    onError: (err) => toast.error(getErrorMessage(err)),
  })
}

export function useDisableAdminUser(id: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => disableAdminUser(id),
    onSuccess: () => {
      toast.success("User disabled")
      qc.invalidateQueries({ queryKey: ["admin", "users"] })
    },
    onError: (err) => toast.error(getErrorMessage(err)),
  })
}

export function useRestoreAdminUser(id: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => restoreAdminUser(id),
    onSuccess: () => {
      toast.success("User restored")
      qc.invalidateQueries({ queryKey: ["admin", "users"] })
    },
    onError: (err) => toast.error(getErrorMessage(err)),
  })
}

// --- Sessions ---
export function useAdminSessions(params?: AdminQueryParams) {
  return useQuery({
    queryKey: ["admin", "sessions", params ?? {}],
    queryFn: () => fetchAdminSessions(params),
  })
}

export function useRevokeAdminSession(id: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => revokeAdminSession(id),
    onSuccess: () => {
      toast.success("Session revoked")
      qc.invalidateQueries({ queryKey: ["admin", "sessions"] })
    },
    onError: (err) => toast.error(getErrorMessage(err)),
  })
}

// --- Audit ---
export function useAdminAudit(params?: AdminQueryParams) {
  return useQuery({
    queryKey: ["admin", "audit", params ?? {}],
    queryFn: () => fetchAdminAudit(params),
  })
}

// --- Rooms ---
export function useAdminRooms(params?: AdminQueryParams) {
  return useQuery({
    queryKey: ["admin", "rooms", params ?? {}],
    queryFn: () => fetchAdminRooms(params),
  })
}

export function useDeleteAdminRoom(id: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => deleteAdminRoom(id),
    onSuccess: () => {
      toast.success("Room deleted")
      qc.invalidateQueries({ queryKey: ["admin", "rooms"] })
    },
    onError: (err) => toast.error(getErrorMessage(err)),
  })
}

export function useAdminRoomMembers(roomId: string) {
  return useQuery({
    queryKey: ["admin", "room-members", roomId],
    queryFn: () => fetchAdminRoomMembers(roomId),
    enabled: !!roomId,
  })
}

export function useAddAdminRoomMember(roomId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: { user_id: string; role?: number }) => addAdminRoomMember(roomId, body),
    onSuccess: () => {
      toast.success("Member added")
      qc.invalidateQueries({ queryKey: ["admin", "room-members", roomId] })
    },
    onError: (err) => toast.error(getErrorMessage(err)),
  })
}

export function useRemoveAdminRoomMember(roomId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (userId: string) => removeAdminRoomMember(roomId, userId),
    onSuccess: () => {
      toast.success("Member removed")
      qc.invalidateQueries({ queryKey: ["admin", "room-members", roomId] })
    },
    onError: (err) => toast.error(getErrorMessage(err)),
  })
}

// --- Messages ---
export function useAdminMessages(params?: AdminQueryParams) {
  return useQuery({
    queryKey: ["admin", "messages", params ?? {}],
    queryFn: () => fetchAdminMessages(params),
  })
}

export function useDeleteAdminMessage(id: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => deleteAdminMessage(id),
    onSuccess: () => {
      toast.success("Message deleted")
      qc.invalidateQueries({ queryKey: ["admin", "messages"] })
    },
    onError: (err) => toast.error(getErrorMessage(err)),
  })
}

// --- Stats ---
export function useAdminStats() {
  const users = useQuery({ queryKey: ["admin", "stats", "users"], queryFn: fetchAdminStatsUsers })
  const sessions = useQuery({ queryKey: ["admin", "stats", "sessions"], queryFn: fetchAdminStatsSessions })
  const rooms = useQuery({ queryKey: ["admin", "stats", "rooms"], queryFn: fetchAdminStatsRooms })
  const messages = useQuery({ queryKey: ["admin", "stats", "messages"], queryFn: fetchAdminStatsMessages })
  return { users, sessions, rooms, messages }
}
