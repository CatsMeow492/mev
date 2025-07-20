import { test, expect } from '@playwright/test';

test.describe('MEV Engine Dashboard', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
  });

  test('should display the dashboard header', async ({ page }) => {
    await expect(page.locator('h1')).toContainText('MEV Engine Dashboard');
    await expect(page.locator('[data-testid="connection-status"]')).toBeVisible();
  });

  test('should have navigation tabs', async ({ page }) => {
    await expect(page.locator('button:has-text("Opportunities")')).toBeVisible();
    await expect(page.locator('button:has-text("Metrics")')).toBeVisible();
    await expect(page.locator('button:has-text("System")')).toBeVisible();
  });

  test('should switch between tabs', async ({ page }) => {
    // Start on opportunities tab
    await expect(page.locator('button:has-text("Opportunities")')).toHaveClass(/bg-mev-primary/);
    
    // Switch to metrics tab
    await page.click('button:has-text("Metrics")');
    await expect(page.locator('button:has-text("Metrics")')).toHaveClass(/bg-mev-primary/);
    
    // Switch to system tab
    await page.click('button:has-text("System")');
    await expect(page.locator('button:has-text("System")')).toHaveClass(/bg-mev-primary/);
  });

  test('should display opportunity filters when on opportunities tab', async ({ page }) => {
    await page.click('button:has-text("Opportunities")');
    
    await expect(page.locator('select')).toBeVisible();
    await expect(page.locator('label:has-text("Strategy:")')).toBeVisible();
    await expect(page.locator('label:has-text("Status:")')).toBeVisible();
    await expect(page.locator('input[type="checkbox"]')).toBeVisible();
  });

  test('should display metrics when on metrics tab', async ({ page }) => {
    await page.click('button:has-text("Metrics")');
    
    // Should show metric cards
    await expect(page.locator('text=Total Trades')).toBeVisible();
    await expect(page.locator('text=Success Rate')).toBeVisible();
    await expect(page.locator('text=Loss Rate')).toBeVisible();
    await expect(page.locator('text=Avg Latency')).toBeVisible();
  });

  test('should display system status when on system tab', async ({ page }) => {
    await page.click('button:has-text("System")');
    
    await expect(page.locator('text=Connection Status')).toBeVisible();
    await expect(page.locator('text=Mempool Connections')).toBeVisible();
    await expect(page.locator('text=Active Simulations')).toBeVisible();
    await expect(page.locator('text=Queue Size')).toBeVisible();
  });

  test('should have emergency controls in header', async ({ page }) => {
    await expect(page.locator('button:has-text("Restart")')).toBeVisible();
    await expect(page.locator('button:has-text("Emergency Stop")')).toBeVisible();
  });

  test('should handle emergency shutdown confirmation', async ({ page }) => {
    // Mock the confirm dialog
    page.on('dialog', async dialog => {
      expect(dialog.message()).toContain('emergency shutdown');
      await dialog.accept();
    });

    await page.click('button:has-text("Emergency Stop")');
  });

  test('should filter opportunities by strategy', async ({ page }) => {
    await page.click('button:has-text("Opportunities")');
    
    // Change strategy filter
    await page.selectOption('select:near(text="Strategy:")', 'sandwich');
    
    // Should update the display
    await expect(page.locator('text=Showing')).toBeVisible();
  });

  test('should toggle profitable only filter', async ({ page }) => {
    await page.click('button:has-text("Opportunities")');
    
    const checkbox = page.locator('input[type="checkbox"]:near(text="Profitable Only")');
    await checkbox.check();
    await expect(checkbox).toBeChecked();
    
    await checkbox.uncheck();
    await expect(checkbox).not.toBeChecked();
  });

  test('should sort opportunities by different criteria', async ({ page }) => {
    await page.click('button:has-text("Opportunities")');
    
    // Click profit sort button
    await page.click('button:has-text("Profit")');
    
    // Should show active state
    await expect(page.locator('button:has-text("Profit")')).toHaveClass(/bg-mev-primary/);
  });

  test('should be responsive on mobile', async ({ page }) => {
    await page.setViewportSize({ width: 375, height: 667 });
    
    // Header should still be visible and functional
    await expect(page.locator('h1')).toBeVisible();
    await expect(page.locator('button:has-text("Opportunities")')).toBeVisible();
    
    // Tabs should work on mobile
    await page.click('button:has-text("Metrics")');
    await expect(page.locator('text=Total Trades')).toBeVisible();
  });
}); 