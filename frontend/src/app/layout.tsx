import type { Metadata } from "next"
import { ThemeProvider } from "next-themes"
import { Providers } from "./providers"
import "./globals.css"

export const metadata: Metadata = {
  title: "Chinwag",
  description: "Simple streaming chat application",
}

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode
}>) {
  return (
    <html lang="en" className="dark" suppressHydrationWarning>
      <body>
        <ThemeProvider
          attribute="class"
          defaultTheme="dark"
          forcedTheme="dark"
          disableTransitionOnChange
        >
          <Providers>{children}</Providers>
        </ThemeProvider>
      </body>
    </html>
  )
}
