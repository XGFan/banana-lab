import { test, expect } from '@playwright/test';
import type { Page } from '@playwright/test';
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

const mockPromptsList = async (page: Page) => {
  await page.route('**/api/prompts?mode=txt2img*', async (route) => {
    await route.fulfill({ json: [] });
  });
  await page.route('**/api/prompts?mode=img2img*', async (route) => {
    await route.fulfill({ json: [] });
  });
  await page.route('**/api/prompts/*/tree', async (route) => {
    const url = new URL(route.request().url());
    const parts = url.pathname.split('/');
    const id = parts[parts.length - 2];
    await route.fulfill({ json: { root_id: id, nodes: [] } });
  });
};

test.beforeEach(async ({ page }) => {
  await mockPromptsList(page);
});

test('reroll stop halts looping and images stay under count', async ({ page }) => {
  let generateCount = 0;
  let lastNodeId = '';
  let releaseFirst: (() => void) | null = null;
  const waitForFirst = new Promise<void>((resolve) => {
    releaseFirst = resolve;
  });

  await page.route('**/api/nodes/*', async (route) => {
    const nodeId = route.request().url().split('/').pop();
    await route.fulfill({
      json: {
        node: createNode(nodeId!, 'cat'),
        images: [createImage(`img-refetch`, nodeId!)],
      },
    });
  });

  await page.route('**/api/generate', async (route) => {
    generateCount += 1;
    lastNodeId = `node-${generateCount}`;
    await route.fulfill({
      status: 200,
      json: {
        node: createNode(lastNodeId, 'cat'),
        images: [createImage(`img-${generateCount}`, lastNodeId)],
      },
    });
  });

  await page.route('**/api/prompts/*/reroll', async (route) => {
    generateCount += 1;
    if (generateCount === 2) {
      await waitForFirst;
    }
    await route.fulfill({
      status: 200,
      json: {
        node: createNode(lastNodeId, 'cat'),
        images: Array.from({ length: generateCount }, (_, idx) => 
          createImage(`reroll-${generateCount}-${idx}`, lastNodeId)
        ),
      },
    });
  });

  await page.goto('/txt2img');

  await setPromptViaSdkModal(page, 'cat');
  await page.getByTestId('generate').click();
  
  await expect(page.getByTestId('result-grid').locator('img').first()).toBeVisible({ timeout: 10000 });
  
  const generateBtn = page.getByTestId('generate');
  await expect(generateBtn).toBeVisible();
  await expect(generateBtn).toBeEnabled();

  await page.getByTestId('reroll-count').fill('5');
  await generateBtn.click();

  const stopButton = page.getByTestId('stop-reroll');
  await expect(stopButton).toBeVisible();
  await stopButton.click();
  releaseFirst?.();

  await expect(page.getByTestId('generate')).toBeVisible();
  await expect.poll(() => generateCount).toBeLessThan(6);
});

test('result preview modal opens and closes with ESC', async ({ page }) => {
  await page.route('**/api/nodes/*', async (route) => {
    const nodeId = route.request().url().split('/').pop();
    await route.fulfill({
      json: {
        node: createNode(nodeId!, 'sunset'),
        images: [createImage('preview-1', nodeId!)],
      },
    });
  });

  await page.route('**/api/generate', async (route) => {
    await route.fulfill({
      status: 200,
      json: {
        node: createNode('node-preview', 'sunset'),
        images: [createImage('preview-1', 'node-preview')],
      },
    });
  });

  await page.goto('/txt2img');

  await setPromptViaSdkModal(page, 'sunset');
  await page.getByTestId('generate').click();

  const grid = page.getByTestId('result-grid');
  await expect(grid.locator('img').first()).toBeVisible({ timeout: 10000 });
  
  const imageCard = grid.locator('button').filter({ has: page.locator('img') }).first();
  await imageCard.click();

  const modalMask = page.getByTestId('preview-modal-mask');
  await expect(modalMask).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(modalMask).toBeHidden();
});

test('generate request includes size and ratio dimensions', async ({ page }) => {
  let capturedBody: any = null;
  await page.route('**/api/generate', async (route) => {
    capturedBody = route.request().postDataJSON();
    await route.fulfill({
      status: 200,
      json: {
        node: createNode('node-payload', 'test'),
        images: [],
      },
    });
  });

  await page.goto('/txt2img');

  await setPromptViaSdkModal(page, 'a cute cat');
  await page.locator('#negative-prompt').fill('dog, bird');
  await page.locator('#cfg-scale').fill('8.5');
  await page.locator('#steps').fill('30');
  await page.locator('#image-size').selectOption('2K');
  await page.locator('#aspect-ratio').selectOption('16:9');
  await page.getByTestId('generate').click();

  await expect.poll(() => capturedBody, { timeout: 5000 }).not.toBeNull();

  expect(capturedBody).toMatchObject({
    // SDK editor serializes prompts with an optional group header (e.g. "##Default\n")
    // before the user text; match containment rather than exact equality.
    prompt: expect.stringContaining('a cute cat'),
    negative_prompt: 'dog, bird',
    cfg_scale: 8.5,
    steps: 30,
    aspect_ratio: '16:9',
    // Default model is gpt-image-2 (first enabled in config.yaml); its 2K 16:9 preset is 3584x2048.
    width: 3584,
    height: 2048,
    reference_image_ids: [],
    model: 'gpt-image-2',
  });
});

