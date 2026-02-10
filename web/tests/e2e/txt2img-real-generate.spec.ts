import { test, expect } from '@playwright/test';
import path from 'node:path';
import fs from 'node:fs';
import { fileURLToPath } from 'node:url';
import { setPromptViaSdkModal } from './helpers/promptEditor';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

test('txt2img real generate through mock-llm', async ({ page }) => {
  await page.goto('/txt2img');

  const generate = page.getByTestId('generate');
  const resultGrid = page.getByTestId('result-grid');

  await setPromptViaSdkModal(page, 'A real banana through mock-llm full chain test');

  await generate.click();

  const images = resultGrid.locator('img');

  await expect(images).toHaveCount(1, { timeout: 30000 });
  await expect(images.first()).toBeVisible();

  const imgSrc = await images.first().getAttribute('src');
  console.log('Generated image src:', imgSrc);
  
  const evidenceDir = path.resolve(__dirname, '../../../.sisyphus/evidence');
  if (!fs.existsSync(evidenceDir)) {
    fs.mkdirSync(evidenceDir, { recursive: true });
  }

  const screenshotPath = path.join(evidenceDir, 'task3-e2e-happy.png');
  await page.screenshot({ path: screenshotPath });
  console.log(`Screenshot saved to ${screenshotPath}`);
});
