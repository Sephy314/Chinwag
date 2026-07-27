"use client"

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { useAuth } from "@/features/auth/hooks/use-auth"
import {
  fetchRooms,
  fetchRoom,
  createRoom,
  updateRoom,
  deleteRoom,
  fetchRoomMember,
  fetchRoomMembers,
  createInviteLink,
  joinRoomViaInvite,
  popRoom,
  updateRoomMember,
  removeRoomMember,
} from "@/features/room/api/room-api"
import type { CreateRoomRequest, UpdateRoomRequest, CreateInviteLinkRequest, UpdateRoomMemberRequest } from "@/types"
import { Role } from "@/types"
import { getErrorMessage } from "@/lib/api-client"

export function useRooms() {
  const { user } = useAuth()

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ["rooms", user?.id],
    queryFn: () => fetchRooms(user!.id),
    enabled: !!user?.id,
  })

  return {
    rooms: data?.data ?? [],
    isLoading,
    error,
    refetch,
  }
}

export function useRoom(id: string) {
  return useQuery({
    queryKey: ["room", id],
    queryFn: () => fetchRoom(id),
    enabled: !!id,
  })
}

export function useCreateRoom() {
  const queryClient = useQueryClient()
  const { user } = useAuth()

  return useMutation({
    mutationFn: (data: CreateRoomRequest) => createRoom(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["rooms", user?.id] })
    },
    onError: (err) => {
      toast.error(getErrorMessage(err))
    },
  })
}

export function useUpdateRoom(id: string) {
  const queryClient = useQueryClient()
  const { user } = useAuth()

  return useMutation({
    mutationFn: (data: UpdateRoomRequest) => updateRoom(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["room", id] })
      queryClient.invalidateQueries({ queryKey: ["rooms", user?.id] })
    },
    onError: (err) => {
      toast.error(getErrorMessage(err))
    },
  })
}

export function useDeleteRoom(id: string) {
  const queryClient = useQueryClient()
  const { user } = useAuth()

  return useMutation({
    mutationFn: () => deleteRoom(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["rooms", user?.id] })
    },
    onError: (err) => {
      toast.error(getErrorMessage(err))
    },
  })
}

export function useCreateInviteLink(roomId: string) {
  return useMutation({
    mutationFn: (data?: CreateInviteLinkRequest) => createInviteLink(roomId, data),
    onError: (err) => {
      toast.error(getErrorMessage(err))
    },
  })
}

export function useIsAdmin(roomId: string) {
  const { user } = useAuth()
  const { data: roomData } = useRoom(roomId)

  const { data, isLoading } = useQuery({
    queryKey: ["roomMember", roomId, user?.id],
    queryFn: () => fetchRoomMember(roomId, user!.id),
    enabled: !!roomId && !!user?.id,
    retry: false,
    throwOnError: false,
  })

  const isOwner = roomData?.data?.owner_id === user?.id
  const isAdmin = isOwner || data?.data?.role === Role.ADMIN

  return {
    isAdmin,
    isLoading,
  }
}

export function useRoomMembers(roomId: string) {
  const { data, isLoading, error } = useQuery({
    queryKey: ["roomMembers", roomId],
    queryFn: () => fetchRoomMembers(roomId),
    enabled: !!roomId,
  })

  return {
    members: data?.data ?? [],
    isLoading,
    error,
  }
}

export function useJoinRoomViaInvite() {
  const queryClient = useQueryClient()
  const { user } = useAuth()

  return useMutation({
    mutationFn: (token: string) => joinRoomViaInvite(token),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["rooms", user?.id] })
    },
    onError: (err) => {
      toast.error(getErrorMessage(err))
    },
  })
}

export function usePopRoom(roomId: string) {
  const queryClient = useQueryClient()
  const { user } = useAuth()

  return useMutation({
    mutationFn: () => popRoom(roomId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["room", roomId] })
      queryClient.invalidateQueries({ queryKey: ["rooms", user?.id] })
    },
    onError: (err) => {
      toast.error(getErrorMessage(err))
    },
  })
}

export function useUpdateRoomMember(roomId: string) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ userId, data }: { userId: string; data: UpdateRoomMemberRequest }) =>
      updateRoomMember(roomId, userId, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["roomMembers", roomId] })
    },
    onError: (err) => {
      toast.error(getErrorMessage(err))
    },
  })
}

export function useRemoveRoomMember(roomId: string) {
  const queryClient = useQueryClient()
  const { user } = useAuth()

  return useMutation({
    mutationFn: (userId: string) => removeRoomMember(roomId, userId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["roomMembers", roomId] })
      queryClient.invalidateQueries({ queryKey: ["rooms", user?.id] })
    },
    onError: (err) => {
      toast.error(getErrorMessage(err))
    },
  })
}
