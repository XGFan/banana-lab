import { expect, test } from "@playwright/test";

test("smoke: static content renders", async ({ page }) => {
  await page.setContent(`
    <!doctype html>
    <html lang="zh-CN">
      <head>
        <meta charset="utf-8" />
        <title>Banana Lab</title>
      </head>
      <body>
        <main>
          <h1>Banana Lab</h1>
          <button type="button" aria-label="开始">开始</button>
        </main>
      </body>
    </html>
  `);

  await expect(
    page.getByRole("heading", {
      name: "Banana Lab",
    })
  ).toBeVisible();

  await expect(
    page.getByRole("button", {
      name: "开始",
    })
  ).toBeEnabled();
});
