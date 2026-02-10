import { test, expect } from '@playwright/test';
import { setPromptViaSdkModal } from './helpers/promptEditor';

test('user journey: txt2img to img2img', async ({ page }) => {
  test.setTimeout(60000);

  let lastNode: any = null;
  let generateCount = 0;

  await page.route('**/api/prompts*', async (route) => {
    const url = route.request().url();
    if (route.request().method() === 'GET' && !url.includes('/tree') && !url.includes('/reroll')) {
      await route.fulfill({ json: lastNode ? [lastNode] : [] });
    } else {
      await route.continue();
    }
  });

  await page.route('**/api/nodes/*', async (route) => {
    if (route.request().method() === 'GET') {
      if (lastNode) {
        await route.fulfill({
          json: {
            node: lastNode,
            images: [
              {
                id: `img-gen-${generateCount}`,
                origin: 'generated',
                file_path: `images/gen-${generateCount}.png`,
                created_at: new Date().toISOString(),
                width: 1024,
                height: 1024,
              },
            ],
            results: [],
          },
        });
      } else {
        await route.fulfill({ status: 404, json: { error: 'not_found', message: 'Node not found' } });
      }
    } else {
      await route.continue();
    }
  });

  await page.route('**/api/prompts/*/tree', async (route) => {
    await route.fulfill({
      json: {
        root_id: lastNode?.id || '',
        nodes: lastNode ? [lastNode] : [],
      }
    });
  });

  await page.route('**/api/library*', async (route) => {
    await route.fulfill({
      json: [
        {
          id: 'img-lib-1',
          origin: 'generated',
          file_path: 'images/lib-1.png',
          created_at: new Date().toISOString(),
          width: 512,
          height: 512,
        },
      ],
    });
  });

  await page.route('**/api/generate', async (route) => {
    generateCount++;
    const body = route.request().postDataJSON();
    lastNode = {
      id: `node-${generateCount}`,
      parent_id: body.parent_id || null,
      mode: body.mode || 'txt2img',
      prompt: body.prompt,
      title: body.prompt.slice(0, 20),
      pinned: false,
      last_used_at: new Date().toISOString(),
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
      archived_at: null,
      deleted_at: null,
    };
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        node: lastNode,
        images: [
          {
            id: `img-gen-${generateCount}`,
            origin: 'generated',
            file_path: `images/gen-${generateCount}.png`,
            created_at: new Date().toISOString(),
            width: 1024,
            height: 1024,
          },
        ],
      }),
    });
  });

  await page.goto('/txt2img');

  const uniquePrompt = `Unique banana ${Date.now()}`;
  await setPromptViaSdkModal(page, uniquePrompt);

  await page.getByTestId('generate').click();


  const txtResultGrid = page.getByTestId('result-grid');
  await expect(txtResultGrid.locator('img').first()).toBeVisible({ timeout: 20000 });

  await page.goto('/img2img');

  await page.getByTestId('upload-card').getByRole('button', { name: 'Library' }).click();
  await expect(page.getByRole('dialog')).toBeVisible();

  const libraryGrid = page.getByTestId('library-grid');
  await expect(libraryGrid.locator('img').first()).toBeVisible({ timeout: 10000 });
  await libraryGrid.locator('img').first().click();
  await page.getByTestId('library-confirm').click();
  await expect(page.getByRole('dialog')).toBeHidden();

  await expect(page.getByTestId('reference-image')).toHaveCount(1);

  await setPromptViaSdkModal(page, 'transform it');
  await page.getByTestId('generate').click();


  const imgResultGrid = page.getByTestId('result-grid');
  await expect(imgResultGrid.locator('img').first()).toBeVisible({ timeout: 20000 });
});
