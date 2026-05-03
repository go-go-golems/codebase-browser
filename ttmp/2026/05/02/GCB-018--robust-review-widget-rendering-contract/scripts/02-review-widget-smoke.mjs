#!/usr/bin/env node
import { chromium } from 'playwright';

const baseURL = process.argv[2] ?? 'http://127.0.0.1:4179';
const forbidden = /doc error|Failed|Unknown|not found|outside this export|Loading…|Loading\.\.\.|Loading annotation|Loading snippet|&#34;/i;

const browser = await chromium.launch({ headless: true });
try {
  const page = await browser.newPage();
  await page.goto(`${baseURL}/?v=widget-smoke#/review/all-widgets-smoke`, { waitUntil: 'networkidle' });
  await page.waitForTimeout(1000);

  const mainText = await page.locator('main').innerText();
  if (forbidden.test(mainText)) {
    throw new Error(`visible widget failure on all-widgets-smoke:\n${mainText}`);
  }

  const fileWidgetParagraphs = await page.locator('[data-directive="codebase-file"] p').count();
  if (fileWidgetParagraphs !== 0) {
    throw new Error(`codebase-file widget contains ${fileWidgetParagraphs} paragraph tag(s), expected 0`);
  }

  const escapedQuote = mainText.includes('&#34;');
  if (escapedQuote) {
    throw new Error('page contains escaped quote artifact &#34;');
  }

  const legacyStubs = await page.locator('[data-codebase-snippet]').count();
  if (legacyStubs !== 0) {
    throw new Error(`page still contains ${legacyStubs} legacy data-codebase-snippet stub(s)`);
  }

  const stepButtons = page.locator('[data-role="commit-walk"] ol button');
  const stepButtonCount = await stepButtons.count();
  for (let i = 0; i < stepButtonCount; i++) {
    await stepButtons.nth(i).click();
    await page.waitForTimeout(300);
    const text = await page.locator('main').innerText();
    if (forbidden.test(text)) {
      throw new Error(`visible widget failure on commit-walk step ${i + 1}:\n${text}`);
    }
  }

  const resolved = mainText.match(/Resolved\s+(\d+)\s+snippet\(s\)/i);
  if (!resolved || Number.parseInt(resolved[1], 10) < 11) {
    throw new Error(`expected at least 11 resolved snippets, got: ${resolved?.[0] ?? 'none'}`);
  }

  console.log('Playwright review-widget smoke PASS');
} finally {
  await browser.close();
}
