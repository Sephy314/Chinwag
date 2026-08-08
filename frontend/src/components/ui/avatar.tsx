"use client"

import { forwardRef, type HTMLAttributes } from "react"
import { cn } from "@/lib/utils"

const Avatar = forwardRef<HTMLDivElement, HTMLAttributes<HTMLDivElement>>(
  ({ className, ...props }, ref) => (
    <div
      ref={ref}
      className={cn("relative flex h-10 w-10 shrink-0 overflow-hidden rounded-full", className)}
      {...props}
    />
  ),
)
Avatar.displayName = "Avatar"

const AvatarImage = forwardRef<HTMLImageElement, HTMLAttributes<HTMLImageElement>>(
  ({ className, ...props }, ref) => (
    <img ref={ref} alt="" className={cn("aspect-square h-full w-full object-cover", className)} {...props} />
  ),
)
AvatarImage.displayName = "AvatarImage"

const AvatarFallback = forwardRef<HTMLDivElement, HTMLAttributes<HTMLDivElement>>(
  ({ className, ...props }, ref) => (
    <div
      ref={ref}
      className={cn(
        "flex h-full w-full items-center justify-center rounded-full bg-gray-800 text-sm font-medium text-gray-400",
        className,
      )}
      {...props}
    />
  ),
)
AvatarFallback.displayName = "AvatarFallback"

export { Avatar, AvatarImage, AvatarFallback }
