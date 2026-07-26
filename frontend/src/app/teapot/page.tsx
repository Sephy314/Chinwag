import Link from "next/link"
import { Coffee } from "lucide-react"

export default function Teapot() {
  return (
    <div className="flex min-h-screen flex-col items-center justify-center px-4 text-center">
      <Coffee className="h-16 w-16 text-gray-700 mb-6" />
      <h1 className="text-4xl font-bold text-gray-100 mb-2">418</h1>
      <p className="text-gray-500 mb-8">
        I&apos;m a teapot.
      </p>
      <Link
        href="/home"
        className="inline-flex items-center justify-center rounded-lg bg-blue-600 px-6 py-2.5 text-sm font-medium text-white hover:bg-blue-700 transition-colors"
      >
        Go home
      </Link>
    </div>
  )
}
