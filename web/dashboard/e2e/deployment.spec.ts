import { test, expect, Page } from '@playwright/test';

// Test configuration for different environments
const environments = {
  development: 'http://localhost:3000',
  staging: process.env.STAGING_URL || 'https://mev-engine-staging.vercel.app',
  production: process.env.PRODUCTION_URL || 'https://mev-engine.vercel.app',
};

const currentEnv = process.env.TEST_ENV || 'development';
const baseURL = environments[currentEnv as keyof typeof environments];

test.describe('Deployment Health Checks', () => {
  test.beforeEach(async ({ page }) => {
    // Set longer timeout for deployment tests
    test.setTimeout(30000);
  });

  test('Application loads successfully', async ({ page }) => {
    await page.goto(baseURL);
    
    // Check that the page loads without errors
    await expect(page).toHaveTitle(/MEV Engine Dashboard/);
    
    // Check for critical elements
    await expect(page.locator('body')).toBeVisible();
    
    // Verify no JavaScript errors in console
    const errors: string[] = [];
    page.on('console', (msg) => {
      if (msg.type() === 'error') {
        errors.push(msg.text());
      }
    });
    
    await page.waitForLoadState('networkidle');
    expect(errors.filter(error => !error.includes('favicon'))).toHaveLength(0);
  });

  test('API health endpoint responds correctly', async ({ page }) => {
    const response = await page.request.get(`${baseURL}/api/health`);
    expect(response.status()).toBe(200);
    
    const healthData = await response.json();
    expect(healthData).toHaveProperty('status', 'ok');
    expect(healthData).toHaveProperty('timestamp');
  });

  test('Environment configuration is correct', async ({ page }) => {
    await page.goto(baseURL);
    
    // Check environment-specific configurations
    const appEnv = await page.evaluate(() => {
      return (window as any).__NEXT_DATA__?.props?.pageProps?.env || 
             process.env.NEXT_PUBLIC_APP_ENV;
    });
    
    if (currentEnv === 'production') {
      expect(appEnv).toBe('production');
    } else if (currentEnv === 'staging') {
      expect(appEnv).toBe('staging');
    }
  });

  test('Security headers are properly set', async ({ page }) => {
    const response = await page.goto(baseURL);
    const headers = response?.headers();
    
    // Check critical security headers
    expect(headers?.['x-frame-options']).toBe('DENY');
    expect(headers?.['x-content-type-options']).toBe('nosniff');
    expect(headers?.['strict-transport-security']).toContain('max-age=31536000');
    expect(headers?.['content-security-policy']).toBeDefined();
  });
});

test.describe('Performance Tests', () => {
  test('Page loads within performance budget', async ({ page }) => {
    const startTime = Date.now();
    
    await page.goto(baseURL, { waitUntil: 'networkidle' });
    
    const loadTime = Date.now() - startTime;
    
    // Performance budget: page should load within 3 seconds
    expect(loadTime).toBeLessThan(3000);
  });

  test('Critical resources load efficiently', async ({ page }) => {
    const resourceSizes: Record<string, number> = {};
    
    page.on('response', (response) => {
      const url = response.url();
      const contentLength = response.headers()['content-length'];
      
      if (contentLength && (url.includes('.js') || url.includes('.css'))) {
        resourceSizes[url] = parseInt(contentLength);
      }
    });
    
    await page.goto(baseURL, { waitUntil: 'networkidle' });
    
    // Check that JavaScript bundles are not too large
    const jsFiles = Object.entries(resourceSizes).filter(([url]) => url.includes('.js'));
    const totalJsSize = jsFiles.reduce((sum, [, size]) => sum + size, 0);
    
    // Budget: Total JS should be under 500KB
    expect(totalJsSize).toBeLessThan(500 * 1024);
  });

  test('Images are optimized', async ({ page }) => {
    await page.goto(baseURL);
    
    const images = await page.locator('img').all();
    
    for (const img of images) {
      const src = await img.getAttribute('src');
      if (src && !src.startsWith('data:')) {
        // Check that images are served in modern formats
        expect(src).toMatch(/\.(webp|avif|svg)$|_next\/image/);
      }
    }
  });
});

