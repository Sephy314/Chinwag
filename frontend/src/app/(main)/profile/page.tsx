"use client"

import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import { User, Loader2 } from "lucide-react"
import { useAuth } from "@/features/auth/hooks/use-auth"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"
import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import { apiPut, ApiError } from "@/lib/api-client"
import { API_PATHS } from "@/lib/api-paths"
import type { User as UserType } from "@/types"
import { useState } from "react"

const profileSchema = z.object({
  name: z.string().min(2, "Name must be at least 2 characters").max(50).optional(),
  email: z.string().email("Invalid email address").optional(),
})

type ProfileForm = z.infer<typeof profileSchema>

export default function ProfilePage() {
  const { user } = useAuth()
  const [error, setError] = useState("")
  const [success, setSuccess] = useState("")

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<ProfileForm>({
    resolver: zodResolver(profileSchema),
    defaultValues: {
      name: user?.name ?? "",
      email: user?.email ?? "",
    },
  })

  const onSubmit = async (data: ProfileForm) => {
    if (!user) return
    setError("")
    setSuccess("")
    try {
      await apiPut<UserType>(API_PATHS.auth.user(user.id), data)
      setSuccess("Profile updated successfully")
    } catch (err) {
      if (err instanceof ApiError) {
        setError(err.message)
      } else {
        setError("Failed to update profile")
      }
    }
  }

  const initials = user?.name
    ?.split(" ")
    .map((n) => n[0])
    .join("")
    .toUpperCase()
    .slice(0, 2) ?? "U"

  return (
    <div className="mx-auto max-w-lg px-4 py-8">
      <Card>
        <CardHeader>
          <div className="flex items-center gap-4">
            <Avatar className="h-16 w-16">
              <AvatarFallback className="text-lg">{initials}</AvatarFallback>
            </Avatar>
            <div>
              <CardTitle>Profile</CardTitle>
              <CardDescription>Update your personal information</CardDescription>
            </div>
          </div>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="name">Display name</Label>
              <Input id="name" {...register("name")} />
              {errors.name && (
                <p className="text-xs text-red-400">{errors.name.message}</p>
              )}
            </div>

            <div className="space-y-2">
              <Label htmlFor="email">Email</Label>
              <Input id="email" type="email" {...register("email")} />
              {errors.email && (
                <p className="text-xs text-red-400">{errors.email.message}</p>
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

            <Button type="submit" disabled={isSubmitting}>
              {isSubmitting ? (
                <Loader2 className="h-4 w-4 animate-spin mr-2" />
              ) : null}
              Save Changes
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
