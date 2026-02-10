import { test, expect } from '@playwright/test';

test.describe('Global Forced Light Theme', () => {
  test('should enforce light theme attributes and colors', async ({ page }) => {
    await page.goto('/txt2img');
    
    const isDarkClassPresent = await page.evaluate(() => {
      return document.documentElement.classList.contains('dark');
    });
    expect(isDarkClassPresent, '<html> should NOT have "dark" class').toBe(false);

    const colorScheme = await page.evaluate(() => {
      return getComputedStyle(document.documentElement).colorScheme;
    });
    expect(colorScheme, 'color-scheme should be "light"').toBe('light');

    const appShell = page.getByTestId('app-shell');
    await expect(appShell).toBeVisible();

    const backgroundColor = await appShell.evaluate((el) => {
      return getComputedStyle(el).backgroundColor;
    });
    
    const parseColor = (colorStr: string) => {
      const rgbMatch = colorStr.match(/rgb\((\d+),\s*(\d+),\s*(\d+)\)/);
      if (rgbMatch) {
        return { 
            type: 'rgb', 
            r: parseInt(rgbMatch[1]), 
            g: parseInt(rgbMatch[2]), 
            b: parseInt(rgbMatch[3]),
            isLight: () => (parseInt(rgbMatch[1]) > 200 && parseInt(rgbMatch[2]) > 200 && parseInt(rgbMatch[3]) > 200),
            isDark: () => (parseInt(rgbMatch[1]) < 100 && parseInt(rgbMatch[2]) < 100 && parseInt(rgbMatch[3]) < 100)
        };
      }
      
      const oklchMatch = colorStr.match(/oklch\(([\d.]+)/);
      if (oklchMatch) {
        const l = parseFloat(oklchMatch[1]);
        return {
            type: 'oklch',
            l,
            isLight: () => l > 0.8,
            isDark: () => l < 0.4
        };
      }

      return null;
    };

    const bgParsed = parseColor(backgroundColor);
    
    if (bgParsed) {
      expect(bgParsed.isLight(), `Background color ${backgroundColor} should be light`).toBe(true);
    } else {
        console.warn('Could not parse background color:', backgroundColor);
    }

    const color = await appShell.evaluate((el) => {
      return getComputedStyle(el).color;
    });
    
    const textParsed = parseColor(color);
    if (textParsed) {
        expect(textParsed.isDark(), `Text color ${color} should be dark`).toBe(true);
    } else {
        console.warn('Could not parse text color:', color);
    }
  });
});
