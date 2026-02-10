import { Page, expect } from '@playwright/test';

/**
 * Sets the prompt via the SDK modal.
 * Handles opening the modal, interacting with the prompt editor SDK, and saving.
 * This helper is robust against SDK localization by finding generic elements.
 */
export async function setPromptViaSdkModal(page: Page, text: string) {
  // 1. Open prompt editor
  const promptArea = page.getByTestId('prompt');
  // Use trial click to check visibility without failing immediately if covered
  try {
    await promptArea.hover({ timeout: 2000 });
  } catch (e) {
    // Ignore hover errors, proceed to try clicking edit button
  }
  
  await page.getByTestId('edit-prompt-btn').click();
  
  // 2. Wait for modal to appear using the robust testid we added
  const modal = page.getByTestId('sdk-modal-overlay');
  await expect(modal).toBeVisible();

  // 3. Use E2E backdoor to set prompt
  // This bypasses the complex SDK UI interaction which is fragile in test envs
  const e2eInput = page.getByTestId('e2e-sdk-prompt');
  await expect(e2eInput).toBeVisible({ timeout: 5000 });
  await e2eInput.fill(text);
  
  // Apply changes (triggering state update in page)
  await page.getByTestId('e2e-sdk-apply').click();
  // Allow React to process state update and SDK to sync
  await page.waitForTimeout(500);

  // 4. Save the modal (Save Changes)
  await page.getByTestId('sdk-save-btn').click();

  // 5. Wait for modal to close
  await expect(modal).not.toBeVisible();
  
  const promptAreaAfter = page.getByTestId('prompt');
  if (text.trim()) {
      await expect(promptAreaAfter).toHaveValue(text);
  }
}
