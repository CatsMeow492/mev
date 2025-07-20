import type { Metadata } from 'next'
import { Inter } from 'next/font/google'
import './globals.css'

const inter = Inter({ subsets: ['latin'] })

export const metadata: Metadata = {
  title: 'MEV Engine Dashboard',
  description: 'Real-time monitoring of MEV opportunities on Base Layer 2',
}

export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <html lang="en" className="dark">
      <body className={`${inter.className} bg-mev-darker text-gray-100 min-h-screen`}>
        {children}
      </body>
    </html>
  )
} 