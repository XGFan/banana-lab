import { test, expect } from '@playwright/test';

test('LibraryModal infinite scroll', async ({ page }) => {
  const mockImages = (count: number, startId: number) => 
    Array.from({ length: count }, (_, i) => ({
      id: `img-${startId + i}`,
      origin: 'generated',
      file_path: `/test/path/${startId + i}.png`,
      created_at: new Date().toISOString(),
    }));

  let page2Requested = false;

  await page.route('**/api/library**', async (route) => {
    const url = new URL(route.request().url());
    const pageParam = url.searchParams.get('page');

    if (pageParam === '1') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(mockImages(60, 0)),
      });
    } else if (pageParam === '2') {
      page2Requested = true;
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(mockImages(10, 60)),
      });
    } else {
      await route.continue();
    }
  });

  await page.goto('/img2img');

  await page.getByTestId('upload-card').getByRole('button', { name: 'Library' }).click();

  const grid = page.getByTestId('library-grid');
  await expect(grid.locator('img')).toHaveCount(60);

  await grid.evaluate((el) => {
    el.scrollTop = el.scrollHeight;
    el.dispatchEvent(new Event('scroll'));
  });

  await expect.poll(() => page2Requested).toBeTruthy();

  await expect(grid.locator('img')).toHaveCount(70);
});
