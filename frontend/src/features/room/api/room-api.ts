import { apiGet, apiPost, apiPut, apiDelete } from "@/lib/api-client"
import { API_PATHS } from "@/lib/api-paths"
import type { Room, RoomMember, InviteLink } from "@/types"
import type { CreateRoomRequest, UpdateRoomRequest, CreateInviteLinkRequest, UpdateRoomMemberRequest } from "@/types"

export async function fetchRooms(userId: string) {
  return apiGet<Room[]>(API_PATHS.rooms.list(userId))
}

export async function fetchRoom(id: string) {
  return apiGet<Room>(API_PATHS.rooms.get(id))
}

export async function createRoom(data: CreateRoomRequest) {
  return apiPost<Room>(API_PATHS.rooms.create, data)
}

export async function updateRoom(id: string, data: UpdateRoomRequest) {
  return apiPut<Room>(API_PATHS.rooms.update(id), data)
}

export async function deleteRoom(id: string) {
  return apiDelete<void>(API_PATHS.rooms.delete(id))
}

export async function fetchRoomMembers(roomId: string) {
  return apiGet<RoomMember[]>(API_PATHS.rooms.members(roomId))
}

export async function fetchRoomMember(roomId: string, userId: string) {
  return apiGet<RoomMember>(API_PATHS.rooms.member(roomId, userId))
}

export async function addRoomMember(roomId: string, userId: string, role?: number) {
  return apiPost<void>(API_PATHS.rooms.members(roomId), { user_id: userId, role })
}

export async function removeRoomMember(roomId: string, userId: string) {
  return apiDelete<void>(API_PATHS.rooms.member(roomId, userId))
}

export async function updateRoomMember(roomId: string, userId: string, data: UpdateRoomMemberRequest) {
  return apiPut<void>(API_PATHS.rooms.member(roomId, userId), data)
}

export async function createInviteLink(roomId: string, data?: CreateInviteLinkRequest) {
  return apiPost<InviteLink>(API_PATHS.rooms.invite(roomId), data ?? {})
}

export async function joinRoomViaInvite(token: string) {
  return apiPost<{ room_id: string }>(API_PATHS.rooms.joinInvite(token))
}

export async function popRoom(roomId: string) {
  return apiPost<void>(API_PATHS.rooms.pop(roomId))
}
