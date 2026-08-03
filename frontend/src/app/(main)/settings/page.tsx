"use client"

import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import { useState } from "react"
import { Settings, Loader2 } from "lucide-react"
import { useAuth } from "@/features/auth/hooks/use-auth"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"
import { apiPut, ApiError } from "@/lib/api-client"
import { API_PATHS } from "@/lib/api-paths"
import type { User as UserType } from "@/types"

const passwordSchema = z.object({
  currentPassword: z.string().min(1, "Current password is required"),
  newPassword: z.string().min(6, "New password must be at least 6 characters"),
})

type PasswordForm = z.infer<typeof passwordSchema>

export default function SettingsPage() {
  const { user, readOnly } = useAuth()
  const [error, setError] = useState("")
  const [success, setSuccess] = useState("")

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors, isSubmitting },
  } = useForm<PasswordForm>({
    resolver: zodResolver(passwordSchema),
  })

  const onSubmit = async (data: PasswordForm) => {
    if (!user) return
    setError("")
    setSuccess("")
    try {
      await apiPut<UserType>(API_PATHS.auth.user(user.id), {
        password: data.newPassword,
      })
      setSuccess("Password updated successfully")
      reset()
    } catch (err) {
      if (err instanceof ApiError) {
        setError(err.message)
      } else {
        setError("Failed to update password")
      }
    }
  }

  return (
    <div className="mx-auto max-w-lg px-4 py-8">
      <Card>
        <CardHeader>
          <div className="flex items-center gap-3">
            <Settings className="h-6 w-6 text-gray-400" />
            <div>
              <CardTitle>Settings</CardTitle>
              <CardDescription>Manage your account settings</CardDescription>
            </div>
          </div>
        </CardHeader>
        <CardContent className="space-y-6">
          <div>
            <h3 className="text-sm font-medium text-gray-300 mb-3">
              Update Password
            </h3>
            <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="currentPassword">Current password</Label>
                <Input
                  id="currentPassword"
                  type="password"
                  {...register("currentPassword")}
                />
                {errors.currentPassword && (
                  <p className="text-xs text-red-400">
                    {errors.currentPassword.message}
                  </p>
                )}
              </div>

              <div className="space-y-2">
                <Label htmlFor="newPassword">New password</Label>
                <Input
                  id="newPassword"
                  type="password"
                  {...register("newPassword")}
                />
                {errors.newPassword && (
                  <p className="text-xs text-red-400">
                    {errors.newPassword.message}
                  </p>
                )}
              </div>

              {error && (
                <div className="rounded-lg bg-red-600/10 border border-red-600/20 px-3 py-2">
                  <p className="text-sm text-red-400">{error}</p>
                </div>
              )}

              {success && (
                <div className="rounded-lg bg-green-600/10 border border-green-600/20 px-3 py-2">
                  <p className="text-sm text-green-400">{success}</p>
                </div>
              )}

              <Button type="submit" disabled={isSubmitting || readOnly}>
                {isSubmitting ? (
                  <Loader2 className="h-4 w-4 animate-spin mr-2" />
                ) : null}
                Update Password
              </Button>
            </form>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
