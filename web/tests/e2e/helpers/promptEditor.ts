import { Page, expect } from '@playwright/test';

/**
 * Sets the prompt via the SDK modal.
 * Handles opening the modal, interacting with the prompt editor SDK, and saving.
 * This helper is robust against SDK localization by finding generic elements.
 */
export async function setPromptViaSdkModal(page: Page, text: string) {
  const modal = page.getByTestId('sdk-modal-overlay');

  // If the modal is already open (e.g. opened automatically by /txt2img?action=new
  // or by a prior step), skip the trigger-click entirely and operate on the existing
  // modal — otherwise the overlay intercepts clicks and the helper times out.
  // A brief settle window lets the SPA finish rendering after navigation before we
  // probe the modal's presence.
  await page.waitForTimeout(150);
  const alreadyOpen = (await modal.count()) > 0;
  if (!alreadyOpen) {
    // 1. Open prompt editor
    const promptArea = page.getByTestId('prompt');
    // Use trial click to check visibility without failing immediately if covered
    try {
      await promptArea.hover({ timeout: 2000 });
    } catch (_e) {
      // Ignore hover errors, proceed to try clicking edit button
    }

    // After Group 6 unified-Generate-button refactor: `edit-prompt-btn` only renders
    // when no images exist on the current node. Once a historical node has been loaded
    // and its images rendered, forking into a new node goes through `fork-prompt-btn`
    // instead. Both open the same editor modal, so detect whichever is visible.
    const editBtn = page.getByTestId('edit-prompt-btn');
    const forkBtn = page.getByTestId('fork-prompt-btn');
    if (await editBtn.count() > 0 && await editBtn.first().isVisible()) {
      await editBtn.click();
    } else {
      await forkBtn.click();
    }
  }
  
  // 2. Wait for modal to appear using the robust testid we added
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
      // The textarea may include SDK group headers (e.g. "##Default\n") before
      // the prompt text — check containment rather than exact equality.
      await expect(promptAreaAfter).toHaveValue(new RegExp(text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')));
  }
}
