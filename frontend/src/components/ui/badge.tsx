"use client"

import { forwardRef, type HTMLAttributes } from "react"
import { cva, type VariantProps } from "class-variance-authority"
import { cn } from "@/lib/utils"

const badgeVariants = cva(
  "inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium transition-colors",
  {
    variants: {
      variant: {
        default: "bg-blue-600/20 text-blue-400 border border-blue-600/30",
        secondary: "bg-gray-800 text-gray-400 border border-gray-700",
        destructive: "bg-red-600/20 text-red-400 border border-red-600/30",
        success: "bg-green-600/20 text-green-400 border border-green-600/30",
        outline: "text-gray-400 border border-gray-700",
      },
    },
    defaultVariants: {
      variant: "default",
    },
  },
)

export interface BadgeProps extends HTMLAttributes<HTMLDivElement>, VariantProps<typeof badgeVariants> {}

const Badge = forwardRef<HTMLDivElement, BadgeProps>(({ className, variant, ...props }, ref) => {
  return <div ref={ref} className={cn(badgeVariants({ variant }), className)} {...props} />
})
Badge.displayName = "Badge"

export { Badge, badgeVariants }
