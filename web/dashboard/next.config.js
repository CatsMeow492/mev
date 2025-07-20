/** @type {import('next').NextConfig} */
const nextConfig = {
  async rewrites() {
    return [
      {
        source: '/api/:path*',
        destination: 'http://localhost:8080/api/:path*', // Proxy to Go backend
      },
      {
        source: '/ws/:path*',
        destination: 'http://localhost:8080/ws/:path*', // Proxy WebSocket connections
      },
    ]
  },
}

module.exports = nextConfig 