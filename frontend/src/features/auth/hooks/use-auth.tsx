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
import { apiPost, apiGet, setAccessToken } from "@/lib/api-client"
import { API_PATHS } from "@/lib/api-paths"
import type { ApiResponse, User, LoginRequest, RegisterRequest } from "@/types"

interface AuthContextType {
  user: User | null
  isLoading: boolean
  isAuthenticated: boolean
  login: (data: LoginRequest) => Promise<void>
  register: (data: RegisterRequest) => Promise<void>
  logout: () => void
  checkSession: () => Promise<boolean>
}

const AuthContext = createContext<AuthContextType | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const router = useRouter()

  const restoreSession = useCallback(async () => {
    try {
      const res = await apiGet<{ user: User }>(API_PATHS.auth.whoami)
      if (res.success && res.data?.user) {
        setUser(res.data.user)
        setIsLoading(false)
        return
      }
    } catch {
      // not authenticated
    }
    setUser(null)
    setIsLoading(false)
  }, [])

  useEffect(() => {
    restoreSession()
  }, [restoreSession])

  const login = useCallback(
    async (data: LoginRequest) => {
      const res = await apiPost<{ token: string }>(API_PATHS.auth.login, data, false)
      if (res.success && res.data?.token) {
        setAccessToken(res.data.token)
        const whoami = await apiGet<{ user: User }>(API_PATHS.auth.whoami)
        if (whoami.success && whoami.data?.user) {
          setUser(whoami.data.user)
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

  const logout = useCallback(() => {
    setUser(null)
    setAccessToken(null)
    router.push("/login")
  }, [router])

  const checkSession = useCallback(async (): Promise<boolean> => {
    try {
      const res = await apiGet<{ user: User }>(API_PATHS.auth.whoami)
      if (res.success && res.data?.user) {
        setUser(res.data.user)
        setIsLoading(false)
        return true
      }
    } catch {
      // not authenticated
    }
    setUser(null)
    setIsLoading(false)
    return false
  }, [])

  return (
    <AuthContext.Provider
      value={{
        user,
        isLoading,
        isAuthenticated: !!user,
        login,
        register,
        logout,
        checkSession,
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
