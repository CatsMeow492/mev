/** @type {import('next-sitemap').IConfig} */
module.exports = {
  siteUrl: process.env.NEXT_PUBLIC_DOMAIN || process.env.NEXT_PUBLIC_VERCEL_URL || 'http://localhost:3000',
  generateRobotsTxt: true,
  generateIndexSitemap: true,
  exclude: [
    '/api/*',
    '/admin/*',
    '/private/*',
    '/_next/*',
    '/server-sitemap-index.xml',
  ],
  
  // Additional paths to include
  additionalPaths: async (config) => [
    await config.transform(config, '/'),
    await config.transform(config, '/dashboard'),
    await config.transform(config, '/analytics'),
    await config.transform(config, '/settings'),
  ],
  
  // Custom robots.txt rules
  robotsTxtOptions: {
    policies: [
      {
        userAgent: '*',
        allow: '/',
        disallow: [
          '/api/',
          '/admin/',
          '/private/',
          '/_next/',
        ],
      },
    ],
    additionalSitemaps: [
      `${process.env.NEXT_PUBLIC_DOMAIN || process.env.NEXT_PUBLIC_VERCEL_URL || 'http://localhost:3000'}/server-sitemap-index.xml`,
    ],
  },
  
  // Transform function to customize URLs
  transform: async (config, path) => {
    // Custom priority and change frequency
    const customPages = {
      '/': { priority: 1.0, changefreq: 'daily' },
      '/dashboard': { priority: 0.9, changefreq: 'hourly' },
      '/analytics': { priority: 0.8, changefreq: 'daily' },
      '/settings': { priority: 0.6, changefreq: 'weekly' },
    };

    const customConfig = customPages[path] || {
      priority: 0.7,
      changefreq: 'daily',
    };

    return {
      loc: path,
      lastmod: new Date().toISOString(),
      priority: customConfig.priority,
      changefreq: customConfig.changefreq,
    };
  },
}; 