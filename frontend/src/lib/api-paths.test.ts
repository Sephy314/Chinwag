import { describe, it, expect } from "vitest"
import { API_PATHS } from "./api-paths"

describe("API_PATHS", () => {
  describe("auth", () => {
    it("should return correct auth paths", () => {
      expect(API_PATHS.auth.whoami).toBe("/auth/whoami")
      expect(API_PATHS.auth.login).toBe("/auth/login")
      expect(API_PATHS.auth.register).toBe("/auth/user")
      expect(API_PATHS.auth.google).toBe("/auth/google")
      expect(API_PATHS.auth.googleCallback).toBe("/auth/google/callback")
      expect(API_PATHS.auth.logout).toBe("/auth/logout")
    })

    it("should build user path with id", () => {
      expect(API_PATHS.auth.user("abc-123")).toBe("/auth/user/abc-123")
    })
  })

  describe("rooms", () => {
    it("should build list path with userId", () => {
      expect(API_PATHS.rooms.list("user-1")).toBe("/users/user-1/rooms")
    })

    it("should build get path with id", () => {
      expect(API_PATHS.rooms.get("room-1")).toBe("/rooms/room-1")
    })

    it("should return static create path", () => {
      expect(API_PATHS.rooms.create).toBe("/rooms")
    })

    it("should build update path", () => {
      expect(API_PATHS.rooms.update("room-1")).toBe("/rooms/room-1")
    })

    it("should build delete path", () => {
      expect(API_PATHS.rooms.delete("room-1")).toBe("/rooms/room-1")
    })

    it("should build members list path", () => {
      expect(API_PATHS.rooms.members("room-1")).toBe("/rooms/room-1/members")
    })

    it("should build member path", () => {
      expect(API_PATHS.rooms.member("room-1", "user-1")).toBe(
        "/rooms/room-1/members/user-1",
      )
    })

    it("should build invite path", () => {
      expect(API_PATHS.rooms.invite("room-1")).toBe("/rooms/room-1/invite")
    })

    it("should build joinInvite path", () => {
      expect(API_PATHS.rooms.joinInvite("token-abc")).toBe(
        "/rooms/invite/token-abc/join",
      )
    })

    it("should build pop path", () => {
      expect(API_PATHS.rooms.pop("room-1")).toBe("/rooms/room-1/pop")
    })
  })

  describe("chat", () => {
    it("should build messages path", () => {
      expect(API_PATHS.chat.messages("room-1")).toBe(
        "/chat/rooms/room-1/messages",
      )
    })

    it("should build message path", () => {
      expect(API_PATHS.chat.message("room-1", "msg-1")).toBe(
        "/chat/rooms/room-1/messages/msg-1",
      )
    })
  })
})
