/**
 * DPoP (RFC 9449) client-side helpers.
 *
 * The DPoP proof-of-possession key pair is generated with WebCrypto using a
 * non-extractable private key so it cannot be exported from JavaScript at all.
 * The CryptoKey object (which is non-extractable) is persisted in IndexedDB;
 * only the public JWK is plain text.
 */

const DB_NAME = "chinwag"
const STORE_NAME = "dpop-keys"
const KEY_NAME = "dpop"

interface StoredKey {
  privateKey: CryptoKey
  publicJwk: JsonWebKey
}

let cachedKey: StoredKey | null = null

function base64url(data: ArrayBuffer | Uint8Array): string {
  const bytes = data instanceof Uint8Array ? data : new Uint8Array(data)
  let bin = ""
  for (const b of bytes) bin += String.fromCharCode(b)
  return btoa(bin).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "")
}

function encodeJSON(value: unknown): string {
  return base64url(new TextEncoder().encode(JSON.stringify(value)))
}

function openDB(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    const req = indexedDB.open(DB_NAME, 1)
    req.onupgradeneeded = () => {
      if (!req.result.objectStoreNames.contains(STORE_NAME)) {
        req.result.createObjectStore(STORE_NAME)
      }
    }
    req.onsuccess = () => resolve(req.result)
    req.onerror = () => reject(req.error)
  })
}

async function getStoredKey(): Promise<StoredKey | null> {
  const db = await openDB()
  try {
    return await new Promise((resolve, reject) => {
      const tx = db.transaction(STORE_NAME, "readonly")
      const req = tx.objectStore(STORE_NAME).get(KEY_NAME)
      req.onsuccess = () => resolve(req.result ?? null)
      req.onerror = () => reject(req.error)
    })
  } finally {
    db.close()
  }
}

async function setStoredKey(key: StoredKey): Promise<void> {
  const db = await openDB()
  try {
    return await new Promise((resolve, reject) => {
      const tx = db.transaction(STORE_NAME, "readwrite")
      tx.objectStore(STORE_NAME).put(key, KEY_NAME)
      tx.oncomplete = () => resolve()
      tx.onerror = () => reject(tx.error)
    })
  } finally {
    db.close()
  }
}

/**
 * Returns the persisted DPoP key pair, generating and storing it on first use.
 */
export async function getOrCreateDPoPKey(): Promise<StoredKey> {
  if (cachedKey) return cachedKey

  const existing = await getStoredKey()
  if (existing && existing.privateKey) {
    cachedKey = existing
    return existing
  }

  const pair = await globalThis.crypto.subtle.generateKey(
    { name: "ECDSA", namedCurve: "P-256" },
    false,
    ["sign", "verify"],
  )

  const publicJwk = await globalThis.crypto.subtle.exportKey("jwk", pair.publicKey)
  const stored: StoredKey = { privateKey: pair.privateKey, publicJwk }
  await setStoredKey(stored)
  cachedKey = stored
  return stored
}

/**
 * RFC 7638 SHA-256 thumbprint (jkt) of the DPoP public key.
 */
export async function computeJkt(publicJwk: JsonWebKey): Promise<string> {
  const canonical = JSON.stringify({
    crv: publicJwk.crv,
    kty: publicJwk.kty,
    x: publicJwk.x,
    y: publicJwk.y,
  })
  const digest = await globalThis.crypto.subtle.digest("SHA-256", new TextEncoder().encode(canonical))
  return base64url(digest)
}

function normalizeHtu(url: string): string {
  const base = typeof window !== "undefined" ? window.location.origin : "http://localhost:3000"
  try {
    const u = new URL(url, base)
    return `${u.origin}${u.pathname}`
  } catch {
    return url
  }
}

/**
 * Creates a DPoP proof JWT for the given HTTP request.
 */
export async function createDPoPProof(opts: {
  method: string
  url: string
  nonce?: string
}): Promise<{ proof: string; jkt: string }> {
  const key = await getOrCreateDPoPKey()
  const { publicJwk, privateKey } = key

  const header = {
    typ: "dpop+jwt",
    alg: "ES256",
    jwk: {
      kty: publicJwk.kty,
      crv: publicJwk.crv,
      x: publicJwk.x,
      y: publicJwk.y,
    },
  }

  const payload: Record<string, unknown> = {
    jti: globalThis.crypto.randomUUID(),
    htm: opts.method.toUpperCase(),
    htu: normalizeHtu(opts.url),
    iat: Math.floor(Date.now() / 1000),
  }
  if (opts.nonce) payload.nonce = opts.nonce

  const signingInput = `${encodeJSON(header)}.${encodeJSON(payload)}`
  const signature = await globalThis.crypto.subtle.sign(
    { name: "ECDSA", hash: "SHA-256" },
    privateKey,
    new TextEncoder().encode(signingInput),
  )

  return {
    proof: `${signingInput}.${base64url(signature)}`,
    jkt: await computeJkt(publicJwk),
  }
}

/**
 * Builds the Google OAuth authorize URL bound to the DPoP key via a jkt
 * query parameter. The server stores (state -> jkt) and binds the issued
 * tokens to the key at the callback.
 */
export async function getGoogleAuthorizeURL(base: string): Promise<string> {
  const key = await getOrCreateDPoPKey()
  const jkt = await computeJkt(key.publicJwk)
  const sep = base.includes("?") ? "&" : "?"
  return `${base}${sep}jkt=${encodeURIComponent(jkt)}`
}