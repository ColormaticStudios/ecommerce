import { expect, test } from "@playwright/test";
import { apiBaseURL, seedAndLoginUser } from "./admin-helpers";

test("admin can save private variant costs and enable business ranking signals", async ({
	page,
	request,
}) => {
	const stamp = Date.now();
	await seedAndLoginUser(page, request, {
		email: `search-admin-${stamp}@example.com`,
		username: `search-admin-${stamp}`,
		role: "admin",
	});
	await Promise.all([
		page.waitForResponse((response) => response.url().endsWith("/api/v1/me/")),
		page.goto("/admin/product/1"),
	]);
	const cost = page.getByRole("spinbutton", { name: "Variant 1 unit cost" });
	await cost.fill("12.34");
	await page.getByRole("button", { name: "Save draft" }).click();
	await expect(page.getByRole("button", { name: /\bPublish$/ })).toBeEnabled();
	await Promise.all([
		page.waitForResponse((response) => response.url().endsWith("/api/v1/me/")),
		page.reload(),
	]);
	await expect(cost).toHaveValue("12.34");
	await cost.fill("0");
	await page.getByRole("button", { name: "Save draft" }).click();
	await expect(page.getByRole("button", { name: /\bPublish$/ })).toBeEnabled();
	await Promise.all([
		page.waitForResponse((response) => response.url().endsWith("/api/v1/me/")),
		page.reload(),
	]);
	await expect(cost).toHaveValue("0");
	const publicProduct = await page.request.get(`${apiBaseURL}/api/v1/products/1`);
	expect(publicProduct.ok()).toBeTruthy();
	expect(await publicProduct.text()).not.toContain('"unit_cost"');
	page.once("dialog", (dialog) => dialog.accept());
	await page.getByRole("button", { name: "Discard draft" }).click();
	await expect(cost).toHaveValue("");
	await Promise.all([
		page.waitForResponse((response) => response.url().endsWith("/api/v1/me/")),
		page.goto("/admin/search/ranking-profiles"),
	]);
	const name = `Business signals ${stamp}`;
	await page.getByRole("textbox", { name: "Name", exact: true }).fill(name);
	await page.getByRole("spinbutton", { name: "Gross margin", exact: true }).fill("5");
	await page.getByRole("spinbutton", { name: "Purchase conversion", exact: true }).fill("10");
	await page.getByRole("button", { name: "Create", exact: true }).click();
	await expect(page.getByText("Search settings saved.", { exact: true })).toBeVisible();
	await Promise.all([
		page.waitForResponse((response) => response.url().endsWith("/api/v1/me/")),
		page.reload(),
	]);
	await page.getByRole("button", { name: new RegExp(name) }).click();
	await expect(page.getByRole("spinbutton", { name: "Gross margin", exact: true })).toHaveValue(
		"5"
	);
	await expect(
		page.getByRole("spinbutton", { name: "Purchase conversion", exact: true })
	).toHaveValue("10");
	page.once("dialog", (dialog) => dialog.accept());
	await page.getByRole("button", { name: "Delete", exact: true }).click();
	await expect(page.getByText("Search configuration deleted.", { exact: true })).toBeVisible();
	await page.goto("/admin/search/operations");
	await expect(
		page.getByRole("heading", { name: "stale index history", exact: true })
	).toBeVisible();
	await page.getByRole("combobox", { name: "Incident status" }).selectOption("resolved");
	await expect(
		page.getByText("No recorded episodes match this filter.", { exact: true })
	).toBeVisible();
});
