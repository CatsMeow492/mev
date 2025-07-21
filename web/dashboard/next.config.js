/** @type {import('next').NextConfig} */
const nextConfig = {
  // Production optimizations
  experimental: {
    optimizeCss: true,
    scrollRestoration: true,
    legacyBrowsers: false,
    browsersListForSwc: true,
  },

  // Compiler optimizations
  compiler: {
    // Remove console.log in production
    removeConsole: process.env.NODE_ENV === 'production' ? {
      exclude: ['error', 'warn']
    } : false,
    
    // React compiler optimizations
    reactRemoveProperties: process.env.NODE_ENV === 'production' ? {
      properties: ['^data-testid$']
    } : false,
  },

  // Build optimizations
  swcMinify: true,
  poweredByHeader: false,
  generateEtags: false,
  compress: true,

  // Environment-specific configuration
  env: {
    CUSTOM_KEY: process.env.CUSTOM_KEY,
    NEXT_PUBLIC_APP_ENV: process.env.NEXT_PUBLIC_APP_ENV || 'development',
    NEXT_PUBLIC_API_URL: process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080',
    NEXT_PUBLIC_WS_URL: process.env.NEXT_PUBLIC_WS_URL || 'ws://localhost:8080',
    NEXT_PUBLIC_VERCEL_URL: process.env.VERCEL_URL,
    NEXT_PUBLIC_SENTRY_DSN: process.env.NEXT_PUBLIC_SENTRY_DSN,
  },

  // Production-specific settings
  ...(process.env.NODE_ENV === 'production' && {
    output: 'standalone',
    generateBuildId: async () => {
      // Use Git commit hash or timestamp for build ID
      return process.env.VERCEL_GIT_COMMIT_SHA || 
             process.env.GITHUB_SHA || 
             `build-${Date.now()}`;
    },
  }),

  // Asset optimization
  images: {
    formats: ['image/avif', 'image/webp'],
    deviceSizes: [640, 750, 828, 1080, 1200, 1920, 2048, 3840],
    imageSizes: [16, 32, 48, 64, 96, 128, 256, 384],
    domains: [],
    dangerouslyAllowSVG: false,
    contentSecurityPolicy: "default-src 'self'; script-src 'none'; sandbox;",
    remotePatterns: [
      {
        protocol: 'https',
        hostname: '**.vercel.app',
      },
      {
        protocol: 'https',
        hostname: 'mev-engine.com',
      },
    ],
  },

  // Dynamic environment-based rewrites
  async rewrites() {
    const apiUrl = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';
    // Convert WebSocket URL to HTTP for rewrites (WebSocket upgrade happens automatically)
    const wsUrl = (process.env.NEXT_PUBLIC_WS_URL || 'ws://localhost:8080').replace('ws://', 'http://').replace('wss://', 'https://');
    
    // Development rewrites (local proxy)
    if (process.env.NODE_ENV === 'development') {
      return [
        {
          source: '/api/:path*',
          destination: `${apiUrl}/api/:path*`,
        },
        {
          source: '/ws/:path*',
          destination: `${wsUrl}/ws/:path*`,
        },
      ];
    }

    // Production rewrites (Edge Functions or external APIs)
    return [
      {
        source: '/api/:path*',
        destination: '/api/proxy/:path*', // Route to Edge Function
      },
      {
        source: '/ws/:path*',
        destination: '/api/websocket/:path*', // Route to WebSocket Edge Function
      },
    ];
  },

  // Security headers
  async headers() {
    return [
      {
        source: '/(.*)',
        headers: [
          {
            key: 'X-Frame-Options',
            value: 'DENY',
          },
          {
            key: 'X-Content-Type-Options',
            value: 'nosniff',
          },
          {
            key: 'Referrer-Policy',
            value: 'strict-origin-when-cross-origin',
          },
          {
            key: 'X-DNS-Prefetch-Control',
            value: 'on',
          },
          {
            key: 'Strict-Transport-Security',
            value: 'max-age=31536000; includeSubDomains; preload',
          },
          {
            key: 'Permissions-Policy',
            value: 'camera=(), microphone=(), geolocation=(), browsing-topics=()',
          },
          {
            key: 'Content-Security-Policy',
            value: [
              "default-src 'self'",
              "script-src 'self' 'unsafe-eval' 'unsafe-inline' https://vercel.live",
              "style-src 'self' 'unsafe-inline' https://fonts.googleapis.com",
              "font-src 'self' https://fonts.gstatic.com",
              "img-src 'self' data: https:",
              "connect-src 'self' wss: https:",
              "frame-src 'none'",
              "object-src 'none'",
              "base-uri 'self'",
              "form-action 'self'",
              "upgrade-insecure-requests",
            ].join('; '),
          },
        ],
      },
    ];
  },

  // Bundle analyzer (optional for debugging)
  ...(process.env.ANALYZE === 'true' && {
    webpack: (config, { buildId, dev, isServer, defaultLoaders, webpack }) => {
      if (!dev && !isServer) {
        const { BundleAnalyzerPlugin } = require('webpack-bundle-analyzer');
        config.plugins.push(
          new BundleAnalyzerPlugin({
            analyzerMode: 'static',
            reportFilename: '../bundle-analyzer-report.html',
            openAnalyzer: false,
          })
        );
      }
      return config;
    },
  }),

  // TypeScript configuration
  typescript: {
    ignoreBuildErrors: false,
  },

  // ESLint configuration
  eslint: {
    dirs: ['src'],
    ignoreDuringBuilds: false,
  },

  // Redirects for domain management
  async redirects() {
    return [
      {
        source: '/dashboard',
        destination: '/',
        permanent: true,
      },
      // Add custom domain redirects here
      ...(process.env.NEXT_PUBLIC_DOMAIN && process.env.VERCEL_URL && process.env.VERCEL_URL !== process.env.NEXT_PUBLIC_DOMAIN ? [
        {
          source: '/:path*',
          has: [
            {
              type: 'host',
              value: process.env.VERCEL_URL,
            },
          ],
          destination: `https://${process.env.NEXT_PUBLIC_DOMAIN}/:path*`,
          permanent: true,
        },
      ] : []),
    ];
  },

  // Experimental features for performance
  experimental: {
    ppr: false, // Partial Pre-rendering (when stable)
    optimizePackageImports: [
      '@heroicons/react',
      '@headlessui/react',
      'framer-motion',
      'recharts',
    ],
    
    // Server components optimization
    serverComponentsExternalPackages: [
      'socket.io-client',
    ],
  },
};

module.exports = nextConfig; 