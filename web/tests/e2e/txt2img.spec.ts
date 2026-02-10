import { test, expect } from '@playwright/test';
import { setPromptViaSdkModal } from './helpers/promptEditor';

test('txt2img happy path', async ({ page }) => {
  await page.goto('/txt2img');

  const generate = page.getByTestId('generate');
  const resultGrid = page.getByTestId('result-grid');

  await setPromptViaSdkModal(page, 'A cute banana in space');
  await generate.click();

  const images = resultGrid.locator('img');
  await expect(images).toHaveCount(1, { timeout: 20000 });
});

