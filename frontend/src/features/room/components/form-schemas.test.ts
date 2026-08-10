import { describe, it, expect } from "vitest"
import { z } from "zod"

const roomSettingsSchema = z.object({
  name: z.string().min(1, "Room name is required").max(100),
  description: z.string().max(500).optional(),
  max_members: z
    .string()
    .optional()
    .refine((v) => !v || (Number(v) >= 2 && Number(v) <= 1000), {
      message: "Must be between 2 and 1000",
    }),
})

const createRoomSchema = z
  .object({
    name: z.string().min(1, "Room name is required").max(100),
    description: z.string().max(500).optional(),
    auto_pop: z.boolean(),
    pop_at: z.string().optional(),
  })
  .superRefine((data, ctx) => {
    if (!data.auto_pop) return
    if (!data.pop_at) {
      ctx.addIssue({
        code: "custom",
        message: "Auto-pop time is required",
        path: ["pop_at"],
      })
      return
    }
    if (new Date(data.pop_at) <= new Date()) {
      ctx.addIssue({
        code: "custom",
        message: "Pop time must be in the future",
        path: ["pop_at"],
      })
    }
  })

describe("roomSettingsSchema", () => {
  it("should accept valid data", () => {
    const result = roomSettingsSchema.safeParse({
      name: "My Room",
      description: "A room",
      max_members: "50",
    })
    expect(result.success).toBe(true)
  })

  it("should accept data with only name", () => {
    const result = roomSettingsSchema.safeParse({ name: "Room" })
    expect(result.success).toBe(true)
  })

  it("should reject empty name", () => {
    const result = roomSettingsSchema.safeParse({ name: "" })
    expect(result.success).toBe(false)
    if (!result.success) {
      expect(result.error.issues[0].message).toBe("Room name is required")
    }
  })

  it("should reject name over 100 chars", () => {
    const result = roomSettingsSchema.safeParse({
      name: "a".repeat(101),
    })
    expect(result.success).toBe(false)
  })

  it("should reject max_members < 2", () => {
    const result = roomSettingsSchema.safeParse({
      name: "Room",
      max_members: "1",
    })
    expect(result.success).toBe(false)
    if (!result.success) {
      expect(result.error.issues[0].message).toBe("Must be between 2 and 1000")
    }
  })

  it("should reject max_members > 1000", () => {
    const result = roomSettingsSchema.safeParse({
      name: "Room",
      max_members: "1001",
    })
    expect(result.success).toBe(false)
  })

  it("should accept empty max_members", () => {
    const result = roomSettingsSchema.safeParse({
      name: "Room",
      max_members: "",
    })
    expect(result.success).toBe(true)
  })
})

describe("createRoomSchema", () => {
  it("should accept room with name only when auto_pop is disabled", () => {
    const result = createRoomSchema.safeParse({ name: "New Room", auto_pop: false })
    expect(result.success).toBe(true)
  })

  it("should reject empty name", () => {
    const result = createRoomSchema.safeParse({ name: "" })
    expect(result.success).toBe(false)
  })

  it("should reject pop_at in the past when auto_pop is enabled", () => {
    const pastDate = new Date(Date.now() - 86400000).toISOString().slice(0, 16)
    const result = createRoomSchema.safeParse({
      name: "Room",
      auto_pop: true,
      pop_at: pastDate,
    })
    expect(result.success).toBe(false)
    if (!result.success) {
      expect(result.error.issues[0].message).toBe("Pop time must be in the future")
    }
  })

  it("should accept pop_at in the future when auto_pop is enabled", () => {
    const futureDate = new Date(Date.now() + 86400000).toISOString().slice(0, 16)
    const result = createRoomSchema.safeParse({
      name: "Room",
      auto_pop: true,
      pop_at: futureDate,
    })
    expect(result.success).toBe(true)
  })

  it("should reject missing pop_at when auto_pop is enabled", () => {
    const result = createRoomSchema.safeParse({
      name: "Room",
      auto_pop: true,
    })
    expect(result.success).toBe(false)
    if (!result.success) {
      expect(result.error.issues[0].message).toBe("Auto-pop time is required")
    }
  })

  it("should accept missing pop_at when auto_pop is disabled", () => {
    const result = createRoomSchema.safeParse({
      name: "Room",
      auto_pop: false,
    })
    expect(result.success).toBe(true)
  })

  it("should accept a past pop_at when auto_pop is disabled", () => {
    const pastDate = new Date(Date.now() - 86400000).toISOString().slice(0, 16)
    const result = createRoomSchema.safeParse({
      name: "Room",
      auto_pop: false,
      pop_at: pastDate,
    })
    expect(result.success).toBe(true)
  })
})
