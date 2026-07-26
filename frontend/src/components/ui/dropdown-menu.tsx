"use client"

import {
  createContext,
  forwardRef,
  useContext,
  useEffect,
  useRef,
  useState,
  type HTMLAttributes,
  type ReactNode,
} from "react"
import { cn } from "@/lib/utils"

interface DropdownMenuContextType {
  open: boolean
  setOpen: (open: boolean) => void
}

const DropdownMenuContext = createContext<DropdownMenuContextType>({
  open: false,
  setOpen: () => {},
})

function DropdownMenu({ children }: { children: ReactNode }) {
  const [open, setOpen] = useState(false)
  return (
    <DropdownMenuContext.Provider value={{ open, setOpen }}>
      {children}
    </DropdownMenuContext.Provider>
  )
}

const DropdownMenuTrigger = forwardRef<
  HTMLButtonElement,
  HTMLAttributes<HTMLButtonElement>
>(({ className, onClick, ...props }, ref) => {
  const { open, setOpen } = useContext(DropdownMenuContext)
  return (
    <button
      ref={ref}
      onClick={(e) => {
        setOpen(!open)
        onClick?.(e)
      }}
      className={cn("", className)}
      aria-haspopup="true"
      aria-expanded={open}
      {...props}
    />
  )
})
DropdownMenuTrigger.displayName = "DropdownMenuTrigger"

function DropdownMenuContent({
  className,
  children,
  align = "start",
  ...props
}: HTMLAttributes<HTMLDivElement> & { align?: "start" | "end" }) {
  const { open, setOpen } = useContext(DropdownMenuContext)
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) return
    const handleClickOutside = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    const handleEscape = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false)
    }
    document.addEventListener("mousedown", handleClickOutside)
    document.addEventListener("keydown", handleEscape)
    return () => {
      document.removeEventListener("mousedown", handleClickOutside)
      document.removeEventListener("keydown", handleEscape)
    }
  }, [open, setOpen])

  if (!open) return null

  return (
    <div
      ref={ref}
      className={cn(
        "absolute z-50 mt-1 min-w-[12rem] rounded-lg border border-gray-800 bg-gray-900 p-1 shadow-lg",
        align === "end" ? "right-0" : "left-0",
        className,
      )}
      role="menu"
      {...props}
    >
      {children}
    </div>
  )
}

const DropdownMenuItem = forwardRef<
  HTMLButtonElement,
  HTMLAttributes<HTMLButtonElement> & { inset?: boolean }
>(({ className, inset, ...props }, ref) => {
  const { setOpen } = useContext(DropdownMenuContext)
  return (
    <button
      ref={ref}
      onClick={(e) => {
        setOpen(false)
        props.onClick?.(e)
      }}
      className={cn(
        "relative flex w-full cursor-default select-none items-center rounded-md px-3 py-2 text-sm text-gray-300 outline-none hover:bg-gray-800 hover:text-gray-100 data-[disabled]:pointer-events-none data-[disabled]:opacity-50",
        inset && "pl-8",
        className,
      )}
      role="menuitem"
      {...props}
    />
  )
})
DropdownMenuItem.displayName = "DropdownMenuItem"

const DropdownMenuSeparator = ({ className, ...props }: HTMLAttributes<HTMLDivElement>) => (
  <div className={cn("my-1 h-px bg-gray-800", className)} {...props} />
)

export {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
}
