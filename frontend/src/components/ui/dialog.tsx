"use client"

import { forwardRef, useCallback, useEffect, useRef, type HTMLAttributes, type ReactNode } from "react"
import { X } from "lucide-react"
import { cn } from "@/lib/utils"

interface DialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  children: ReactNode
}

function Dialog({ open, onOpenChange, children }: DialogProps) {
  if (!open) return null
  return <DialogInner onOpenChange={onOpenChange}>{children}</DialogInner>
}

function DialogInner({
  onOpenChange,
  children,
}: {
  onOpenChange: (open: boolean) => void
  children: ReactNode
}) {
  const overlayRef = useRef<HTMLDivElement>(null)

  const handleKeyDown = useCallback(
    (e: KeyboardEvent) => {
      if (e.key === "Escape") onOpenChange(false)
    },
    [onOpenChange],
  )

  useEffect(() => {
    document.addEventListener("keydown", handleKeyDown)
    document.body.style.overflow = "hidden"
    return () => {
      document.removeEventListener("keydown", handleKeyDown)
      document.body.style.overflow = ""
    }
  }, [handleKeyDown])

  return (
    <div
      ref={overlayRef}
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60"
      onClick={(e) => {
        if (e.target === overlayRef.current) onOpenChange(false)
      }}
      role="dialog"
      aria-modal="true"
    >
      {children}
    </div>
  )
}

const DialogContent = forwardRef<HTMLDivElement, HTMLAttributes<HTMLDivElement>>(
  ({ className, children, ...props }, ref) => (
    <div
      ref={ref}
      className={cn(
        "relative z-50 w-full max-w-lg rounded-xl border border-gray-800 bg-gray-900 p-6 shadow-lg",
        className,
      )}
      {...props}
    >
      {children}
    </div>
  ),
)
DialogContent.displayName = "DialogContent"

const DialogHeader = ({ className, ...props }: HTMLAttributes<HTMLDivElement>) => (
  <div className={cn("flex flex-col space-y-1.5 mb-4", className)} {...props} />
)

const DialogTitle = ({ className, ...props }: HTMLAttributes<HTMLHeadingElement>) => (
  <h2 className={cn("text-lg font-semibold leading-none", className)} {...props} />
)

const DialogDescription = ({ className, ...props }: HTMLAttributes<HTMLParagraphElement>) => (
  <p className={cn("text-sm text-gray-400", className)} {...props} />
)

const DialogClose = ({
  onClick,
  className,
  ...props
}: HTMLAttributes<HTMLButtonElement>) => (
  <button
    onClick={onClick}
    className={cn(
      "absolute right-4 top-4 rounded-sm opacity-70 hover:opacity-100 transition-opacity text-gray-400 hover:text-gray-200",
      className,
    )}
    aria-label="Close"
    {...props}
  >
    <X className="h-4 w-4" />
  </button>
)

export { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogClose }
