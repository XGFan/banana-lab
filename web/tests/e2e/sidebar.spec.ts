import path from 'node:path';
import { test, expect } from '@playwright/test';
import { setPromptViaSdkModal } from './helpers/promptEditor';

test.describe('Sidebar', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/txt2img');
  });

  test('tabs switching', async ({ page }) => {
    const tabTxt2Img = page.getByTestId('tab-txt2img');
    const tabImg2Img = page.getByTestId('tab-img2img');

    await expect(tabTxt2Img).toHaveAttribute('aria-current', 'page');
    await expect(tabImg2Img).not.toHaveAttribute('aria-current', 'page');

    await tabImg2Img.click();

    await expect(page).toHaveURL(/\/img2img/);
    await expect(tabImg2Img).toHaveAttribute('aria-current', 'page');
    await expect(tabTxt2Img).not.toHaveAttribute('aria-current', 'page');
  });

  test('generate prompts and display in sidebar', async ({ page }) => {
    const promptText = `E2E Test Prompt ${Math.random().toString(36).substring(7)}`;
    
    await setPromptViaSdkModal(page, promptText);
    
    await expect(page.getByTestId('generate')).toBeEnabled();
    const genPromise = page.waitForResponse('**/api/generate');
    await page.getByTestId('generate').click();
    await genPromise;
    
    await expect(page).toHaveURL(/node_id=/, { timeout: 15000 });
    const nodeId = new URL(page.url()).searchParams.get('node_id');
    
    const promptsPromise = page.waitForResponse('**/api/prompts*');
    await page.reload();
    await promptsPromise;

    const sidebarList = page.getByTestId('prompts-list');
    await expect(sidebarList).toContainText(promptText);
    
    const item = page.getByTestId(`sidebar-item-${nodeId}`);
    await expect(item).toBeVisible();
  });

  test('navigation by clicking sidebar item', async ({ page }) => {
    // Node A
    await setPromptViaSdkModal(page, 'Node A');
    
    await expect(page.getByTestId('generate')).toBeEnabled();
    const genAPromise = page.waitForResponse('**/api/generate');
    await page.getByTestId('generate').click();
    await genAPromise;
    
    await expect(page).toHaveURL(/node_id=/, { timeout: 15000 });
    const nodeAId = new URL(page.url()).searchParams.get('node_id');

    // New node
    await page.getByTestId('new-node-btn').click();
    await expect(page).not.toHaveURL(/node_id=/);
    
    // Node B
    await setPromptViaSdkModal(page, 'Node B');
    
    await expect(page.getByTestId('generate')).toBeEnabled();
    const genBPromise = page.waitForResponse('**/api/generate');
    await page.getByTestId('generate').click();
    await genBPromise;
    
    await expect(page).toHaveURL(/node_id=/, { timeout: 15000 });
    const nodeBId = new URL(page.url()).searchParams.get('node_id');

    const promptsPromise = page.waitForResponse('**/api/prompts*');
    await page.reload();
    await promptsPromise;

    await page.getByTestId(`sidebar-item-${nodeAId}`).click();
    await expect(page).toHaveURL(new RegExp(`node_id=${nodeAId}`));
    await expect(page.getByTestId('prompt')).toHaveValue('Node A');
    
    await page.getByTestId(`sidebar-item-${nodeBId}`).click();
    await expect(page).toHaveURL(new RegExp(`node_id=${nodeBId}`));
    await expect(page.getByTestId('prompt')).toHaveValue('Node B');
  });

  test('menu actions: rename', async ({ page }) => {
    await setPromptViaSdkModal(page, 'To be renamed');
    
    await expect(page.getByTestId('generate')).toBeEnabled();
    const genPromise = page.waitForResponse('**/api/generate');
    await page.getByTestId('generate').click();
    await genPromise;
    
    await expect(page).toHaveURL(/node_id=/, { timeout: 15000 });
    const nodeId = new URL(page.url()).searchParams.get('node_id');

    const promptsPromise = page.waitForResponse('**/api/prompts*');
    await page.reload();
    await promptsPromise;

    const item = page.getByTestId(`sidebar-item-${nodeId}`);
    await item.hover();
    await item.getByLabel('Options').click();
    await page.getByTestId('menu-rename-btn').click();
    
    const input = item.locator('input[type="text"]');
    await expect(input).toBeVisible();
    await input.fill('New Brilliant Title');
    
    const updatePromise = page.waitForResponse('**/api/prompts/*');
    await input.press('Enter');
    await updatePromise;

    await expect(item).toContainText('New Brilliant Title');
  });

  test('menu actions: pin', async ({ page }) => {
    // Node 1
    await setPromptViaSdkModal(page, 'Node 1');
    await expect(page.getByTestId('generate')).toBeEnabled();
    const gen1Promise = page.waitForResponse('**/api/generate');
    await page.getByTestId('generate').click();
    await gen1Promise;
    await expect(page).toHaveURL(/node_id=/, { timeout: 15000 });
    const node1Id = new URL(page.url()).searchParams.get('node_id');

    // Node 2
    await page.getByTestId('new-node-btn').click();
    await setPromptViaSdkModal(page, 'Node 2');
    await expect(page.getByTestId('generate')).toBeEnabled();
    const gen2Promise = page.waitForResponse('**/api/generate');
    await page.getByTestId('generate').click();
    await gen2Promise;
    await expect(page).toHaveURL(/node_id=/, { timeout: 15000 });
    const node2Id = new URL(page.url()).searchParams.get('node_id');

    const reloadPromise = page.waitForResponse('**/api/prompts*');
    await page.reload();
    await reloadPromise;

    const items = page.locator('[data-testid^="sidebar-item-"]');
    await expect(items.nth(0)).toContainText('Node 2');

    // Pin Node 1
    const item1 = page.getByTestId(`sidebar-item-${node1Id}`);
    await item1.hover();
    await item1.getByLabel('Options').click();
    
    const pinPromise = page.waitForResponse('**/api/prompts/*');
    const reloadAfterPin = page.waitForResponse('**/api/prompts*');
    await page.getByTestId('menu-pin-btn').click();
    await pinPromise;
    await reloadAfterPin;

    await expect(items.nth(0)).toContainText('Node 1');

    // Unpin Node 1
    await item1.hover();
    await item1.getByLabel('Options').click();
    const unpinPromise = page.waitForResponse('**/api/prompts/*');
    const reloadAfterUnpin = page.waitForResponse('**/api/prompts*');
    await page.getByTestId('menu-pin-btn').click();
    await unpinPromise;
    await reloadAfterUnpin;

    await expect(items.nth(0)).toContainText('Node 2');
  });

  test('menu actions: archive', async ({ page }) => {
    await setPromptViaSdkModal(page, 'To be archived');
    await expect(page.getByTestId('generate')).toBeEnabled();
    const genPromise = page.waitForResponse('**/api/generate');
    await page.getByTestId('generate').click();
    await genPromise;
    await expect(page).toHaveURL(/node_id=/, { timeout: 15000 });
    const nodeId = new URL(page.url()).searchParams.get('node_id');

    const reloadPromise = page.waitForResponse('**/api/prompts*');
    await page.reload();
    await reloadPromise;

    const item = page.getByTestId(`sidebar-item-${nodeId}`);
    await item.hover();
    await item.getByLabel('Options').click();
    
    page.on('dialog', dialog => dialog.accept());
    const archivePromise = page.waitForResponse('**/api/prompts/*');
    await page.getByTestId('menu-archive-btn').click();
    await archivePromise;

    await expect(item).not.toBeVisible();
    await expect(page).not.toHaveURL(/node_id=/);
  });

  test('banana lab overlap - sidebar header should contain title', async ({ page }) => {
    const header = page.getByTestId('sidebar-logo');
    const title = header.getByRole('heading', { name: /Banana Lab/i });
    const screenshotPath = path.resolve(process.cwd(), '..', '.sisyphus', 'evidence', 'sidebar-header-verification.png');

    await expect(header).toBeVisible();
    await expect(title).toBeVisible();

    const headerBox = await header.boundingBox();
    const titleBox = await title.boundingBox();

    await page.screenshot({ path: screenshotPath });

    if (!headerBox || !titleBox) {
      throw new Error('无法获取 sidebar header 或标题的 bounding box');
    }

    expect(titleBox.y).toBeGreaterThanOrEqual(headerBox.y);
    expect(titleBox.y + titleBox.height).toBeLessThanOrEqual(headerBox.y + headerBox.height);
  });
});
