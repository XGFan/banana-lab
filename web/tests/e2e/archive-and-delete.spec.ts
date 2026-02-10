import { test, expect } from '@playwright/test';
import type { Page, APIRequestContext } from '@playwright/test';
import path from 'node:path';
import { mkdir, writeFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import { setPromptViaSdkModal } from './helpers/promptEditor';

type NodePayload = {
  node: {
    id: string;
    parent_id?: string;
    reference_image_ids?: string[];
  };
  images: Array<{
    id: string;
    file_path: string;
    origin: 'generated' | 'uploaded';
    created_at: string;
  }>;
  results?: Array<{
    id: string;
    node_id: string;
    asset_id: string;
    source_asset_id?: string;
    source_domain?: string;
    status: 'active' | 'archived' | 'deleted';
  }>;
};

type PromptTreePayload = {
  root_id: string;
  nodes: Array<{
    id: string;
    parent_id?: string;
  }>;
};

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const evidenceDir = path.resolve(__dirname, '../../../.sisyphus/evidence');
const apiBase = 'http://127.0.0.1:18080';

test.describe.configure({ timeout: 120_000 });

const unique = (prefix: string) => `${prefix}-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`;

async function ensureEvidenceDir(): Promise<void> {
  await mkdir(evidenceDir, { recursive: true });
}

async function writeEvidenceJSON(name: string, data: unknown): Promise<void> {
  await ensureEvidenceDir();
  await writeFile(path.join(evidenceDir, name), `${JSON.stringify(data, null, 2)}\n`, 'utf8');
}

async function writeEvidenceScreenshot(page: Page, name: string): Promise<void> {
  await ensureEvidenceDir();
  await page.screenshot({ path: path.join(evidenceDir, name), fullPage: true });
}

function getQueryValue(urlString: string, key: string): string | null {
  return new URL(urlString).searchParams.get(key);
}

async function generateNode(
  request: APIRequestContext,
  input: {
    mode: 'txt2img' | 'img2img';
    prompt: string;
    parentNodeId?: string;
    referenceImageIds?: string[];
  }
) {
  const response = await request.post(`${apiBase}/api/generate`, {
    data: {
      mode: input.mode,
      parent_node_id: input.parentNodeId,
      prompt: input.prompt,
      width: 1024,
      height: 1024,
      reference_image_ids: input.referenceImageIds ?? [],
    },
  });
  if (!response.ok()) {
    throw new Error(`generateNode failed: ${response.status()} ${await response.text()}`);
  }
  return (await response.json()) as NodePayload;
}

async function deletePrompt(request: APIRequestContext, nodeId: string): Promise<void> {
  const response = await request.delete(`${apiBase}/api/prompts/${nodeId}`);
  if (!response.ok()) {
    throw new Error(`deletePrompt failed: ${response.status()} ${await response.text()}`);
  }
}

async function getNode(request: APIRequestContext, nodeId: string): Promise<NodePayload> {
  const response = await request.get(`${apiBase}/api/nodes/${nodeId}`);
  if (!response.ok()) {
    throw new Error(`getNode failed: ${response.status()} ${await response.text()}`);
  }
  return (await response.json()) as NodePayload;
}

async function getPromptTree(request: APIRequestContext, nodeId: string): Promise<PromptTreePayload> {
  const response = await request.get(`${apiBase}/api/prompts/${nodeId}/tree`);
  if (!response.ok()) {
    throw new Error(`getPromptTree failed: ${response.status()} ${await response.text()}`);
  }
  return (await response.json()) as PromptTreePayload;
}

test('task-9: 结果软删后常规视图消失', async ({ page }) => {
  await page.goto('/txt2img');

  await setPromptViaSdkModal(page, unique('task-9-soft-delete'));
  const generateRespPromise = page.waitForResponse('**/api/generate');
  await page.getByTestId('generate').click();
  await generateRespPromise;

  const images = page.getByTestId('result-grid').locator('img');
  await expect(images.first()).toBeVisible({ timeout: 30000 });

  const nodeId = getQueryValue(page.url(), 'node_id');
  expect(nodeId).toBeTruthy();
  const beforeDeleteButtonCount = await page.locator('[data-testid^="result-delete-button-"]').count();
  expect(beforeDeleteButtonCount).toBeGreaterThan(0);

  page.once('dialog', async (dialog) => {
    await dialog.accept();
  });

  const deleteButton = page.locator('[data-testid^="result-delete-button-"]').first();
  await expect(deleteButton).toBeVisible();
  const deleteButtonTestId = await deleteButton.getAttribute('data-testid');
  const resultId = deleteButtonTestId?.replace('result-delete-button-', '');
  await deleteButton.click({ force: true });
  await expect(deleteButton).toHaveCount(0);
  await expect(page.locator('[data-testid^="result-delete-button-"]')).toHaveCount(beforeDeleteButtonCount - 1);

  await writeEvidenceScreenshot(page, 'task-9-soft-delete.png');
  await writeEvidenceJSON('task-9-soft-delete.json', {
    nodeId,
    deletedResultId: resultId,
    beforeDeleteButtonCount,
    afterDeleteButtonCount: beforeDeleteButtonCount - 1,
  });
});

test('task-9: prompt 删除 Mode A 后重挂接且页面可继续访问', async ({ page, request }) => {
  const root = await generateNode(request, { mode: 'txt2img', prompt: unique('task-9-root') });
  const target = await generateNode(request, {
    mode: 'txt2img',
    parentNodeId: root.node.id,
    prompt: unique('task-9-target-a'),
  });
  const child = await generateNode(request, {
    mode: 'txt2img',
    parentNodeId: target.node.id,
    prompt: unique('task-9-child-a'),
  });

  await page.goto(`/txt2img?node_id=${target.node.id}`);
  await expect(page.getByTestId('prompt')).toBeVisible();
  await expect(page).toHaveURL(new RegExp(`node_id=${target.node.id}`));

  await deletePrompt(request, target.node.id);

  const deletedNodeResp = await request.get(`${apiBase}/api/nodes/${target.node.id}`);
  expect(deletedNodeResp.status()).toBe(404);
  await expect(deletedNodeResp.json()).resolves.toMatchObject({ error: 'not_found' });

  const tree = await getPromptTree(request, child.node.id);
  expect(tree.nodes.some(n => n.id === target.node.id)).toBeFalsy();
  const childInTree = tree.nodes.find(n => n.id === child.node.id);
  expect(childInTree).toBeTruthy();
  expect(childInTree?.parent_id).toBe(root.node.id);

  const tree404Promise = page.waitForResponse(
    (resp) => resp.url().includes(`/api/prompts/${target.node.id}/tree`) && resp.status() === 404,
  );
  await page.goto(`/txt2img?node_id=${target.node.id}`);
  await tree404Promise;
  await expect(page).not.toHaveURL(/node_id=/);
  await expect(page.getByTestId('prompt')).toHaveValue('');

  await writeEvidenceScreenshot(page, 'task-9-mode-a-reparent.png');
  await writeEvidenceJSON('task-9-mode-a-reparent.json', {
    deletedNodeId: target.node.id,
    rootNodeId: root.node.id,
    childNodeId: child.node.id,
    tree,
    deletedNodeStatus: deletedNodeResp.status(),
  });
});

test('task-9: prompt 删除 Mode B 后常规视图消失并进入归档分组', async ({ page, request }) => {
  const root = await generateNode(request, { mode: 'txt2img', prompt: unique('task-9-root-b') });
  const target = await generateNode(request, {
    mode: 'txt2img',
    parentNodeId: root.node.id,
    prompt: unique('task-9-target-b'),
  });
  const child = await generateNode(request, {
    mode: 'txt2img',
    parentNodeId: target.node.id,
    prompt: unique('task-9-child-b'),
  });

  await deletePrompt(request, target.node.id);

  const deletedNodeResp = await request.get(`${apiBase}/api/nodes/${target.node.id}`);
  expect(deletedNodeResp.status()).toBe(404);
  await expect(deletedNodeResp.json()).resolves.toMatchObject({ error: 'not_found' });

  const tree = await getPromptTree(request, child.node.id);
  const childInTree = tree.nodes.find(n => n.id === child.node.id);
  expect(childInTree?.parent_id).toBe(root.node.id);

  const tree404Promise = page.waitForResponse(
    (resp) => resp.url().includes(`/api/prompts/${target.node.id}/tree`) && resp.status() === 404,
  );
  await page.goto(`/txt2img?node_id=${target.node.id}`);
  await tree404Promise;
  await expect(page).not.toHaveURL(/node_id=/);
  await expect(page.getByTestId('prompt')).toHaveValue('');

  await writeEvidenceScreenshot(page, 'task-9-mode-b-archive.png');
  await writeEvidenceJSON('task-9-mode-b-archive.json', {
    deletedNodeId: target.node.id,
    rootNodeId: root.node.id,
    childNodeId: child.node.id,
    tree,
    deletedNodeStatus: deletedNodeResp.status(),
  });
});

test('task-9: txt2img 结果作为 img2img 输入时 asset 解耦', async ({ page, request }) => {
  await page.goto('/txt2img');

  const sourcePrompt = unique('task-9-source-txt2img');
  await setPromptViaSdkModal(page, sourcePrompt);
  const generateSourcePromise = page.waitForResponse('**/api/generate');
  await page.getByTestId('generate').click();
  await generateSourcePromise;
  await expect(page.getByTestId('result-grid').locator('img').first()).toBeVisible({ timeout: 30000 });

  const sourceNodeId = getQueryValue(page.url(), 'node_id');
  expect(sourceNodeId).toBeTruthy();
  const sourceNode = await getNode(request, sourceNodeId!);
  const sourceAssetId = sourceNode.results?.[0]?.asset_id ?? sourceNode.images[0]?.id;
  expect(sourceAssetId).toBeTruthy();

  await page.goto('/img2img');
  await page.getByTestId('upload-card').getByRole('button', { name: 'Library' }).click();
  await expect(page.getByRole('dialog')).toBeVisible();

  await page.getByTestId('library-type').selectOption('txt2img');
  await page.getByTestId('library-search').fill(sourceAssetId);

  const libraryGrid = page.getByTestId('library-grid');
  const firstLibraryImage = libraryGrid.locator('button').first();
  await expect(firstLibraryImage).toBeVisible({ timeout: 15000 });
  await firstLibraryImage.click();
  await page.getByTestId('library-confirm').click();

  await expect(page.getByRole('dialog')).toBeHidden();
  await expect(page.getByTestId('reference-image')).toHaveCount(1);

  await setPromptViaSdkModal(page, unique('task-9-img2img-generate'));
  const generateImg2ImgPromise = page.waitForResponse('**/api/generate');
  await page.getByTestId('generate').click();
  await generateImg2ImgPromise;
  await expect(page.getByTestId('result-grid').locator('img').first()).toBeVisible({ timeout: 30000 });

  const img2imgNodeId = getQueryValue(page.url(), 'node_id');
  expect(img2imgNodeId).toBeTruthy();

  const img2imgNode = await getNode(request, img2imgNodeId!);
  const refId = img2imgNode.node.reference_image_ids?.[0];
  expect(refId).toBeTruthy();
  expect(refId).not.toBe(sourceAssetId);

  const img2imgResult = img2imgNode.results?.[0];
  if (img2imgResult) {
    expect(img2imgResult.source_asset_id).toBe(sourceAssetId);
    expect(img2imgResult.asset_id).not.toBe(sourceAssetId);
  }

  await writeEvidenceScreenshot(page, 'task-9-img2img-decouple.png');
  await writeEvidenceJSON('task-9-img2img-decouple.json', {
    sourceNodeId,
    sourceAssetId,
    img2imgNodeId,
    img2imgReferenceImageIds: img2imgNode.node.reference_image_ids,
    img2imgResult,
  });
});
