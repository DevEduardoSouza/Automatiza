import { expect, test } from '@playwright/test';
import { setupAutomationLogsMocks } from './fixtures/automation-logs.fixture';

test.setTimeout(60_000);

test('debug virtual table', async ({ page }) => {
  page.on('pageerror', (err) => {
    console.log(`[PAGE ERROR] ${err.message}`);
  });

  await setupAutomationLogsMocks(page);
  await page.goto('/automation-logs');
  await page.waitForSelector('h1', { state: 'visible' });
  await page.waitForTimeout(5000);

  // Dump the desktop block HTML (first 1000 chars)
  const desktopHTML = await page.evaluate(() => {
    // Look for elements with both 'hidden' and 'md:block' classes
    const els = document.querySelectorAll('[class]');
    const matching: string[] = [];
    els.forEach((el) => {
      const cls = el.className;
      if (typeof cls === 'string' && cls.includes('hidden') && cls.includes('md:block')) {
        matching.push(
          `<${el.tagName} class="${cls.substring(0, 60)}">\nINNER: ${el.innerHTML.substring(0, 200)}\n---`
        );
      }
    });
    return matching.join('\n\n');
  });
  console.log(`Desktop blocks:\n${desktopHTML}`);

  // Check all tables on the page
  const tableInfo = await page.evaluate(() => {
    const tables = document.querySelectorAll('table');
    return Array.from(tables).map((t, i) => {
      const rows = t.querySelectorAll('tbody tr').length;
      const headerRows = t.querySelectorAll('thead tr').length;
      const parent = t.parentElement?.className?.substring(0, 50) || 'no parent';
      return `table[${i}]: header=${headerRows}, body=${rows}, parent="${parent}"`;
    });
  });
  console.log(`Tables: ${JSON.stringify(tableInfo, null, 2)}`);

  // Find all elements with "Enviado" text and check their visibility
  const enviadoInfo = await page.evaluate(() => {
    const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT, {
      acceptNode: (node) =>
        node.textContent?.includes('Enviado') ? NodeFilter.FILTER_ACCEPT : NodeFilter.FILTER_REJECT,
    });
    const results: string[] = [];
    let node: Node | null = walker.nextNode();
    while (node) {
      const el = node.parentElement;
      if (el) {
        const rect = el.getBoundingClientRect();
        results.push(
          `"${node.textContent?.trim()}" in <${el.tagName} class="${el.className?.substring(0, 40)}"> visible=${rect.width > 0 && rect.height > 0} rect=${JSON.stringify({ x: rect.x, y: rect.y, w: rect.width, h: rect.height })}`
        );
      }
      node = walker.nextNode();
    }
    return results;
  });
  console.log(`Enviado text nodes:\n${enviadoInfo.join('\n')}`);

  expect(true).toBe(true);
});
