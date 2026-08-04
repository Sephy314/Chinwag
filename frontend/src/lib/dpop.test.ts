import { describe, it, expect, beforeEach } from "vitest"
import {
  computeJkt,
  createDPoPProof,
  getOrCreateDPoPKey,
  getGoogleAuthorizeURL,
} from "./dpop"

function decodePart(part: string): Record<string, unknown> {
  const b64 = part.replace(/-/g, "+").replace(/_/g, "/")
  const padded = b64 + "=".repeat((4 - (b64.length % 4)) % 4)
  return JSON.parse(atob(padded))
}

describe("dpop key management", () => {
  beforeEach(async () => {
    const req = indexedDB.deleteDatabase("chinwag")
    await new Promise((resolve) => {
      req.onsuccess = resolve
      req.onerror = resolve
    })
  })

  it("generates a non-extractable private key and persists it", async () => {
    const key = await getOrCreateDPoPKey()
    expect(key.privateKey.extractable).toBe(false)
    expect(key.privateKey.type).toBe("private")
    expect(key.publicJwk.kty).toBe("EC")
    expect(key.publicJwk.crv).toBe("P-256")
    expect(key.publicJwk.x).toBeTruthy()
    expect(key.publicJwk.y).toBeTruthy()
    expect(key.publicJwk.d).toBeUndefined()

    const again = await getOrCreateDPoPKey()
    expect(again.privateKey.extractable).toBe(false)
  })

  it("computes a stable RFC 7638 thumbprint", async () => {
    const key = await getOrCreateDPoPKey()
    const jkt1 = await computeJkt(key.publicJwk)
    const jkt2 = await computeJkt(key.publicJwk)
    expect(jkt1).toBe(jkt2)
    expect(jkt1).toMatch(/^[A-Za-z0-9_-]{43}$/)
  })
})

describe("createDPoPProof", () => {
  it("creates a compact JWS with dpop+jwt header and required claims", async () => {
    const { proof, jkt } = await createDPoPProof({
      method: "POST",
      url: "http://localhost:3000/auth/refresh",
      nonce: "nonce-123",
    })

    const parts = proof.split(".")
    expect(parts).toHaveLength(3)

    const header = decodePart(parts[0])
    expect(header.typ).toBe("dpop+jwt")
    expect(header.alg).toBe("ES256")
    expect((header.jwk as Record<string, unknown>).kty).toBe("EC")

    const payload = decodePart(parts[1])
    expect(payload.jti).toBeTruthy()
    expect(payload.htm).toBe("POST")
    expect(payload.htu).toBe("http://localhost:3000/auth/refresh")
    expect(payload.nonce).toBe("nonce-123")
    expect(typeof payload.iat).toBe("number")

    const key = await getOrCreateDPoPKey()
    expect(jkt).toBe(await computeJkt(key.publicJwk))
  })

  it("omits nonce when not provided", async () => {
    const { proof } = await createDPoPProof({ method: "POST", url: "http://localhost:3000/auth/refresh" })
    const payload = decodePart(proof.split(".")[1])
    expect(payload.nonce).toBeUndefined()
  })
})

describe("getGoogleAuthorizeURL", () => {
  it("appends the jkt thumbprint", async () => {
    const url = await getGoogleAuthorizeURL("http://localhost:8000/auth/google")
    expect(url.startsWith("http://localhost:8000/auth/google?jkt=")).toBe(true)
    const key = await getOrCreateDPoPKey()
    const jkt = await computeJkt(key.publicJwk)
    expect(url).toBe(`http://localhost:8000/auth/google?jkt=${encodeURIComponent(jkt)}`)
  })
})
