"use client"

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from "react"
import { useRouter } from "next/navigation"
import { apiPost, apiGet, apiRequest, setAccessToken, ApiError } from "@/lib/api-client"
import { API_PATHS } from "@/lib/api-paths"
import type { User, LoginRequest, RegisterRequest } from "@/types"

const USER_CACHE_KEY = "chinwag.cached_user"

function getCachedUser(): User | null {
  if (typeof window === "undefined") return null
  try {
    const raw = window.localStorage.getItem(USER_CACHE_KEY)
    if (!raw) return null
    const parsed = JSON.parse(raw)
    if (parsed && typeof parsed.id === "string" && typeof parsed.name === "string") {
      return parsed as User
    }
    return null
  } catch {
    return null
  }
}

function setCachedUser(user: User | null) {
  if (typeof window === "undefined") return
  try {
    if (user) {
      window.localStorage.setItem(USER_CACHE_KEY, JSON.stringify(user))
    } else {
      window.localStorage.removeItem(USER_CACHE_KEY)
    }
  } catch {
    // storage unavailable, ignore
  }
}

function isAuthUnavailable(err: unknown): boolean {
  return err instanceof ApiError && err.status >= 500
}

interface AuthContextType {
  user: User | null
  isLoading: boolean
  isAuthenticated: boolean
  readOnly: boolean
  login: (data: LoginRequest) => Promise<void>
  register: (data: RegisterRequest) => Promise<void>
  logout: () => void
  checkSession: () => Promise<boolean>
  refreshSession: () => Promise<void>
}

const AuthContext = createContext<AuthContextType | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [readOnly, setReadOnly] = useState(false)
  const router = useRouter()

  const restoreSession = useCallback(async () => {
    try {
      const res = await apiGet<{ user: User }>(API_PATHS.auth.whoami)
      if (res.success && res.data?.user) {
        setUser(res.data.user)
        setCachedUser(res.data.user)
        setReadOnly(false)
        setIsLoading(false)
        return
      }
    } catch (err) {
      if (isAuthUnavailable(err)) {
        const cached = getCachedUser()
        if (cached) {
          setUser(cached)
          setReadOnly(true)
          setIsLoading(false)
          return
        }
      }
    }
    setUser(null)
    setReadOnly(false)
    setIsLoading(false)
  }, [])

  useEffect(() => {
    // restoreSession() is async: its setState calls run after the first await,
    // so this is not a synchronous setState-in-effect (rule false positive).
    // eslint-disable-next-line react-hooks/set-state-in-effect
    restoreSession()
  }, [restoreSession])

  const login = useCallback(
    async (data: LoginRequest) => {
      const res = await apiRequest<{ token: string }>(API_PATHS.auth.login, {
        method: "POST",
        body: data,
        auth: false,
      })
      if (res.success && res.data?.token) {
        setAccessToken(res.data.token)
        const whoami = await apiGet<{ user: User }>(API_PATHS.auth.whoami)
        if (whoami.success && whoami.data?.user) {
          setUser(whoami.data.user)
          setCachedUser(whoami.data.user)
          setReadOnly(false)
          router.push("/home")
          return
        }
      }
      throw new Error(res.message ?? "Login failed")
    },
    [router],
  )

  const register = useCallback(
    async (data: RegisterRequest) => {
      const res = await apiPost<User>(API_PATHS.auth.register, data, false)
      if (res.success) {
        await login({ email: data.email, password: data.password })
      } else {
        throw new Error(res.message ?? "Registration failed")
      }
    },
    [login],
  )

  const logout = useCallback(async () => {
    try {
      await apiPost(API_PATHS.auth.logout, undefined, false)
    } catch {
      // ignore errors
    }
    setUser(null)
    setCachedUser(null)
    setReadOnly(false)
    setAccessToken(null)
    router.push("/login")
  }, [router])

  const checkSession = useCallback(async (): Promise<boolean> => {
    try {
      const res = await apiGet<{ user: User }>(API_PATHS.auth.whoami)
      if (res.success && res.data?.user) {
        setUser(res.data.user)
        setCachedUser(res.data.user)
        setReadOnly(false)
        setIsLoading(false)
        return true
      }
    } catch (err) {
      if (isAuthUnavailable(err)) {
        const cached = getCachedUser()
        if (cached) {
          setUser(cached)
          setReadOnly(true)
          setIsLoading(false)
          return true
        }
      }
    }
    setUser(null)
    setReadOnly(false)
    setIsLoading(false)
    return false
  }, [])

  const refreshSession = useCallback(async () => {
    setIsLoading(true)
    await restoreSession()
  }, [restoreSession])

  return (
    <AuthContext.Provider
      value={{
        user,
        isLoading,
        isAuthenticated: !!user,
        readOnly,
        login,
        register,
        logout,
        checkSession,
        refreshSession,
      }}
    >
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const context = useContext(AuthContext)
  if (!context) {
    throw new Error("useAuth must be used within an AuthProvider")
  }
  return context
}