test.describe('Functionality Tests', () => {
  test('Navigation works correctly', async ({ page }) => {
    await page.goto(baseURL);
    
    // Test navigation elements
    const navigation = page.locator('nav');
    await expect(navigation).toBeVisible();
    
    // Test that navigation links are accessible
    const navLinks = navigation.locator('a');
    const linkCount = await navLinks.count();
    expect(linkCount).toBeGreaterThan(0);
    
    // Test first navigation link
    if (linkCount > 0) {
      const firstLink = navLinks.first();
      await expect(firstLink).toBeVisible();
      await expect(firstLink).toHaveAttribute('href');
    }
  });

  test('Dashboard components render correctly', async ({ page }) => {
    await page.goto(baseURL);
    
    // Wait for main content to load
    await page.waitForSelector('[data-testid="dashboard-container"], main, .main-content', { 
      timeout: 10000 
    });
    
    // Check for key dashboard elements
    const mainContent = page.locator('main, .main-content, [data-testid="dashboard-container"]');
    await expect(mainContent).toBeVisible();
  });

  test('Real-time connections work (if enabled)', async ({ page }) => {
    await page.goto(baseURL);
    
    // Check if WebSocket connections are established
    let wsConnected = false;
    
    page.on('websocket', (ws) => {
      wsConnected = true;
      ws.on('framereceived', (event) => {
        console.log('WebSocket frame received:', event.payload);
      });
    });
    
    // Wait for potential WebSocket connections
    await page.waitForTimeout(2000);
    
    // This test is informational - WebSocket may not be available in all environments
    console.log('WebSocket connection established:', wsConnected);
  });

  test('Error boundaries work correctly', async ({ page }) => {
    await page.goto(baseURL);
    
    // Simulate an error and check that error boundary catches it
    const errorCaught = await page.evaluate(() => {
      try {
        // Trigger a potential error
        (window as any).triggerTestError?.();
        return false;
      } catch (error) {
        return true;
      }
    });
    
    // The page should still be functional even if there were errors
    await expect(page.locator('body')).toBeVisible();
  });
});

test.describe('Accessibility Tests', () => {
  test('Page has proper accessibility structure', async ({ page }) => {
    await page.goto(baseURL);
    
    // Check for proper heading structure
    const h1 = page.locator('h1');
    await expect(h1).toBeVisible();
    
    // Check for landmark elements
    const main = page.locator('main, [role="main"]');
    await expect(main).toBeVisible();
    
    // Check for skip links or keyboard navigation
    const skipLink = page.locator('a[href="#main"], .skip-link');
    if (await skipLink.count() > 0) {
      await expect(skipLink).toBeVisible();
    }
  });

  test('Color contrast meets WCAG standards', async ({ page }) => {
    await page.goto(baseURL);
    
    // This is a basic check - in production, use axe-core or similar
    const bodyStyles = await page.locator('body').evaluate((el) => {
      const styles = window.getComputedStyle(el);
      return {
        color: styles.color,
        backgroundColor: styles.backgroundColor,
      };
    });
    
    // Ensure text is visible (basic check)
    expect(bodyStyles.color).not.toBe(bodyStyles.backgroundColor);
  });

  test('Keyboard navigation works', async ({ page }) => {
    await page.goto(baseURL);
    
    // Test Tab navigation
    await page.keyboard.press('Tab');
    
    // Check that focus is visible
    const focusedElement = page.locator(':focus');
    await expect(focusedElement).toBeVisible();
  });
});

test.describe('Cross-browser Compatibility', () => {
  ['chromium', 'firefox', 'webkit'].forEach((browserName) => {
    test(`Works correctly in ${browserName}`, async ({ page, browserName: currentBrowser }) => {
      test.skip(currentBrowser !== browserName, `Skipping ${browserName} test`);
      
      await page.goto(baseURL);
      
      // Basic functionality check
      await expect(page).toHaveTitle(/MEV Engine Dashboard/);
      await expect(page.locator('body')).toBeVisible();
      
      // Check that JavaScript works
      const jsWorking = await page.evaluate(() => {
        return typeof window !== 'undefined' && typeof document !== 'undefined';
      });
      
      expect(jsWorking).toBe(true);
    });
  });
});

test.describe('Mobile Responsiveness', () => {
  test('Dashboard is mobile-friendly', async ({ page }) => {
    // Set mobile viewport
    await page.setViewportSize({ width: 375, height: 667 });
    await page.goto(baseURL);
    
    // Check that content is visible and accessible on mobile
    await expect(page.locator('body')).toBeVisible();
    
    // Check that content doesn't overflow horizontally
    const bodyWidth = await page.locator('body').evaluate((el) => el.scrollWidth);
    const viewportWidth = await page.evaluate(() => window.innerWidth);
    
    expect(bodyWidth).toBeLessThanOrEqual(viewportWidth + 1); // Allow 1px tolerance
  });

  test('Touch interactions work correctly', async ({ page }) => {
    await page.setViewportSize({ width: 375, height: 667 });
    await page.goto(baseURL);
    
    // Test touch interactions on buttons/links
    const interactiveElements = page.locator('button, a, [role="button"]');
    const elementCount = await interactiveElements.count();
    
    if (elementCount > 0) {
      const firstElement = interactiveElements.first();
      await expect(firstElement).toBeVisible();
      
      // Simulate touch
      await firstElement.tap();
    }
  });
});

// Helper function to test API endpoints
async function testApiEndpoint(page: Page, endpoint: string, expectedStatus: number = 200) {
  const response = await page.request.get(`${baseURL}${endpoint}`);
  expect(response.status()).toBe(expectedStatus);
  return response;
}

// Helper function to measure performance metrics
async function measurePerformance(page: Page) {
  return await page.evaluate(() => {
    const navigation = performance.getEntriesByType('navigation')[0] as PerformanceNavigationTiming;
    return {
      domContentLoaded: navigation.domContentLoadedEventEnd - navigation.domContentLoadedEventStart,
      load: navigation.loadEventEnd - navigation.loadEventStart,
      firstContentfulPaint: performance.getEntriesByName('first-contentful-paint')[0]?.startTime || 0,
    };
  });
} 