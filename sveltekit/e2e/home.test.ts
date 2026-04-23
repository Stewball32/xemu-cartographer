import { test, expect } from '@playwright/test';

test('home page loads successfully', async ({ page }) => {
	await page.goto('/');
	await expect(page).toHaveTitle(/.+/);
});

test('navigation bar is visible', async ({ page }) => {
	await page.goto('/');
	const nav = page.locator('nav');
	await expect(nav.first()).toBeVisible();
});

test('unauthenticated user can reach login page', async ({ page }) => {
	await page.goto('/login/');
	await expect(page.locator('h1')).toContainText('Sign In');
});
