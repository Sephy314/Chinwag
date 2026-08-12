import { apiGet, apiPost, apiPut, apiDelete } from "@/lib/api-client"
import { API_PATHS } from "@/lib/api-paths"
import type {
  AdminUser,
  AdminCreateUserInput,
  AdminUpdateUserInput,
  AdminSession,
  AuditEvent,
  AdminStats,
  AdminRoomStats,
  AdminMessageStats,
  Room,
  RoomMember,
  Message,
} from "@/types"

export type AdminQueryParams = Record<string, string | number | undefined>

// --- Users ---
export const fetchAdminUsers = (params?: AdminQueryParams) =>
  apiGet<AdminUser[]>(API_PATHS.admin.users, params)
export const fetchAdminUser = (id: string) => apiGet<AdminUser>(API_PATHS.admin.user(id))
export const createAdminUser = (input: AdminCreateUserInput) =>
  apiPost<AdminUser>(API_PATHS.admin.users, input)
export const updateAdminUser = (id: string, input: AdminUpdateUserInput) =>
  apiPut<AdminUser>(API_PATHS.admin.user(id), input)
export const updateAdminUserRole = (id: string, role: string) =>
  apiPut<AdminUser>(API_PATHS.admin.userRole(id), { role })
export const disableAdminUser = (id: string) => apiDelete<AdminUser>(API_PATHS.admin.user(id))
export const restoreAdminUser = (id: string) => apiPost<AdminUser>(API_PATHS.admin.userRestore(id))

// --- Sessions ---
export const fetchAdminSessions = (params?: AdminQueryParams) =>
  apiGet<AdminSession[]>(API_PATHS.admin.sessions, params)
export const fetchAdminSession = (id: string) => apiGet<AdminSession>(API_PATHS.admin.session(id))
export const revokeAdminSession = (id: string) => apiDelete(API_PATHS.admin.session(id))

// --- Audit ---
export const fetchAdminAudit = (params?: AdminQueryParams) =>
  apiGet<AuditEvent[]>(API_PATHS.admin.audit, params)

// --- Rooms ---
export const fetchAdminRooms = (params?: AdminQueryParams) =>
  apiGet<Room[]>(API_PATHS.admin.rooms, params)
export const fetchAdminRoom = (id: string) => apiGet<Room>(API_PATHS.admin.room(id))
export const deleteAdminRoom = (id: string) => apiDelete(API_PATHS.admin.room(id))
export const fetchAdminRoomMembers = (roomId: string) =>
  apiGet<RoomMember[]>(API_PATHS.admin.roomMembers(roomId))
export const addAdminRoomMember = (roomId: string, body: { user_id: string; role?: number }) =>
  apiPost(API_PATHS.admin.roomMembers(roomId), body)
export const updateAdminRoomMember = (
  roomId: string,
  userId: string,
  body: { role?: number },
) => apiPut(API_PATHS.admin.roomMember(roomId, userId), body)
export const removeAdminRoomMember = (roomId: string, userId: string) =>
  apiDelete(API_PATHS.admin.roomMember(roomId, userId))
export const fetchAdminUserRooms = (userId: string) =>
  apiGet<Room[]>(API_PATHS.admin.userRooms(userId))

// --- Messages ---
export const fetchAdminMessages = (params?: AdminQueryParams) =>
  apiGet<Message[]>(API_PATHS.admin.messages, params)
export const deleteAdminMessage = (id: string) => apiDelete(API_PATHS.admin.message(id))

// --- Stats ---
export const fetchAdminStatsUsers = () => apiGet<AdminStats>(API_PATHS.admin.statsUsers)
export const fetchAdminStatsSessions = () => apiGet<AdminStats>(API_PATHS.admin.statsSessions)
export const fetchAdminStatsRooms = () => apiGet<AdminRoomStats>(API_PATHS.admin.statsRooms)
export const fetchAdminStatsMessages = () => apiGet<AdminMessageStats>(API_PATHS.admin.statsMessages)
