import { describe, it, expect, vi, beforeEach } from "vitest"
import * as apiClient from "@/lib/api-client"
import {
  fetchRooms,
  fetchRoom,
  createRoom,
  updateRoom,
  deleteRoom,
  fetchRoomMembers,
  fetchRoomMember,
  addRoomMember,
  removeRoomMember,
  updateRoomMember,
  createInviteLink,
  joinRoomViaInvite,
  popRoom,
} from "./room-api"

vi.mock("@/lib/api-client", () => ({
  apiGet: vi.fn(),
  apiPost: vi.fn(),
  apiPut: vi.fn(),
  apiDelete: vi.fn(),
}))

const mock = vi.mocked(apiClient)

describe("room-api", () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe("fetchRooms", () => {
    it("should call apiGet with correct path", async () => {
      const response = { success: true, data: [], message: "ok", code: "OK" }
      mock.apiGet.mockResolvedValue(response)

      const result = await fetchRooms("user-1")

      expect(mock.apiGet).toHaveBeenCalledWith("/users/user-1/rooms")
      expect(result).toEqual(response)
    })
  })

  describe("fetchRoom", () => {
    it("should call apiGet with room id", async () => {
      const room = { id: "room-1", name: "Test Room" }
      mock.apiGet.mockResolvedValue({ success: true, data: room, message: "ok", code: "OK" })

      const result = await fetchRoom("room-1")

      expect(mock.apiGet).toHaveBeenCalledWith("/rooms/room-1")
      expect(result.data).toEqual(room)
    })
  })

  describe("createRoom", () => {
    it("should call apiPost with room data", async () => {
      const data = { name: "New Room", description: "A room" }
      const room = { id: "room-2", ...data }
      mock.apiPost.mockResolvedValue({ success: true, data: room, message: "ok", code: "OK" })

      const result = await createRoom(data)

      expect(mock.apiPost).toHaveBeenCalledWith("/rooms", data)
      expect(result.data).toEqual(room)
    })
  })

  describe("updateRoom", () => {
    it("should call apiPut with id and data", async () => {
      const data = { name: "Updated" }
      mock.apiPut.mockResolvedValue({ success: true, data: null, message: "ok", code: "OK" })

      await updateRoom("room-1", data)

      expect(mock.apiPut).toHaveBeenCalledWith("/rooms/room-1", data)
    })
  })

  describe("deleteRoom", () => {
    it("should call apiDelete with room id", async () => {
      mock.apiDelete.mockResolvedValue({ success: true, data: null, message: "ok", code: "OK" })

      await deleteRoom("room-1")

      expect(mock.apiDelete).toHaveBeenCalledWith("/rooms/room-1")
    })
  })

  describe("fetchRoomMembers", () => {
    it("should call apiGet with members path", async () => {
      const members = [
        { user_id: "u1", user_name: "Alice", role: 0 },
        { user_id: "u2", user_name: "Bob", role: 1 },
      ]
      mock.apiGet.mockResolvedValue({ success: true, data: members, message: "ok", code: "OK" })

      const result = await fetchRoomMembers("room-1")

      expect(mock.apiGet).toHaveBeenCalledWith("/rooms/room-1/members")
      expect(result.data).toEqual(members)
    })
  })

  describe("fetchRoomMember", () => {
    it("should call apiGet with member path", async () => {
      const member = { user_id: "u1", user_name: "Alice", role: 0 }
      mock.apiGet.mockResolvedValue({ success: true, data: member, message: "ok", code: "OK" })

      const result = await fetchRoomMember("room-1", "u1")

      expect(mock.apiGet).toHaveBeenCalledWith("/rooms/room-1/members/u1")
      expect(result.data).toEqual(member)
    })
  })

  describe("addRoomMember", () => {
    it("should call apiPost with member data", async () => {
      mock.apiPost.mockResolvedValue({ success: true, data: null, message: "ok", code: "OK" })

      await addRoomMember("room-1", "u1", 0)

      expect(mock.apiPost).toHaveBeenCalledWith("/rooms/room-1/members", {
        user_id: "u1",
        role: 0,
      })
    })

    it("should handle missing role", async () => {
      mock.apiPost.mockResolvedValue({ success: true, data: null, message: "ok", code: "OK" })

      await addRoomMember("room-1", "u1")

      expect(mock.apiPost).toHaveBeenCalledWith("/rooms/room-1/members", {
        user_id: "u1",
        role: undefined,
      })
    })
  })

  describe("removeRoomMember", () => {
    it("should call apiDelete with member path", async () => {
      mock.apiDelete.mockResolvedValue({ success: true, data: null, message: "ok", code: "OK" })

      await removeRoomMember("room-1", "u1")

      expect(mock.apiDelete).toHaveBeenCalledWith("/rooms/room-1/members/u1")
    })
  })

  describe("updateRoomMember", () => {
    it("should call apiPut with member path and role data", async () => {
      mock.apiPut.mockResolvedValue({ success: true, data: null, message: "ok", code: "OK" })

      await updateRoomMember("room-1", "u1", { role: 1 })

      expect(mock.apiPut).toHaveBeenCalledWith("/rooms/room-1/members/u1", { role: 1 })
    })
  })

  describe("createInviteLink", () => {
    it("should call apiPost with invite data", async () => {
      const data = { single_use: true, ttl_hours: 24 }
      const link = { token: "tok", room_id: "room-1", expires_at: "2026-01-01" }
      mock.apiPost.mockResolvedValue({ success: true, data: link, message: "ok", code: "OK" })

      const result = await createInviteLink("room-1", data)

      expect(mock.apiPost).toHaveBeenCalledWith("/rooms/room-1/invite", data)
      expect(result.data).toEqual(link)
    })

    it("should default to empty object when no data", async () => {
      mock.apiPost.mockResolvedValue({ success: true, data: null, message: "ok", code: "OK" })

      await createInviteLink("room-1")

      expect(mock.apiPost).toHaveBeenCalledWith("/rooms/room-1/invite", {})
    })
  })

  describe("joinRoomViaInvite", () => {
    it("should call apiPost with token", async () => {
      mock.apiPost.mockResolvedValue({
        success: true,
        data: { room_id: "room-1" },
        message: "ok",
        code: "OK",
      })

      const result = await joinRoomViaInvite("token-abc")

      expect(mock.apiPost).toHaveBeenCalledWith("/rooms/invite/token-abc/join")
      expect(result.data).toEqual({ room_id: "room-1" })
    })
  })

  describe("popRoom", () => {
    it("should call apiPost with pop path", async () => {
      mock.apiPost.mockResolvedValue({ success: true, data: null, message: "ok", code: "OK" })

      await popRoom("room-1")

      expect(mock.apiPost).toHaveBeenCalledWith("/rooms/room-1/pop")
    })
  })
})
