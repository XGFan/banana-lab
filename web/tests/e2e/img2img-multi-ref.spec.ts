import { test, expect } from '@playwright/test';
import { setPromptViaSdkModal } from './helpers/promptEditor';

const pngBase64 =
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAOa1s8kAAAAASUVORK5CYII=';
const pngBuffer = Buffer.from(pngBase64, 'base64');

const createPromptNode = (id: string, overrides = {}) => {
  const now = new Date().toISOString();
  return {
    id,
    parent_id: null,
    mode: 'img2img',
    prompt: '',
    title: id,
    pinned: false,
    last_used_at: now,
    created_at: now,
    updated_at: now,
    archived_at: null,
    deleted_at: null,
    ...overrides,
  };
};

const createUploadImage = (id: string) => ({
  id,
  origin: 'uploaded' as const,
  file_path: `uploads/${id}.png`,
  created_at: new Date().toISOString(),
});

const createGeneratedImage = (id: string) => ({
  id,
  origin: 'generated' as const,
  file_path: `images/${id}.png`,
  created_at: new Date().toISOString(),
});

const uploadFile = async (page: any, name: string) => {
  await page.getByTestId('file-input').setInputFiles({
    name,
    mimeType: 'image/png',
    buffer: pngBuffer,
  });
};

test.beforeEach(async ({ page }) => {
  await page.route('**/api/prompts?mode=img2img*', async (route) => {
    await route.fulfill({ json: [] });
  });
});

test('img2img multi ref upload, delete, limit, payload, and preview', async ({ page }) => {
  let uploadIndex = 0;
  const uploadedIds: string[] = [];

  await page.route('**/api/upload', async (route) => {
    uploadIndex += 1;
    const id = `ref-${uploadIndex}`;
    uploadedIds.push(id);
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ image: createUploadImage(id) }),
    });
  });

  let capturedBody: any = null;
  const newNodeId = 'node-img2img-result';

  await page.route('**/api/generate', async (route) => {
    capturedBody = route.request().postDataJSON();
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        node: createPromptNode(newNodeId, { prompt: capturedBody?.prompt || 'transform' }),
        images: [createGeneratedImage('gen-1')],
      }),
    });
  });

  await page.route(`**/api/nodes/${newNodeId}`, async (route) => {
    await route.fulfill({
      json: {
        node: createPromptNode(newNodeId, { prompt: 'transform' }),
        images: [createGeneratedImage('gen-1')],
      },
    });
  });

  await page.route(`**/api/prompts/${newNodeId}/tree`, async (route) => {
    await route.fulfill({
      json: { root_id: newNodeId, nodes: [createPromptNode(newNodeId, { prompt: 'transform' })] },
    });
  });

  await page.goto('/img2img');

  await uploadFile(page, 'ref-1.png');
  await uploadFile(page, 'ref-2.png');
  await uploadFile(page, 'ref-3.png');
  await expect(page.getByTestId('reference-image')).toHaveCount(3);

  const secondRef = page.getByTestId('reference-image').nth(1);
  await secondRef.hover();
  await page.getByTestId('delete-ref-image').nth(1).click();
  await expect(page.getByTestId('reference-image')).toHaveCount(2);

  await uploadFile(page, 'ref-4.png');
  await uploadFile(page, 'ref-5.png');
  await uploadFile(page, 'ref-6.png');
  await expect(page.getByTestId('reference-image')).toHaveCount(5);

  await uploadFile(page, 'ref-7.png');
  await expect(page.getByTestId('reference-image')).toHaveCount(5);
  
  await setPromptViaSdkModal(page, 'transform');
  await page.locator('#image-size').selectOption('2K');
  await page.locator('#aspect-ratio').selectOption('16:9');

  
  const generatePromise = page.waitForResponse('**/api/generate');
  await page.getByTestId('generate').click();
  await generatePromise;

  expect(capturedBody).not.toBeNull();
  
  const expectedReferenceIds = [
    uploadedIds[0],
    uploadedIds[2],
    uploadedIds[3],
    uploadedIds[4],
    uploadedIds[5],
  ];

  expect(capturedBody).toMatchObject({
    // SDK editor may prepend a group header (e.g. "##Default\n"); match containment.
    prompt: expect.stringContaining('transform'),
    aspect_ratio: '16:9',
    // Default model is gpt-image-2; its 2K 16:9 preset is 3584x2048.
    width: 3584,
    height: 2048,
    reference_image_ids: expectedReferenceIds,
  });

  const imageCard = page.getByTestId('result-grid').locator('.cursor-zoom-in').first();
  await expect(imageCard).toBeVisible({ timeout: 10000 });
  
  await imageCard.click({ trial: true });
  await imageCard.click();

  const modalMask = page.getByTestId('preview-modal-mask');
  await expect(modalMask).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(modalMask).toBeHidden();
});

