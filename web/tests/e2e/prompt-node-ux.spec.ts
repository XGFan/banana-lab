import { test, expect } from '@playwright/test';
import { setPromptViaSdkModal } from './helpers/promptEditor';

const createNode = (id: string, prompt: string, overrides: any = {}) => {
  const now = new Date().toISOString();
  return {
    id,
    mode: 'txt2img',
    prompt,
    title: prompt || 'New Node',
    pinned: false,
    last_used_at: now,
    created_at: now,
    updated_at: now,
    archived_at: null,
    deleted_at: null,
    parent_id: null,
    ...overrides,
  };
};

const createImage = (id: string, nodeId?: string) => ({
  id,
  node_id: nodeId,
  origin: 'generated',
  file_path: `images/${id}.png`,
  created_at: new Date().toISOString(),
});

test.describe('Prompt Node UX Enhancements', () => {
  test.beforeEach(async ({ page }) => {
    await page.route('**/api/prompts?mode=txt2img*', route => route.fulfill({ json: [] }));
    await page.route('**/api/prompts?mode=img2img*', route => route.fulfill({ json: [] }));
    await page.route('**/api/prompt-snippets', route => route.fulfill({ json: [] }));
    await page.route('**/api/library*', route => route.fulfill({ json: [] }));
    await page.route('**/api/prompts/*/tree', route => route.fulfill({ json: { root_id: 'root', nodes: [] } }));
    
    await page.route('**/api/nodes/*', async (route) => {
        if (route.request().method() === 'GET') {
            await route.fulfill({ status: 404, json: { error: 'not_found' } });
        } else {
            await route.continue();
        }
    });
  });

  test('scenario 1: /txt2img?action=new auto-opens modal', async ({ page }) => {
    await page.goto('/txt2img?action=new');
    await expect(page.getByTestId('breadcrumbs')).toBeVisible({ timeout: 15000 });
    const modal = page.getByTestId('sdk-modal-overlay');
    await expect(modal).toBeVisible({ timeout: 15000 });
    await expect(page).not.toHaveURL(/action=new/);
    await page.screenshot({ path: '../.sisyphus/evidence/prompt-node-ux-action-new.png' });
  });

  test('scenario 2: fork flow', async ({ page }) => {
    const sourceNodeId = 'node-source';
    const sourcePrompt = 'fork me';
    
    await page.route(`**/api/nodes/${sourceNodeId}`, route => route.fulfill({
      json: {
        node: createNode(sourceNodeId, sourcePrompt),
        // fork-prompt-btn only renders when the node has at least one generated image
        // (see Txt2ImgPage.tsx: `{currentNodeId && images.length > 0 && (...fork-prompt-btn...)}`).
        images: [createImage('img-src', sourceNodeId)],
        results: [],
      },
    }));

    let capturedPayload: any = null;
    await page.route('**/api/generate', async (route) => {
      capturedPayload = route.request().postDataJSON();
      await route.fulfill({
        json: {
          node: createNode('node-new', sourcePrompt, { parent_id: sourceNodeId }),
          images: [createImage('img-2', 'node-new')],
          results: [],
        },
      });
    });

    await page.goto(`/txt2img?node_id=${sourceNodeId}`);
    await expect(page.getByTestId('prompt')).toHaveValue(sourcePrompt, { timeout: 15000 });

    await page.getByTestId('prompt').hover();
    await page.getByTestId('fork-prompt-btn').click();

    await expect(page.getByTestId('sdk-modal-overlay')).toBeVisible();
    await page.screenshot({ path: '../.sisyphus/evidence/prompt-node-ux-fork-prefill.png' });
    
    await page.getByTestId('sdk-save-btn').click();
    await expect(page.getByTestId('sdk-modal-overlay')).not.toBeVisible();
    await expect(page).not.toHaveURL(/action=fork/);

    await page.getByTestId('generate').click();
    await expect.poll(() => capturedPayload).not.toBeNull();
    expect(capturedPayload).toMatchObject({
      parent_node_id: sourceNodeId,
      // SDK editor may prepend a group header (e.g. "##Default\n"); match containment.
      prompt: expect.stringContaining(sourcePrompt)
    });
    
    await page.screenshot({ path: '../.sisyphus/evidence/prompt-node-ux-fork-payload.png' });
  });

  test('scenario 3: view flow negative case', async ({ page }) => {
    const nodeId = 'node-view';
    const sourcePrompt = 'view only';
    
    await page.route(`**/api/nodes/${nodeId}`, route => route.fulfill({
      json: {
        node: createNode(nodeId, sourcePrompt),
        images: [createImage('img-view', nodeId)],
        results: [{ id: 'r-view', image_id: 'img-view' }],
      },
    }));

    await page.goto(`/txt2img?node_id=${nodeId}`);
    await expect(page.getByTestId('prompt')).toHaveValue(sourcePrompt, { timeout: 15000 });

    await page.getByTestId('prompt').hover();
    await page.getByTestId('view-prompt-btn').click();

    await expect(page.getByTestId('sdk-save-btn')).toBeHidden();
    await expect(page.getByRole('button', { name: /close|关闭/i })).toBeVisible();
    
    await page.screenshot({ path: '../.sisyphus/evidence/prompt-node-ux-readonly.png' });
  });

  test('scenario 4: multiline preservation', async ({ page }) => {
    const nodeId = 'node-multiline';
    const multiline = 'Line 1\nLine 2';
    
    await page.route(`**/api/nodes/${nodeId}`, route => route.fulfill({
      json: {
        node: createNode(nodeId, multiline),
        images: [],
        results: [],
      },
    }));

    await page.goto(`/txt2img?node_id=${nodeId}`);
    
    const display = page.getByTestId('prompt');
    // Verify initial load preserves newlines
    await expect(display).toHaveValue(multiline, { timeout: 15000 });

    // Open editor and save without changes
    await page.getByTestId('edit-prompt-btn').click();
    await expect(page.getByTestId('sdk-modal-overlay')).toBeVisible();
    await page.getByTestId('sdk-save-btn').click();
    await expect(page.getByTestId('sdk-modal-overlay')).not.toBeVisible();
    
    // Verify display STILL preserves newlines after "save".
    // Note: SDK editor may prepend a group header (e.g. "##Default\n") on save,
    // so assert the user-entered multiline body via containment rather than exact equality.
    await expect(display).toHaveValue(
      new RegExp(multiline.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'))
    );
    
    await page.screenshot({ path: '../.sisyphus/evidence/prompt-node-ux-multiline.png' });
  });

  test('scenario 5: single Generate CTA behavior', async ({ page }) => {
    const nodeId = 'node-cta';
    const initialPrompt = 'initial';
    await page.route(`**/api/nodes/${nodeId}`, route => route.fulfill({
      json: {
        node: createNode(nodeId, initialPrompt),
        images: [],
        results: [],
      },
    }));

    await page.goto(`/txt2img?node_id=${nodeId}`);
    
    const generateBtn = page.getByTestId('generate');
    await expect(generateBtn).toBeVisible({ timeout: 15000 });
    // Assert visible primary CTA text is "Generate"
    await expect(generateBtn).toHaveText('Generate');
    await expect(page.getByTestId('reroll-count')).toBeVisible();
    
    // Make dirty to see if label changes (it should NOT per Task 5)
    await page.getByTestId('edit-prompt-btn').click();
    // Using simple input to make it dirty
    const e2eInput = page.getByTestId('e2e-sdk-prompt');
    await e2eInput.fill('dirty');
    await page.getByTestId('e2e-sdk-apply').click();
    await page.getByTestId('sdk-save-btn').click();
    
    // Verify label is still "Generate"
    await expect(generateBtn).toHaveText('Generate');
    await expect(page.getByTestId('reroll-count')).toBeHidden();
    
    await page.screenshot({ path: '../.sisyphus/evidence/prompt-node-ux-generate-cta.png' });
  });
});
