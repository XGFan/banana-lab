import { test, expect } from '@playwright/test';
import path from 'node:path';
import { mkdir } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import { setPromptViaSdkModal } from './helpers/promptEditor';

test('evidence generate empty prompt', async ({ page }) => {
  const currentFilePath = fileURLToPath(import.meta.url);
  const currentDir = path.dirname(currentFilePath);
  const evidenceDir = path.resolve(currentDir, '../../../.sisyphus/evidence');
  await mkdir(evidenceDir, { recursive: true });
  const screenshotPath = path.join(evidenceDir, 'task1-generate-empty.png');

  await page.goto('/txt2img');
  await expect(page).not.toHaveURL(/node_id=/);

  const generateBtn = page.getByTestId('generate');
  await expect(generateBtn).toBeDisabled();

  await setPromptViaSdkModal(page, 'test prompt');
  await expect(generateBtn).toBeEnabled();

  await setPromptViaSdkModal(page, '   ');
  await expect(generateBtn).toBeDisabled();


  await expect(page).not.toHaveURL(/node_id=/);

  await page.screenshot({ path: screenshotPath, fullPage: true });
});