test('empty prompt and reroll disabled', async ({ page }) => {
  await page.goto('/txt2img');

  await setPromptViaSdkModal(page, '');
  
  const generateBtn = page.getByTestId('generate');
  await expect(generateBtn).toBeDisabled();

  const url = new URL(page.url());
  expect(url.searchParams.has('node_id')).toBeFalsy();
});

test('reroll-count input toggles off when prompt becomes dirty', async ({ page }) => {
  // Under the unified "Generate" button, the button itself is always named "generate"
  // and its action (fork vs reroll) depends on whether the prompt is dirty. The visible
  // toggle is the reroll-count input: shown when a node is loaded AND the prompt is
  // clean (matches the node), hidden once the user starts editing.
  const nodeId = 'node-toggle';
  await page.route('**/api/nodes/*', async (route) => {
    await route.fulfill({
      json: {
        node: createNode(nodeId, 'initial prompt'),
        images: [],
      },
    });
  });

  await page.goto(`/txt2img?node_id=${nodeId}`);

  // clean + node loaded → reroll-count visible, generate button always visible
  await expect(page.getByTestId('generate')).toBeVisible();
  await expect(page.getByTestId('reroll-count')).toBeVisible();

  await setPromptViaSdkModal(page, 'dirty');

  // dirty → reroll-count hidden, generate still visible
  await expect(page.getByTestId('generate')).toBeVisible();
  await expect(page.getByTestId('reroll-count')).toBeHidden();
});

test('historical reroll keeps node_id and remains consistent after refresh', async ({ page }) => {
  const oldNodeId = 'node-old';
  const oldPrompt = 'cat';

  let currentImages = [createImage('old-1', oldNodeId), createImage('old-2', oldNodeId)];
  const rerollImages = [createImage('reroll-1', oldNodeId)];

  await page.route('**/api/nodes/*', async (route) => {
    await route.fulfill({
      json: {
        node: createNode(oldNodeId, oldPrompt),
        images: currentImages,
      },
    });
  });

  await page.route('**/api/prompts/*/reroll', async (route) => {
    currentImages = rerollImages;
    await route.fulfill({
      status: 200,
      json: {
        node: createNode(oldNodeId, oldPrompt),
        images: rerollImages,
      },
    });
  });

  await page.goto(`/txt2img?node_id=${oldNodeId}`);

  await expect(page.getByTestId('result-grid').locator('img').first()).toBeVisible();
  await expect(page.getByTestId('result-grid').locator('img')).toHaveCount(2);
  await expect(page).toHaveURL(new RegExp(`node_id=${oldNodeId}`));

  await page.getByTestId('generate').click();

  await expect(page.getByTestId('result-grid').locator('img')).toHaveCount(1);
  await expect(page).toHaveURL(new RegExp(`node_id=${oldNodeId}`));

  await page.reload();

  await expect(page.getByTestId('result-grid').locator('img').first()).toBeVisible();
  await expect(page.getByTestId('result-grid').locator('img')).toHaveCount(1);
  await expect(page).toHaveURL(new RegExp(`node_id=${oldNodeId}`));
});

test('historical Generate & Fork updates node_id and remains on new node after refresh', async ({ page }) => {
  const oldNodeId = 'node-old';
  const newNodeId = 'node-new';
  const oldPrompt = 'cat';
  const newPrompt = 'cat with hat';

  const oldImages = [createImage('old-1', oldNodeId)];
  const newImages = [createImage('new-1', newNodeId)];
  let capturedGenerateBody: any = null;

  await page.route('**/api/nodes/*', async (route) => {
    const url = new URL(route.request().url());
    const nodeId = url.pathname.split('/').pop();
    if (nodeId === oldNodeId) {
      await route.fulfill({
        json: {
          node: createNode(oldNodeId, oldPrompt),
          images: oldImages,
        },
      });
    } else if (nodeId === newNodeId) {
      await route.fulfill({
        json: {
          node: createNode(newNodeId, newPrompt, { parent_id: oldNodeId }),
          images: newImages,
        },
      });
    } else {
      await route.fulfill({ status: 404, json: { error: 'not found' } });
    }
  });

  await page.route('**/api/generate', async (route) => {
    capturedGenerateBody = route.request().postDataJSON();
    await route.fulfill({
      status: 200,
      json: {
        node: createNode(newNodeId, newPrompt, { parent_id: oldNodeId }),
        images: newImages,
      },
    });
  });

  await page.goto(`/txt2img?node_id=${oldNodeId}`);

  await expect(page.getByTestId('result-grid').locator('img').first()).toBeVisible();
  await expect(page.getByTestId('result-grid').locator('img')).toHaveCount(1);
  await expect(page).toHaveURL(new RegExp(`node_id=${oldNodeId}`));

  await setPromptViaSdkModal(page, newPrompt);
  await page.getByTestId('generate').click();

  await expect.poll(() => capturedGenerateBody).not.toBeNull();
  expect(capturedGenerateBody).toMatchObject({
    parent_node_id: oldNodeId,
    // SDK editor serializes prompts with an optional group header (e.g. "##Default\n");
    // match containment rather than exact equality.
    prompt: expect.stringContaining(newPrompt),
  });

  await expect(page.getByTestId('result-grid').locator('img').first()).toBeVisible();
  await expect(page.getByTestId('result-grid').locator('img')).toHaveCount(1);
  await expect(page).toHaveURL(new RegExp(`node_id=${newNodeId}`));

  await page.reload();

  await expect(page.getByTestId('result-grid').locator('img').first()).toBeVisible();
  await expect(page.getByTestId('result-grid').locator('img')).toHaveCount(1);
  await expect(page).toHaveURL(new RegExp(`node_id=${newNodeId}`));
});
