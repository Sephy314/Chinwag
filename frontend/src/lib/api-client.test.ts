import { describe, it, expect, beforeEach, vi } from "vitest"
import { ApiError, setAccessToken, getAccessToken } from "./api-client"

describe("ApiError", () => {
  it("should create an error with status, code, and message", () => {
    const error = new ApiError(404, "NOT_FOUND", "Resource not found")

    expect(error).toBeInstanceOf(Error)
    expect(error).toBeInstanceOf(ApiError)
    expect(error.name).toBe("ApiError")
    expect(error.status).toBe(404)
    expect(error.code).toBe("NOT_FOUND")
    expect(error.message).toBe("Resource not found")
  })

  it("should accept optional data", () => {
    const data = { details: "extra info" }
    const error = new ApiError(400, "BAD_REQUEST", "Invalid input", data)

    expect(error.data).toEqual(data)
  })

  it("should work without data", () => {
    const error = new ApiError(500, "INTERNAL", "Server error")

    expect(error.data).toBeUndefined()
  })
})

describe("setAccessToken / getAccessToken", () => {
  beforeEach(() => {
    setAccessToken(null)
  })

  it("should return null initially", () => {
    expect(getAccessToken()).toBeNull()
  })

  it("should store and retrieve a token", () => {
    setAccessToken("test-token-123")
    expect(getAccessToken()).toBe("test-token-123")
  })

  it("should clear token when set to null", () => {
    setAccessToken("token")
    setAccessToken(null)
    expect(getAccessToken()).toBeNull()
  })
})
