import { describe, it, expect } from "vitest"
import { cn } from "./utils"

describe("cn", () => {
  it("should merge class names", () => {
    const result = cn("text-red-500", "text-blue-500")
    expect(result).toBe("text-blue-500")
  })

  it("should handle conditional classes", () => {
    const result = cn("base", false && "hidden", true && "active")
    expect(result).toContain("base")
    expect(result).toContain("active")
    expect(result).not.toContain("hidden")
  })

  it("should handle empty inputs", () => {
    expect(cn()).toBe("")
    expect(cn("")).toBe("")
    expect(cn("a", "", "b")).toBe("a b")
  })

  it("should handle tailwind merge conflicts", () => {
    const result = cn("px-4 py-2", "px-8")
    expect(result).toBe("py-2 px-8")
  })
})
